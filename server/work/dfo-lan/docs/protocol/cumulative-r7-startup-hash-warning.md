# 累计 R7 启动哈希差异提示

2026-10-11 用户要求：合并后放宽启动验证，哈希不同仍可启动，只提示不一致。

GUI 2.0.7 的 `Core.Prepare/Validate` 实际计算发行文件 SHA256，并把警告传给运行日志。`ReleaseManifest.Validate` 列出文件、记录值和实际值，不抛出哈希不一致错误。`--verify-release` 同样读取实际哈希。

Go `LaunchPlan` 的 `releaseHashWarnings/startupSourceHashWarnings` 向直接入口与图形入口输出组件和 PVF/profile/policy 差异，`startSession` 在起进程前显示警告。哈希差异不影响返回值。

网关 `prepareRuntime` 传入 `CatalogInputs.StartupHashWarnings=true`；`gamedata.Open` 在实际归档上派生当前资源身份并记录不一致，角色 policy 仅在内存副本上绑定实际资源。`savecontract`、schema、所有玩法规则、角色容量和事务保持。假人管理运行时重开同源 PVF 使用同一策略。审计工具默认仍严格匹配，不放宽派生缓存来源、归档解密/解析、存档备份、安装载荷和回退校验。

实际全量 build/vet/test 4780 通过、319 条件跳过、0 失败；R6 已通过测试逐名无退化。帮助文本金样同步新的哈希告警说明后重跑全量。GUI 30 项及实际 GUI/Go 入口在故意不一致的发行记录与 PVF/profile/policy 上返回 0、显示告警、保持记录和合成存档字节。

使用有效 R5 原生归档的实际不同 SHA256，对照 R6 预期 SHA256 与角色 policy，实际构建网关告警后完整准备 61 域，未访问玩家库或启动监听/游戏。归档只追加无效尾部的负例仍被解析器拒绝，不把解析损坏当成单纯发行哈希差异。

原45G/R2/R6合成玩家安装、备份、重复安装、精确回退通过，容量28、四阶誓约与未来字段回退保持。R6 客户端资源、DLL、可选输入、安装器和所有权策略字节一致。客户端16GiB与90%提示保持。完整包压缩、CRC、实际解压逐文件哈希通过；本机安装另有全部存档表/schema/typed-row验证。未代用户启动游戏，原实机验收边界保持。

证据：`G:/电脑问题/45G累计更新制作-20261010/additional-r7-20261011/evidence/`；本机实际结果：同工作区 `live-install/live-installation-result.json`。后续合并以本冻结源码为本机基线，不从共享开发源重新覆盖。
