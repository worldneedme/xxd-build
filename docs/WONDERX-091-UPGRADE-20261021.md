# WONDERX V1 / FlClash 0.9.1 完整融合与交付记录

更新时间：2026-10-21 UTC。状态：RUNNING；未发布交付包。

## 目标与边界

采用官方 FlClash v0.9.1 的完整 UI、导航、设置、交互及平台集成作为基底，
保留 WONDERX V1 名称/图标、Mieru、Juicity/Hop、NaiveProxy、Sudoku HTTP-mask 等
自定义协议及必要逻辑；保留首页节点名/长名称跑马灯和 LAN 地址卡。
不整块覆盖回旧侧栏、旧背景、旧导航。不改服务端、面板、DNS、生产节点或 v1 域名。

源码仓库 worldneedme/xxdmanfl0609 始终 PRIVATE。
xxd-build 仅临时公开构建脚本、脱敏验收和产物，不公开私有源码树。
代码编辑通过 GitHub API；依赖、格式化、代码生成、测试、编译均在
GitHub-hosted runner。没有 VPS clone、SDK 安装、本地构建或产物下载。

## 固定来源

- 交付候选分支：codex/flclash-0.9.1-complete-20261021。
- 当前固定源码：c34637e714bbf3bdd429043de4f69d4ac18b8b2b。
- 官方 v0.9.1：8d4f9744948751f1273fe871e1d05276fac09b0e。
- 首个完整实体化候选：48d99d0cefaa972dafa072c3470e9bad881d0be2。
- 可用旧版回滚：codex/dashboard-direct-fix / 980ce42355b15ecb208c8e502587726e7e83622f。
- 当前五轮/打包 workflow：bbd26c1dd69e39738c271d45adf15ca96f6ca8fc。
- 当前验收运行：https://github.com/worldneedme/xxd-build/actions/runs/39622054524 。

官方 v0.9.0 与 v0.9.1 的 Mihomo gitlink 同为
8380d0e7462f061a964323e9bf3129a236fcc5d5。
本轮没有用一个未经验证的上游纯核心替换已验证的自定义 Mihomo；
迁入官方新版 wrapper、gomobile/IPC、生命周期和 UI 架构，再保留自定义协议树。
不能将其描述为所有第三方协议依赖都升级到了各自最新版。

## 实施步骤与已解决的问题

1. 先核对官方来源和历史可用源码，保留回滚分支、既有 Release 和安装包。
2. 在托管 runner 生成并提交完整融合源码。后续检出固定 SHA，禁止每次打包重跑
   compose/prepare，避免“SHA 相同、实际源码不同”。
3. 迁入官方完整 UI 和平台架构；保留品牌、协议和指定主页逻辑。
4. 保留默认 UA 的 flclash-x juicity-hop/1 clash-verge，
   用户自定义 UA 仍权威；保留订阅 URL 清理和传输失败时直连回退，401 不绕过认证。
5. 新版 CurrentOutboundCard 展示节点名而非公网 IP；保留官方详情、重试、质量标记
   和启停操作，LAN 地址另卡显示。
6. 保留 helper 生命周期清理、TUN listener 恢复、默认 MTU 1500、
   delay 并发 8 与 google.cn/generate_204 检测地址。
7. 首轮静态失败：节点名组件未使用导入。删除无用导入、纠正注释，托管格式化。
8. Go wrapper 测试失败：仍期待上游 gstatic URL，与 V1 已保留默认值不符。
   修正为精确 google.cn 契约，保留原断言。
9. 完整 Flutter 首次报告 12 项失败：
   - 五项协议表单契约：Naive 两别名共用一个 Go case，旧测试只能读取单别名；
     类型清单顺序也未对齐。解析所有别名并逐别名核对真实 Option struct，
     调整表单类型顺序，不删协议、不跳过字段/凭据/必填检查。
   - 节点跑马灯 ScrollController：横向、程序驱动、NeverScrollable，不应接入
     纵向 sheet 控制器。按既有水平 group chips 模式加入限定例外，
     并新增方向与 NeverScrollable 行为断言，避免把例外误用于竖向页面。
   - dashboard 两项布局：全局 Scrollable 计数把跑马灯也算成页面滚动。
     限定竖向滚动对象，保留页面数量、大小、overflow 与滚动范围断言。
   - state_derived、DNS 两项：检测 URL fixture 与 V1 默认值不符。
     修正精确预期，不删除健康检查或延迟断言。
   - network_config：MTU fixture 仍期待 9000，修正精确预期为 V1 1500。
10. 对 models 的修改在托管 CI 执行 build_runner，生成后格式化；
    hook 的临时开关仅在生成阶段生效并恢复，完整测试保留 Rust native hook。
11. 运行完整 Flutter、bundled plugin、setup_hooks、Go wrapper、
    Android bridge vet、自定义 parser/adapter、Juicity Hop、Rust 两组件回归。
12. 同一最终源码连续五轮完整 Flutter 回归，原生 Go 测试 -count=5。
    这五轮是自动化回归，不代表用户设备的五轮真实节点连通测试。
13. 前置全部成功才编译 Windows amd64 EXE/ZIP 与 Android arm64-v8a APK。
14. Windows 校验 ZIP、核心 manifest SHA、Naive helper；独立目录静默安装
    真实 Inno Setup EXE；启动应用并确认 Core/IPC，另用独立管道验证
    length-prefixed JSON RPC，观察 20 秒稳定性；仅清理本轮进程和目录。
    新版同意书字段为 acceptedDisclaimerVersion，不沿用已失效的旧 bool。
15. Android 校验新版 gomobile Mobile/Seq binding、libclash.so、
    Rust native assets、libnaive.so、ABI、extractNativeLibs、APK 签名，
    运行 common/service/app JVM 测试。旧 libcore.so/旧 JNI checker 不适用。
16. 两平台分别保存受限附件和脱敏 manifest；总发布 job 等待两平台与所有检查，
    验证同 SHA/文件大小/校验值后创建独立真实预发布 Release，保留旧 Release。
17. 成功交付后查 API 中真实附件地址，恢复 xxd-build PRIVATE，
    清理本轮临时 SOURCE_WRITE_KEY 与 writer deploy key（不删既有读取 key）。

## 验收台账

| 阶段 | 状态 | 证据 |
| --- | --- | --- |
| 源码实体化 | PASSED | run 39613031457 |
| 生成与格式化 | PASSED | run 39618825824 |
| 完整 Flutter / plugin / setup_hooks | PASSED | run 39619272899 |
| Go wrapper / custom protocols / Rust | PASSED | run 39619272899 |
| 同 SHA 五轮验收 | RUNNING | run 39622054524；旧失败运行不计通过 |
| Windows 安装、启动、Core RPC | NOT_STARTED | 等待前置 |
| Android APK / JVM | NOT_STARTED | 等待前置 |
| 双平台真实 Release | NOT_STARTED | 禁止提前交付 |
| 恢复构建仓库 PRIVATE / 临时 writer 清理 | NOT_STARTED | 交付后执行 |

格式化或生成成功不能充当一轮回归。
Actions 编译成功不能充当安装/启动成功，更不能充当用户节点实测。

## 签名和设备侧限制

本构建仓库没有 Android 原发布 keystore/password Secrets。
当前 APK 采用工程已有的 .dev 测试包身份，不保证覆盖安装既有应用或继承设置；
不得把它说成原签名正式包。签名证书摘要随附件提供。

Windows 用户电脑、Android 手机、iPhone 小火箭实际安装/订阅/连接：
PENDING_DEVICE_TEST。无真实用户订阅、服务端和生产切换测试。

## 回滚与接续

保留旧版本 Release，异常时停止推荐新包并回到旧包，不覆盖旧标签。
源码和 workflow 修改均是普通提交，可针对本轮提交 revert；不强推/硬回退整条分支。
旧流程仅停用/删除重复入口，历史提交和运行证据保留，可精确恢复。
接续先读 docs/WONDERX-091-ACTIVE-FLOW.md 与本文，再查固定运行状态；
不要从旧分支重新拼接或把旧临时产物重命名成新版本。


## 15:02 UTC：五轮状态泄漏定位与重验
- 39620764738：Go wrapper 五轮失败；已取消未完成的其他 job，不计验收通过。
- 根因：core/route_test.go 的 TestRouteEpochFollowsStructuralChanges 调用 updateConfig 切换 Global 后没有恢复 tunnel.Mode。第一次顺序运行可过，第二轮 probe 只有 DIRECT fixture，却残留 Global 模式，导致 tunnel.resolveMetadata 取不到 GLOBAL，在 DialContext 崩溃。
- 最小修复：测试保存原 mode，并在 t.Cleanup 恢复。未更改 Mihomo、协议、生产逻辑；未减少轮数、未删除/放宽断言。重复测试继续覆盖此问题。
- 当前不可变客户端源码：c34637e714bbf3bdd429043de4f69d4ac18b8b2b。
- 当前 workflow：bbd26c1dd69e39738c271d45adf15ca96f6ca8fc；Go verbose 输出保留失败测试名。
- 当前运行：https://github.com/worldneedme/xxd-build/actions/runs/39622054524 。五轮完整 Flutter + native/plugin 验收，全部成功才开始双平台打包，再检查 Windows 安装/启动/Core RPC、Android JNI/helper/JVM，最后成对发布。
- 发布标签预留 wonderx-v1-0.9.1-official-ui-20261021-1455；目前无交付 Release，不得当作下载链接。
- 子智能体只读监控；主智能体负责诊断和修复。所有编辑走 GitHub API，测试/编译均 GitHub-hosted runner，无 VPS 本地构建。
