# BUG and Migration Log

## 1. NaiveProxy / Juicity Outbounds Timeout on Android

### 问题现象 (Symptom)
* 在 Android 端，虽然 `VLESS-XHTTP` 和 `Mieru` 协议能够正常连接并测试出延迟（如 109ms/115ms），但所有的 `NaiveProxy` 和 `Juicity` 节点均显示 `Timeout` 超时。
* 在 Windows 端，这些节点可以正常测试出延迟，但在 Android 端全部失效。

### 原因分析 (Root Cause)
1. **外部进程执行限制 (W^X Policy)**：
   * 在当前 `android` 分支中，`NaiveProxy` 和 `Juicity` 都是基于“外置辅助程序（helper-backed）”模式实现的。它们在启动时需要通过 `os/exec` 执行外部二进制程序 `naive` 或 `juicity-client`，并通过本地端口进行流量中转。
   * Android 10 (API level 29) 及以上版本出于安全考虑实施了 W^X (Write XOR Execute) 策略，禁止应用程序执行位于私有数据目录（如 `/data/data/...` 或 `files/` / `cache/`）中的外部二进制文件。
   * 外部可执行程序必须打包成 `libxxx.so` 存放在 APK 的 `lib/` 目录中，在安装时由系统安全解压并赋予可执行权限后方可运行。

2. **可执行文件路径解析失败 (Path Resolution)**：
   * `helper_proxy.go` 中的 `resolveHelperPath` 依赖于 `os.Executable()` 获取执行路径，并寻找 `protocol-helpers/` 目录。
   * 在 Android 环境下，`os.Executable()` 返回的是 Android 系统的应用进程启动器路径 `/system/bin/app_process`，这导致解析出的 helper 搜索路径变成了只读且不存在的 `/system/bin/protocol-helpers/`，因此无法定位到任何辅助程序。

3. **缺少 Android 架构二进制**：
   * 仓库中仅包含 Windows 端的 `naive.exe`，缺乏针对 Android (arm64-v8a / armeabi-v7a) 编译的 `naive` 和 `juicity-client` 二进制包。

### 解决方案与优化策略 (Proposed Solution)

#### 1. Juicity 协议：原生化 (Native Outbound)
* **策略**：将 `win-native-outbound` 分支中已经实现的 **Native Juicity** 代码合并到 `android` 分支中。
* **说明**：Native Juicity 引入了 Go 语言原生实现的 Juicity 协议库（`github.com/daeuniverse/outbound/protocol/juicity`），使 Clash.Meta/Mihomo 核心可以直接在内存中建立 Juicity 隧道，**无需调用任何外部 `juicity-client` 进程**。这能够完美、彻底地解决 Android 上的 Juicity 连接超时问题。

#### 2. NaiveProxy 协议：外置辅助程序路径适配与打包 / 协议降级
* **策略**：
  * **合并并解决编译**：合并 `win-native-outbound` 分支以确保代码库统一。
  * **分析说明**：由于 NaiveProxy 在 Mihomo 中暂无纯原生 Go 实现，仍需依赖外部 C++ 编写的 `naive`。要完全在 Android 跑通 NaiveProxy，需要交叉编译 Android (arm64-v8a, armeabi-v7a) 版本的 `naive` 并重命名为 `libnaive.so` 打包在 APK 的 `jniLibs` 中，且 Go 代码中需要通过 JNI/环境传参获取 Android native library 目录进行调用。

---

### Juicity Timeout 修复 (v0.8.93-android-test.17)
* **Native Juicity 合并**：将 `win-native-outbound` 分支中基于 `github.com/daeuniverse/outbound/protocol/juicity` 的原生 Go 实现代码合并至 `android` 分支。
* **解决编译冲突**：
  * 修改了 [helper_proxy.go](file:///d:/code/projects/flclash-android-test/core/Clash.Meta/adapter/outbound/helper_proxy.go)，移除了 Android 平台上的 helper-backed Juicity 逻辑（避免由于 Android 平台无法执行外部二进制导致的超时）。
  * 引入了原生 Go 实现的 `Juicity` 协议，直接编译进 `libcore.so`，摆脱外部辅助进程 `juicity-client` 的依赖。
* **效果**：在 Android 平台上直接以内核原生模块形式运行 Juicity 协议，规避了 Android W^X 安全策略以及 `os.Executable()` 在 Android 下返回 `/system/bin` 导致的辅助程序路径寻址失败问题。

---

## 3. NaiveProxy 待处理事项 (NaiveProxy Pending Items)
* **现状**：NaiveProxy 当前仍依赖外部 C++ `naive` 辅助程序。在 Android 平台上依然会遇到 W^X 策略限制导致的 `Timeout` 现象。
* **后续优化方案**：
  1. 交叉编译 Android 架构（arm64-v8a, armeabi-v7a 等）的 `naive` 二进制，并以 `libnaive.so` 的形式打包入 APK 的 `jniLibs` 中。
  2. 修改 Go 端的 `helper_proxy.go` 中的路径解析逻辑，在 Android 环境下读取应用私有的 Native Library 目录（即 `Context.getApplicationInfo().nativeLibraryDir`）来定位并执行 `naive` 辅助程序。

