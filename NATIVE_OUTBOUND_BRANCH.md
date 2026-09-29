# Native Outbound Branch

Use `win-native-outbound` for native mihomo outbound work.

The `win` branch intentionally keeps the verified helper-backed implementation. Native outbound work must stay separate until it can pass:

- parser/config tests for `naive`, `naiveproxy`, and `juicity`
- 12 Windows protocol test rounds
- Windows package helper/runtime checks updated for the native implementation
- `FlClash.exe` start smoke
- proxy-page delay smoke without enabling the run switch

Do not publish a native Windows package until the Windows smoke gate passes.
