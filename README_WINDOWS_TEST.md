# FlClash TEST Windows

This branch is the Windows TEST build line.

## Branches

- `win`: Windows helper-backed build that currently supports parsing and delay testing for `naive`, `naiveproxy`, and `juicity`.
- `win-native-outbound`: separate branch reserved for the native mihomo outbound implementation. Do not merge native outbound experiments into `win` until they pass the same Windows smoke gate.
- `android`: Android line. Keep Android and Windows package work separated.

## Current Windows implementation

`NaiveProxy` and `Juicity` are exposed as mihomo adapters. The Windows runtime delegates `NaiveProxy` to a local helper executable, while `Juicity` is native in `FlClashCore.exe`:

- `windows/protocol-helpers/naive.exe`

The packaged app installs the NaiveProxy helper next to `FlClash.exe` under `protocol-helpers`.

The helper local SOCKS port is derived from the full node configuration fingerprint. This avoids reusing a stale helper after a subscription refresh changes server, credentials, SNI, hop ports, or other protocol fields while keeping the same node name.

## Verification

Local focused verification:

```powershell
$env:FLCLASH_TEST_NAIVE_HELPER = "D:\code\projects\flclash-core-test\app-copy\protocol-helpers\naive.exe"
cd D:\code\projects\flclash-core-test\src\FlClash\core\Clash.Meta
go test ./adapter ./config ./adapter/outbound -run 'TestParseHelperBackedProtocols|TestParseHelperBackedProtocolConfig|TestHelperBackedProxyStartsOfficialHelpers|TestNaiveProxyPasswordOnlyAndStablePort' -count=1 -timeout=60s
```

GitHub Actions verification:

- `.github/workflows/windows-test-release.yml`
- Runs the protocol tests 12 times.
- Builds the Windows zip.
- Verifies `FlClashCore.exe` and `naive.exe` are in the packaged bundle. Juicity is native in `FlClashCore.exe` and `juicity-client.exe` must not be packaged.
- Starts `FlClash.exe` once and fails the build if the process exits during smoke testing.

Create a release by pushing a tag named like `win-v0.8.93-test.1` after the workflow passes on `win`.
