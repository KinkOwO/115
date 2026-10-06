# DFO 服务端源码与启动脚本交接包

## 工具命令入口（2026-10-03）

`work/dfo-lan/cmd` 现保留 `wireprobe`、`admin`、`gmtool`、`dfo-tool` 四个入口。原独立导出/审计/维护工具统一改为 `go run ./cmd/dfo-tool <原工具名> <参数>`，在 `work/dfo-lan` 下用 `go run ./cmd/dfo-tool -h` 查看清单。现有游戏启动和GM入口保持；构建工具程序用 `go build -trimpath -o bin/dfo-tool.exe ./cmd/dfo-tool`。详见 [cmd/README.md](work/dfo-lan/cmd/README.md)。

实际工具已从59个裁减到20个：39个无当前外部执行调用的旧导出/一次性调查/重复工具已删除，不能继续按历史工具名调用；具体范围与保留理由见 [工具裁减记录](../docs/todo/server-tool-pruning-20261003.md)。启动器仅构建wireprobe，不依赖这些离线工具；运行内容仍直接由同一PVF Source准备。

## MR !139 合并后的首次使用（2026-10-03源码收口）

本轮删除旧内容配置并调整严格policy字段，必须配套当前MR源码构建。旧bin不随Git源码更新；在仓库根目录执行：

```powershell
$env:GOTOOLCHAIN = 'go1.26.5'
pwsh -NoProfile -File ./server/Build-Server.ps1
./scripts/启动游戏.cmd --source-build
```

只启动服务端时，最后一行改为 `./scripts/启动服务端.cmd --source-build`。启动前手动关闭原游戏会话；构建脚本执行测试/vet并生成 `wireprobe-handoff-source.exe`，已有默认程序保留，首次缺少默认程序时会补齐。用户完成实机验收后，再运行 `pwsh -NoProfile -File ./server/Build-Server.ps1 -UpdatePVFDefault` 更新日常默认程序。39归档和现有数据库保持。

本轮合入上游61106a0e后的全量测试/vet、25项Python检查、真实PVF默认54域准备通过。下面旧程序身份属于既有实机基线，不冒充本轮构建；新交互仍需手动验收。上游MR集成树含63份顶层JSON，下一批清理未实施。


交付日期：2026-09-12。目标客户端：DFO 2.38.2.34，Windows x64。

这个包用于继续开发本地兼容服。包含 **Go 源码、完整导出配置、启动脚本、原39版服务端程序、补齐后重新编译的源码版、探针程序及源码、开发记录**。不是完整游戏安装包，不包含玩家数据库、密码、运行抓包、Go/Python/PostgreSQL 安装包，也不包含完整客户端。

## 默认PVF与历史运行版本

| 文件 | 用途与验证边界 |
|---|---|
| `work/dfo-lan/bin/wireprobe-pvf.exe` | SHA256 bc6211802e361f1407a14fd62d7a5730be9ed5250851a7f4c6c48aea6d32310e，第四批剩余投影缓存已确认，三个根入口默认使用；保持54选择项/63投影、历史存档准入及来源自动派生。 |
| `work/dfo-lan/bin/wireprobe-dungeon39.exe` | 原39版归档程序。前一任务的对接记录记载装备显示、重登保留和不崩已经用户确认；本次打包没有重做该实机验收。保留作历史回退。 |
| `work/dfo-lan/bin/wireprobe-handoff-source.exe` | SHA256 bc6211802e361f1407a14fd62d7a5730be9ed5250851a7f4c6c48aea6d32310e，与默认入口相同，含上游SHA身份修复及已确认的第四批剩余优化。 |
| `work/dfo-lan/bin/wireprobe-dungeon37.exe` | 历史回退参考；使用它时必须同时选择相匹配的配置。 |

**没有附会导致入城崩溃的38版EXE。** 文件名 `next38-equipment-display.md` 记载的是修复到39版的结果，不代表应该启动38版。

当前确认bc6211802e361f1407a14fd62d7a5730be9ed5250851a7f4c6c48aea6d32310e支持来源自动派生、归档元数据/联合物品缓存及七类投影缓存，用户已连续两轮手动启动并通过角色1第46帧入场预检；日常使用根入口即可。旧2e4bd343不支持空校验配置，仅保留历史匹配组合回退。启动脚本缺少内层manifest时可能重建资源，当前用户已生成be95d64e内层及manifest；原7ef离线采样不扩展为新归档性能数据。

第四批联合物品缓存阶段的历史样本为准备40.49→31.15秒，首次建缓存42.23秒，保留堆基本持平；源码c6b2bace备份于work/dfo-lan/.tmp/pvf-phase4/bin/wireprobe-handoff-source.confirmed-before.exe。当前其余投影缓存也已确认，完整阶段记录见../docs/todo/pvf/PVF启动与内存优化实施计划.md。

归档元数据缓存已确认：用户确认速度提升。17:50会话两类缓存miss/stored，准备50.244秒，角色4的45帧入场预检通过；17:53会话两类缓存hit，元数据2.361秒、联合物品2.647秒、全部准备25.704秒，角色11的49帧预检通过。正式/源码程序均核对为46c349cd8151ea66b9f056ce32f1c9f63368ee4ec6cc448205fbd713a062f7d8，无需替换，纳入confirmed baseline。确认依据用户反馈及上述日志，不扩大为所有玩法逐项验收；不改schema、存档准入/profile/客户端资源。按授权提交本段，再拉取合并上游SHA相关更新，继续其它投影及缓存保留策略。下文候选状态为历史记录。

第四批剩余项已确认：七类确定性投影缓存（装备绑定/掉落/副本/赛季/背景券/传送/终场剧情）及旧缓存保留策略，绑定实际PVF/完整程序/实际输入策略，损坏重建、不可写回退及私有查询索引恢复。424216条装备、18387张地图、七类全部字段和54/63冷热启动一致；全量Go测试/vet、独立PostgreSQL16存档身份迁移回归通过。已提交确认段3def161并以2c24faa合并上游07e1551。用户手动连续两轮启动源码入口并确认：18:21:24冷轮准备48.0247秒，九类缓存miss/stored，角色1第46帧entry_preflight_passed；18:23:49热轮准备14.4253秒，九类缓存hit，角色1第46帧entry_preflight_passed。正式入口与源码入口均核对为SHA256 bc6211802e361f1407a14fd62d7a5730be9ed5250851a7f4c6c48aea6d32310e，现纳入confirmed baseline。热轮相较此前确认热轮23.9059秒快39.44%、累计分配降低61.84%；首次建九文件48.51秒，热堆527.47→536.01MiB，缓存合计约191MiB。实机确认范围为连续两次启动及选角进入前置检查，未扩大为所有玩法逐项验收。

上游包含存档身份契约迁移：新源码启动后旧46c349cd默认程序不能直接作为回退。优先保持源码入口并设置DFO_PVF_CACHE_DIR='-'恢复原生导入；若需撤回本批实现，关闭会话后将.tmp/pvf-phase4c/bin/wireprobe-metadata-sha-compatible.exe复制到源码入口，再继续--source-build。该185ae7e99853d2b4d96a1043c47eb046279cfb6d777e536e7e1df6c1d46ba92f程序来自合并提交2c24faa，含上游身份修复及已确认元数据/物品缓存，不含本批七类投影；54/63离线完整报告与候选一致，未操作玩家数据库。46c349cd精确备份.tmp/pvf-phase4c/bin/wireprobe-handoff-source.confirmed-before.exe仅作迁移前历史快照。用户已手动关闭会话后启动游戏.cmd --source-build连续两轮并确认；默认入口与源码入口均已核对为当前SHA。

## 首次启动

归档元数据候选在已确认物品缓存基础上，本机单次准备33.14→26.18秒，保留堆基本持平；首次建立两类缓存51.10秒，元数据文件约162MiB。关闭会话后用--source-build两轮验证，gateway.err热轮同时出现archive metadata cache hit及derived item cache hit；DFO_PVF_CACHE_DIR='-'同时禁用两种缓存。旧源码8d6f979a备份work/dfo-lan/.tmp/pvf-phase4b/bin/wireprobe-handoff-source.confirmed-before.exe，详细证据及剩余范围见../docs/todo/pvf/PVF启动与内存优化实施计划.md。上文第四批首段记录为历史验证轮次。

1. 解压到固定目录，如 `D:/DFO-dev`。**不需要任何数据库服务**：2026-10-05 起 PostgreSQL 支持已整体移除，存档就是包内 work/dfo-lan/runtime/storage/dfolan.sqlite3 一个 SQLite 文件（服务端首次启动自己创建并建表）。继续编译还需要 Go 1.26（本包用 1.26.5 验证）；**运行不需要 Python**（启动链全部是 Go）。
2. 向项目提供者取得**完整的、当前能运行的隔离客户端目录**：原工作区 `work/dfo_probe_client`，包括资源和配套文件。可以放到解压目录的同名位置，也可放在其他磁盘。仅复制DFO.exe、PVF、sk.dat三个文件不够。配套校验值见 `client-requirements.json`。
3. 将 `launcher.example.json` 复制为 `launcher.local.json`。编辑 `client_dir` 为客户端目录，相对路径以解压根目录为基准，或填写绝对路径。Windows JSON路径建议用 `/`。
4. 存储档无需初始化：把 `work/dfo-lan/runtime/storage/local.example.json` 复制成 `local.json`，
   确认里面的 `sqlite_path` 是本机的绝对路径即可（相对路径会被服务端明确拒绝）。库文件由服务端首次
   启动创建，重复启动幂等。

> `dfolauncher init-storage` 已随 PostgreSQL 支持一起删除；历史 `pgdata/` 只是留档，
> 可救路径见 `work/dfo-lan/docs/sqlite-operations.md` §3。

5. 先检查，再启动（`launch --check` 只读，打印 Storage / Binary / Data mode / Client 四行）：

```powershell
.\bin\dfolauncher.exe launch --check
```

检查通过后，以管理员身份运行 `scripts\启动游戏.cmd`（或 `scripts\启动游戏-SQLite.cmd`）。入口用仓库内的 Go 启动器拉起网关与客户端；SQLite 是文件，没有服务要起。服务端启动时迁移表结构并建立开发账号 `probe`；角色由客户端创建。不会带入原机6666或LanTest01的存档。

## 修改源码与测试

在解压根目录执行：

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File ./Build-Server.ps1
```

若Go不在PATH，给脚本加 `-Go 'D:/tools/go/bin/go.exe'`。脚本依次执行 `go test ./...`、`go vet ./...`、编译源码版；首次构建补齐bin/wireprobe-pvf.exe，已有确认PVF程序默认保留。后续已验收构建使用-UpdatePVFDefault更新默认PVF；**不覆盖原39版**。首次编译可能需要下载 `go.mod/go.sum` 中的依赖，包中没有vendor。

测试源码候选版：关闭同一个测试会话后，在管理员PowerShell运行：

```powershell
.\Start-DFO.cmd --source-build
```

数据库集成回归在 `work/dfo-lan` 下运行 `go run ./cmd/dfo-tool charactercheck`。先确保自己的存储已经配置并启动；它使用临时schema。打包时没有为该检查连接朋友的环境，也没有复制原机数据库。

## 限制与排障

- 目前仍是**回环地址开发服**：频道目录127.0.0.1:7001，游戏监听127.0.0.2的动态端口，固定开发账号。多人局域网账号登录器、共享战斗同步等还不是完成品，不能只把监听改成0.0.0.0就当多人完成。
- 7001占用时检查是否已有会话。每次启动在 `work/dfo-lan/runtime/roles_..._next37/` 下记录 `run.json`、`events.jsonl`、`helper.err`；tag仍叫next37，默认EXE为wireprobe-pvf。
- 请解压后再启动，不要从压缩包内部运行。初始化新库后若移动目录，需更新自己 `runtime/storage/local.json` 的 `postgres_data` 和工具路径。
- 默认关闭内存观察器。日常启动只需要Python标准库；`reference/analysis-tools` 的历史分析脚本可能需要pefile/capstone/unicorn/cryptography，且含原机路径，**不能直接批量执行**。
- 原机启动脚本只作对照，位于 `reference/original-launcher`，不要用它代替本包根目录的新入口。
- 功能和协议详细交接看 `开发对接文档.md`。文件校验看 `MANIFEST.sha256`、`package-manifest.json`。
