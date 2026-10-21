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
保留 preview-flclash-091-merge.yml 作为当前源码实体化入口，禁止无故重跑旧 prepare。
未删除历史运行日志、分支、Release 或可用回滚包；停用入口可明确重新启用，删除文件可从 Git 提交恢复。
没有其他进行中的旧工作流需要取消。

## 后续验收
v1-091-validate.yml 直接检出固定完整源码 SHA，不在每次构建中重新拼接源码。
先运行 analyzer、行为/布局与核心平台回归、自定义协议测试，再修复真实失败。
最终同一源码 SHA 完成五轮验收，然后 Windows setup.exe/ZIP 与 Android arm64 APK。
当前不能声称五轮验收、安装启动或发布已完成。
Android 原签名 Secret 缺失，不能声称覆盖安装兼容。
源码仓库始终 PRIVATE；构建仓库临时 PUBLIC，交付后恢复 PRIVATE。
不修改任何服务器、订阅面板、DNS 或生产节点。
