# 2026-10-21：融合流程收敛
状态：RUNNING，尚未发布交付包。

## 唯一实施路线
官方 FlClash v0.9.1 的 UI、导航、设置与平台架构；
保留 WONDERX V1 名称、图标、验证过的自定义协议及必要功能逻辑。
私有源码分支：codex/flclash-0.9.1-complete-20261021。
首个完整源码候选：48d99d0cefaa972dafa072c3470e9bad881d0be2。
官方基线：8d4f9744948751f1273fe871e1d05276fac09b0e。
源码实体化运行：https://github.com/worldneedme/xxd-build/actions/runs/39613031457 。

## 本次清理
以下旧入口已停用，不再作为交付流程：
- 391193297 / sync-flclash-091-audit.yml：早期合并审计。
- 391280182 / replay-flclash-091-custom-fixes.yml：早期自定义修复重放审计。
- 391600721：旧 integration probe。
- 391606316：旧 official-base integration 候选构建。
- 391564347：旧 migration smoke。
删除重复入口 .github/workflows/materialize-v1-091.yml；
原 preview-flclash-091-merge.yml 已转为唯一的完整源码验收入口（workflow ID 391328108）；源码实体化阶段已完成，禁止重跑旧 prepare。
未删除历史运行日志、分支、Release 或可用回滚包；停用入口可明确重新启用，删除文件可从 Git 提交恢复。
没有其他进行中的旧工作流需要取消。

## 后续验收
preview-flclash-091-merge.yml 直接检出固定完整源码 SHA，不在每次构建中重新拼接源码。重复的 v1-091-validate.yml 也已删除，只保留一个入口。
先运行 analyzer、行为/布局与核心平台回归、自定义协议测试，再修复真实失败。
最终同一源码 SHA 完成五轮验收，然后 Windows setup.exe/ZIP 与 Android arm64 APK。
当前不能声称五轮验收、安装启动或发布已完成。
Android 原签名 Secret 缺失，不能声称覆盖安装兼容。
源码仓库始终 PRIVATE；构建仓库临时 PUBLIC，交付后恢复 PRIVATE。
不修改任何服务器、订阅面板、DNS 或生产节点。

## 首轮记录
https://github.com/worldneedme/xxd-build/actions/runs/39613961183 ：FAILED。
静态检查发现 current_node_name.dart 未使用导入，已修复。
Go 包装层 go test . 失败，日志展示需定位具体断言，未跳过或放宽测试。
首页展示测试已迁移至节点名称和不泄露公网 IP 的契约，保留详情、重试和启停检查。
云端格式化运行：https://github.com/worldneedme/xxd-build/actions/runs/39614419868 。

## 当前接续点（2026-10-21 14:40 UTC）
- 唯一融合验收入口：391328108 / preview-flclash-091-merge.yml。
- 诊断运行 39616413528：FAILED；输入 9abeb8f210ac658a1c410bd8aa2feb37cbfc165c；Go wrapper、custom protocol parser/adapters、Juicity Hop、两套 Rust 检查 PASSED。Flutter 根套件 4276 passed / 12 failed / 6 skipped，插件/setup-hooks 尚未到达。
- 格式运行 39616561338：PASSED，未修改源码；不得算作冒烟回归。
- 已提交修复：MTU 断言保留 V1 1500；精确检测 URL 改为 V1 google.cn；dashboard 只统计竖向页面滚动（不把横向节点名跑马灯当成嵌套列表）；跑马灯为 horizontal + NeverScrollable，独立控制器的合理例外有行为断言。
- 新协议契约修复：customProxyTypes 与真实 parser 顺序一致；测试解析共享 case 的 naive/naiveproxy 两个别名并对每个别名读取真实 Option struct。没有删除协议、放宽必填字段断言或跳过类型。
- 源码输入 f75196f5013561df7bae5eaac19216ed55f13eea；云端生成/格式化运行：https://github.com/worldneedme/xxd-build/actions/runs/39618825824 ，RUNNING。
- 该生成步骤只在 GitHub-hosted runner 翻转 hook 开关，生成后恢复 pubspec；完整测试继续保留 rust_api native hook。
- 用户授权的子智能体 /root/monitor_091_ci 正在只读监控，主智能体负责修复，不把排队当失败、不把格式化当回归。
- 下一步：待生成结束读取分支最新 SHA，再运行完整 Flutter/native/plugin 回归；最终源码五轮成功后才打包交付。
- 此任务无服务端修改、无生产修改。源码 PRIVATE、构建临时 PUBLIC；交付后恢复构建 PRIVATE。
