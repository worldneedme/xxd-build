$ErrorActionPreference = 'Stop'
$installRoot = Join-Path $env:RUNNER_TEMP 'wonderx-091-install'
if (-not $installRoot.StartsWith($env:RUNNER_TEMP, [StringComparison]::OrdinalIgnoreCase)) {
  throw 'Refusing to clean outside the isolated runner temp directory.'
}
if (Test-Path -LiteralPath $installRoot) {
  $owned = @(Get-CimInstance Win32_Process | Where-Object {
    $_.ExecutablePath -and $_.ExecutablePath.StartsWith($installRoot, [StringComparison]::OrdinalIgnoreCase)
  })
  foreach ($process in $owned) {
    Stop-Process -Id $process.ProcessId -Force -ErrorAction SilentlyContinue
  }
  $appSupportDirectory = Join-Path ([Environment]::GetFolderPath([Environment+SpecialFolder]::ApplicationData)) 'com.follow\WONDERX V1'
  $ownerMarker = Join-Path $appSupportDirectory '.ci-smoke-owner'
  if (Test-Path -LiteralPath $ownerMarker -PathType Leaf) {
    $ownerRunId = (Get-Content -LiteralPath $ownerMarker -Raw).Trim()
    if ($ownerRunId -ne $env:GITHUB_RUN_ID) {
      throw 'The application support directory belongs to a different test run; refusing to delete it.'
    }
    Remove-Item -LiteralPath $appSupportDirectory -Recurse -Force
  }
  $applicationNames = @('FlClash.exe', 'FlClashCore.exe', 'FlClashHelperService.exe')
  $foreign = @(Get-CimInstance Win32_Process | Where-Object {
    ($applicationNames -contains $_.Name) -and
    (-not $_.ExecutablePath -or -not $_.ExecutablePath.StartsWith($installRoot, [StringComparison]::OrdinalIgnoreCase))
  })
  if ($foreign.Count -gt 0) {
    throw 'A same-named process outside the test installation is present; refusing the installer uninstaller hook.'
  }
  $uninstaller = Join-Path $installRoot 'unins000.exe'
  if (Test-Path -LiteralPath $uninstaller -PathType Leaf) {
    $uninstall = Start-Process -FilePath $uninstaller -Wait -PassThru -WindowStyle Hidden -ArgumentList @(
      '/VERYSILENT', '/SUPPRESSMSGBOXES', '/NORESTART'
    )
    if ($uninstall.ExitCode -ne 0) { throw "Test installation cleanup failed with exit code $($uninstall.ExitCode)." }
  }
  if (Test-Path -LiteralPath $installRoot) { Remove-Item -LiteralPath $installRoot -Recurse -Force }
}

$shell = New-Object -ComObject WScript.Shell
foreach ($desktopRoot in @(
  [Environment]::GetFolderPath([Environment+SpecialFolder]::CommonDesktopDirectory)
  [Environment]::GetFolderPath([Environment+SpecialFolder]::DesktopDirectory)
) | Select-Object -Unique) {
  foreach ($name in @('V1.lnk', 'WONDERX V1.lnk')) {
    $linkPath = Join-Path $desktopRoot $name
    if (Test-Path -LiteralPath $linkPath -PathType Leaf) {
      $shortcut = $shell.CreateShortcut($linkPath)
      if ([String]::Equals($shortcut.TargetPath, (Join-Path $installRoot 'FlClash.exe'), [StringComparison]::OrdinalIgnoreCase)) {
        Remove-Item -LiteralPath $linkPath
      }
    }
  }
}
