param([Parameter(Mandatory=$true)][string]$ReleaseTag)
$ErrorActionPreference = 'Stop'
$smokeRoot = Join-Path $env:RUNNER_TEMP ('v1-import-' + $env:GITHUB_RUN_ID)
if (-not $smokeRoot.StartsWith($env:RUNNER_TEMP, [StringComparison]::OrdinalIgnoreCase) -or (Test-Path -LiteralPath $smokeRoot)) { throw 'Invalid isolated smoke directory.' }
New-Item -ItemType Directory -Path $smokeRoot | Out-Null
$installRoot = Join-Path $smokeRoot 'install'
$headers = @{ 'Accept'='application/vnd.github+json'; 'User-Agent'='WonderX-import-smoke' }
if ($env:GH_TOKEN) { $headers.Authorization = 'Bearer ' + $env:GH_TOKEN }
$release = Invoke-RestMethod -Uri ('https://api.github.com/repos/worldneedme/xxd-build/releases/tags/' + [Uri]::EscapeDataString($ReleaseTag)) -Headers $headers
$asset = @($release.assets | Where-Object { $_.name -like '*-windows-amd64-setup.exe' })
if ($asset.Count -ne 1 -or $asset[0].digest -notmatch '^sha256:[0-9a-f]{64}$') { throw 'Expected exactly one checksummed Windows installer.' }
$setup = Join-Path $smokeRoot $asset[0].name
$downloadHeaders = @{ 'Accept'='application/octet-stream'; 'User-Agent'='WonderX-import-smoke' }
if ($env:GH_TOKEN) { $downloadHeaders.Authorization = 'Bearer ' + $env:GH_TOKEN }
Invoke-WebRequest -Uri $asset[0].url -Headers $downloadHeaders -OutFile $setup
if ((Get-FileHash -LiteralPath $setup -Algorithm SHA256).Hash.ToLowerInvariant() -ne $asset[0].digest.Substring(7)) { throw 'Installer checksum failed.' }
if (Get-CimInstance Win32_Process | Where-Object { $_.Name -in @('FlClash.exe','FlClashCore.exe','FlClashHelperService.exe') }) { throw 'Refusing to run installer with preexisting app processes.' }
$appSupport = Join-Path ([Environment]::GetFolderPath('ApplicationData')) 'com.follow\WONDERX V1'
if (Test-Path -LiteralPath $appSupport) { throw 'Refusing to overwrite preexisting client data.' }
New-Item -ItemType Directory -Path $appSupport | Out-Null
@{ 'flutter.config'='{"appSettingProps":{"disclaimerAccepted":true},"themeProps":{}}' } | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $appSupport 'shared_preferences.json') -NoNewline -Encoding utf8
Set-Content -LiteralPath (Join-Path $appSupport '.ci-import-owner') -Value $env:GITHUB_RUN_ID -NoNewline -Encoding ascii
$report = @{ tag=$ReleaseTag; asset=$asset[0].name; bytes=$asset[0].size; sha256=$asset[0].digest.Substring(7); install=$false; cold=$false; warm=$false; confirmation=$false; synthetic_tokens=$true }
$stdout = Join-Path $smokeRoot 'app.stdout.log'
$stderr = Join-Path $smokeRoot 'app.stderr.log'
function Owned-Processes {
    @(Get-CimInstance Win32_Process | Where-Object { $_.ExecutablePath -and $_.ExecutablePath.StartsWith($installRoot + '\', [StringComparison]::OrdinalIgnoreCase) })
}
function Read-Logs {
    $logText = ''
    foreach ($file in @($stdout,$stderr)) { if (Test-Path -LiteralPath $file) { $logText += [IO.File]::ReadAllText($file) } }
    $logText
}
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
function Dialog-Elements {
    $process = Get-Process -Name FlClash -ErrorAction SilentlyContinue | Where-Object { $_.Path -and $_.Path.StartsWith($installRoot + '\', [StringComparison]::OrdinalIgnoreCase) } | Select-Object -First 1
    if (-not $process -or -not $process.MainWindowHandle) { return @() }
    try {
        $root = [Windows.Automation.AutomationElement]::FromHandle($process.MainWindowHandle)
        @($root.FindAll([Windows.Automation.TreeScope]::Descendants,[Windows.Automation.Condition]::TrueCondition))
    } catch { @() }
}
function Wait-Link([string]$marker) {
    $deadline = (Get-Date).AddSeconds(45)
    while ((Get-Date) -lt $deadline) {
        $logs = Read-Logs
        if ($logs -match ('onAppLink:.*' + [regex]::Escape($marker))) { return 'app-link-event' }
        $elements = Dialog-Elements
        if ($elements | Where-Object { $_.Current.Name.Contains($marker) }) { return 'import-dialog' }
        if (-not (Owned-Processes | Where-Object { $_.Name -eq 'FlClash.exe' })) { throw 'Client exited before receiving import link.' }
        Start-Sleep -Milliseconds 500
    }
    throw ('No import event or dialog observed for ' + $marker + '. Check isolated smoke logs.')
}
function Dismiss-Confirmation {
    $elements = Dialog-Elements
    $cancel = $elements | Where-Object { $_.Current.Name -match '^(Cancel|取消)$' } | Select-Object -First 1
    if ($cancel) {
        $cancel.GetCurrentPattern([Windows.Automation.InvokePattern]::Pattern).Invoke()
        return $true
    }
    return $false
}
try {
    $installer = Start-Process -FilePath $setup -Wait -PassThru -WindowStyle Hidden -ArgumentList @('/VERYSILENT','/SUPPRESSMSGBOXES','/NORESTART','/SP-',('/DIR=' + $installRoot))
    if ($installer.ExitCode -ne 0) { throw ('Installer failed: ' + $installer.ExitCode) }
    $exe = Join-Path $installRoot 'FlClash.exe'
    foreach ($file in @('FlClash.exe','FlClashCore.exe','flutter_windows.dll','unins000.exe')) { if (-not (Test-Path -LiteralPath (Join-Path $installRoot $file))) { throw ('Missing installed file: ' + $file) } }
    $report.install=$true
    $coldUrl = 'https://example.invalid/subscription?token=smoke-cold-' + $env:GITHUB_RUN_ID + '&flag=meta'
    $coldLink = 'flclash://install-config?name=WonderX&url=' + [Uri]::EscapeDataString($coldUrl)
    $app = Start-Process -FilePath $exe -WorkingDirectory $installRoot -WindowStyle Hidden -PassThru -ArgumentList ('"' + $coldLink + '"') -RedirectStandardOutput $stdout -RedirectStandardError $stderr
    $report.cold_evidence = Wait-Link ('smoke-cold-' + $env:GITHUB_RUN_ID)
    $report.cold=$true
    $report.confirmation = Dismiss-Confirmation
    $warmUrl = 'https://example.invalid/subscription?token=smoke-warm-' + $env:GITHUB_RUN_ID + '&flag=meta'
    $warmLink = 'flclash://install-config?name=WonderX&url=' + [Uri]::EscapeDataString($warmUrl)
    $registry = Get-ItemProperty -LiteralPath 'Registry::HKEY_CURRENT_USER\Software\Classes\flclash\shell\open\command'
    if ($registry.'(default)' -ne ('"' + $exe + '" "%1"')) { throw 'Installed client did not register quoted flclash scheme.' }
    Start-Process -FilePath $warmLink
    $report.warm_evidence = Wait-Link ('smoke-warm-' + $env:GITHUB_RUN_ID)
    $report.warm=$true
    $report.confirmation = (Dismiss-Confirmation) -or $report.confirmation
    $ownedApps = @(Owned-Processes | Where-Object { $_.Name -eq 'FlClash.exe' })
    if ($ownedApps.Count -ne 1 -or $ownedApps[0].ProcessId -ne $app.Id) { throw 'Warm import created a second running instance.' }
    $report.same_instance=$true
    $logs = Read-Logs
    $report.logged_inner_flag = ($logs -match 'onAppLink:.*flag%3[Dd]meta') -or $report.cold_evidence -eq 'import-dialog'
    $report.passed=$true
} finally {
    $report | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath (Join-Path $smokeRoot 'report.json') -Encoding utf8
    $report | ConvertTo-Json -Depth 8
    Owned-Processes | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }
}
