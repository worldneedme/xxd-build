$ErrorActionPreference = 'Stop'
$setup = Get-ChildItem -Path '.\dist' -Recurse -Filter '*-windows-*-setup.exe' | Select-Object -First 1
$zip = Get-ChildItem -Path '.\dist' -Recurse -Filter '*-windows-*.zip' | Select-Object -First 1
if (-not $setup -or $setup.Length -le 0) { throw 'A non-empty Windows installer was not produced.' }
if (-not $zip -or $zip.Length -le 0) { throw 'A non-empty Windows ZIP was not produced.' }

$installRoot = Join-Path $env:RUNNER_TEMP 'wonderx-091-install'
$stdout = Join-Path $env:RUNNER_TEMP 'wonderx-091-app.stdout.log'
$stderr = Join-Path $env:RUNNER_TEMP 'wonderx-091-app.stderr.log'
if (-not $installRoot.StartsWith($env:RUNNER_TEMP, [StringComparison]::OrdinalIgnoreCase)) {
  throw 'Refusing to install outside the isolated runner temp directory.'
}
if (Test-Path -LiteralPath $installRoot) { throw 'The isolated install path already exists.' }

$applicationNames = @('FlClash.exe', 'FlClashCore.exe', 'FlClashHelperService.exe')
$preexisting = @(Get-CimInstance Win32_Process | Where-Object { $applicationNames -contains $_.Name })
if ($preexisting.Count -gt 0) {
  throw 'A same-named FlClash process already exists on the runner; refusing an installer whose hook terminates by image name.'
}

$install = Start-Process -FilePath $setup.FullName -Wait -PassThru -WindowStyle Hidden -ArgumentList @(
  '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART', '/SP-', "/DIR=$installRoot"
)
if ($install.ExitCode -ne 0) { throw "Inno Setup installer failed with exit code $($install.ExitCode)." }

$requiredFiles = @(
  'FlClash.exe', 'FlClashCore.exe', 'FlClashHelperService.exe', 'manifest.json', 'unins000.exe',
  'flutter_windows.dll', 'data\icudtl.dat', 'protocol-helpers\naive.exe'
)
foreach ($relative in $requiredFiles) {
  if (-not (Test-Path -LiteralPath (Join-Path $installRoot $relative) -PathType Leaf)) {
    throw "Installed application is missing $relative"
  }
}
if (-not (Test-Path -LiteralPath (Join-Path $installRoot 'data\flutter_assets') -PathType Container)) {
  throw 'Installed application is missing data/flutter_assets.'
}
if (Get-ChildItem -LiteralPath $installRoot -Recurse -Filter 'juicity-client.exe' -ErrorAction SilentlyContinue) {
  throw 'Juicity is native in FlClashCore and must not ship as a helper executable.'
}

$exe = Join-Path $installRoot 'FlClash.exe'
$versionInfo = [Diagnostics.FileVersionInfo]::GetVersionInfo($exe)
if ($versionInfo.CompanyName -ne 'com.follow' -or $versionInfo.ProductName -ne 'WONDERX V1') {
  throw 'The installed executable has unexpected Windows application-data identity metadata.'
}
$appSupportDirectory = Join-Path ([Environment]::GetFolderPath([Environment+SpecialFolder]::ApplicationData)) 'com.follow\WONDERX V1'
if (Test-Path -LiteralPath $appSupportDirectory) {
  throw 'The isolated runner already has an application support directory; refusing to overwrite its preferences.'
}
New-Item -ItemType Directory -Path $appSupportDirectory | Out-Null
$preferences = @{ 'flutter.config' = '{"appSettingProps":{"autoCheckUpdate":false},"themeProps":{}}' }
$preferences | ConvertTo-Json -Compress | Set-Content -LiteralPath (Join-Path $appSupportDirectory 'shared_preferences.json') -NoNewline -Encoding utf8
Set-Content -LiteralPath (Join-Path $appSupportDirectory '.ci-smoke-owner') -Value $env:GITHUB_RUN_ID -NoNewline -Encoding ascii
$preexistingPipeNames = @([IO.Directory]::GetFiles('\\.\pipe\') | ForEach-Object { [IO.Path]::GetFileName($_) })
$app = Start-Process -FilePath $exe -WorkingDirectory $installRoot -WindowStyle Hidden -PassThru -RedirectStandardOutput $stdout -RedirectStandardError $stderr
$appPid = $app.Id
$corePid = $null
$pipeName = $null
$deadline = (Get-Date).AddSeconds(30)
while ((Get-Date) -lt $deadline) {
  $owned = @(Get-CimInstance Win32_Process | Where-Object {
    $_.ExecutablePath -and $_.ExecutablePath.StartsWith($installRoot, [StringComparison]::OrdinalIgnoreCase)
  })
  $appProcess = $owned | Where-Object { [IO.Path]::GetFileName($_.ExecutablePath) -ieq 'FlClash.exe' } | Select-Object -First 1
  $coreProcess = $owned | Where-Object { [IO.Path]::GetFileName($_.ExecutablePath) -ieq 'FlClashCore.exe' } | Select-Object -First 1
  $pipes = @([IO.Directory]::GetFiles('\\.\pipe\') | Where-Object {
    $name = [IO.Path]::GetFileName($_)
    $name -like 'FlClashCore_*' -and $preexistingPipeNames -notcontains $name
  })
  if ($appProcess -and $coreProcess -and $pipes.Count -gt 0) {
    $appPid = [int]$appProcess.ProcessId
    $corePid = [int]$coreProcess.ProcessId
    $pipeName = [IO.Path]::GetFileName($pipes[0])
    break
  }
  Start-Sleep -Seconds 1
}
if (-not $corePid -or -not $pipeName) {
  throw 'The installed app did not keep its process and initialize FlClashCore IPC within 30 seconds.'
}

function Read-ExactBytes {
  param([IO.Stream]$Stream, [int]$Count)
  $buffer = New-Object byte[] $Count
  $offset = 0
  while ($offset -lt $Count) {
    $readTask = $Stream.ReadAsync($buffer, $offset, $Count - $offset)
    if (-not $readTask.Wait(10000)) { throw 'Timed out waiting for an installed Core RPC frame.' }
    $read = [int]$readTask.Result
    if ($read -le 0) { throw 'Core RPC pipe closed before a complete frame arrived.' }
    $offset += $read
  }
  return ,$buffer
}

# The app's pipe uses length-prefixed JSON-RPC, not HTTP. Keep its live
# channel untouched and probe the installed Core through a separate pipe.
$probePipeName = "FlClashCore_CI_$([Guid]::NewGuid().ToString('N'))"
$probePipeAddress = "\\.\pipe\$probePipeName"
$probeStdout = Join-Path $env:RUNNER_TEMP 'wonderx-091-core-probe.stdout.log'
$probeStderr = Join-Path $env:RUNNER_TEMP 'wonderx-091-core-probe.stderr.log'
$probeServer = $null
$probeCore = $null
try {
  $probeServer = [System.IO.Pipes.NamedPipeServerStream]::new(
    $probePipeName,
    [System.IO.Pipes.PipeDirection]::InOut,
    1,
    [System.IO.Pipes.PipeTransmissionMode]::Byte,
    [System.IO.Pipes.PipeOptions]::Asynchronous,
    65536,
    65536
  )
  $connection = $probeServer.WaitForConnectionAsync()
  $probeCore = Start-Process -FilePath (Join-Path $installRoot 'FlClashCore.exe') `
    -ArgumentList @($probePipeAddress) -WorkingDirectory $installRoot -WindowStyle Hidden -PassThru `
    -RedirectStandardOutput $probeStdout -RedirectStandardError $probeStderr
  if (-not $connection.Wait(15000)) { throw 'A separately launched installed Core did not connect to its named pipe.' }

  $request = [Text.Encoding]::ASCII.GetBytes('{"id":"ci-smoke-1","method":"getIsInit"}')
  $requestLength = [BitConverter]::GetBytes([uint32]$request.Length)
  $probeServer.Write($requestLength, 0, $requestLength.Length)
  $probeServer.Write($request, 0, $request.Length)
  $probeServer.Flush()

  $responseLengthBytes = Read-ExactBytes -Stream $probeServer -Count 4
  $responseLength = [BitConverter]::ToUInt32($responseLengthBytes, 0)
  if ($responseLength -eq 0 -or $responseLength -gt 1048576) {
    throw "Installed Core returned an invalid RPC frame length: $responseLength"
  }
  $responseBytes = Read-ExactBytes -Stream $probeServer -Count ([int]$responseLength)
  $response = [Text.Encoding]::UTF8.GetString($responseBytes) | ConvertFrom-Json
  if ($response.id -ne 'ci-smoke-1' -or $response.PSObject.Properties.Name -contains 'error' -or
      $response.result -isnot [bool] -or $response.result -ne $false) {
    throw "Installed Core returned an unexpected getIsInit RPC response: $response"
  }
  $probeStillRunning = Get-CimInstance Win32_Process -Filter "ProcessId = $($probeCore.Id)" -ErrorAction SilentlyContinue
  if (-not $probeStillRunning -or -not $probeStillRunning.ExecutablePath -or
      -not $probeStillRunning.ExecutablePath.StartsWith($installRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'The isolated Core process exited or changed identity after its RPC response.'
  }
} finally {
  if ($probeServer) { $probeServer.Dispose() }
  if ($probeCore) {
    $probeStillRunning = Get-CimInstance Win32_Process -Filter "ProcessId = $($probeCore.Id)" -ErrorAction SilentlyContinue
    if ($probeStillRunning -and $probeStillRunning.ExecutablePath -and
        $probeStillRunning.ExecutablePath.StartsWith($installRoot, [StringComparison]::OrdinalIgnoreCase)) {
      Stop-Process -Id $probeCore.Id -Force -ErrorAction SilentlyContinue
    }
  }
}

for ($second = 0; $second -lt 20; $second++) {
  $appStillRunning = Get-CimInstance Win32_Process -Filter "ProcessId = $appPid" -ErrorAction SilentlyContinue
  $coreStillRunning = Get-CimInstance Win32_Process -Filter "ProcessId = $corePid" -ErrorAction SilentlyContinue
  if (-not $appStillRunning -or -not $coreStillRunning) {
    throw 'The installed app or core exited during the 20-second stability window.'
  }
  if (-not $appStillRunning.ExecutablePath.StartsWith($installRoot, [StringComparison]::OrdinalIgnoreCase) -or
      -not $coreStillRunning.ExecutablePath.StartsWith($installRoot, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'A process ID was reused by an executable outside the test install directory.'
  }
  Start-Sleep -Seconds 1
}
$savedPreferences = Get-Content -LiteralPath (Join-Path $appSupportDirectory 'shared_preferences.json') -Raw | ConvertFrom-Json
$savedConfig = $savedPreferences.'flutter.config' | ConvertFrom-Json
if ($savedConfig.appSettingProps.acceptedDisclaimerVersion -gt 0) {
  throw 'Fresh startup must not fabricate disclaimer acceptance.'
}
Write-Host 'Windows fresh-start smoke passed without pre-accepted terms: actual install, app/Core stability, read-only RPC and unchanged consent state.'

$startupLog = @(
  Get-Content -LiteralPath $stdout -ErrorAction SilentlyContinue
  Get-Content -LiteralPath $stderr -ErrorAction SilentlyContinue
  Get-Content -LiteralPath (Join-Path $env:RUNNER_TEMP 'wonderx-091-core-probe.stdout.log') -ErrorAction SilentlyContinue
  Get-Content -LiteralPath (Join-Path $env:RUNNER_TEMP 'wonderx-091-core-probe.stderr.log') -ErrorAction SilentlyContinue
)
$startupLog | Select-Object -Last 40
$fatal = $startupLog | Select-String -Pattern 'Unhandled Exception|FATAL|panic:' -CaseSensitive:$false
if ($fatal) { throw 'The installed app reported a fatal startup error.' }
