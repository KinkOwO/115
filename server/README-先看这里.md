# DFO 服务端源码与启动脚本交接包

交付日期：2026-09-12。目标客户端：DFO 2.38.2.34，Windows x64。

这个包用于继续开发本地兼容服。包含 **Go 源码、完整导出配置、启动脚本、原39版服务端程序、补齐后重新编译的源码版、探针程序及源码、开发记录**。不是完整游戏安装包，不包含玩家数据库、密码、运行抓包、Go/Python/PostgreSQL/Redis 安装包，也不包含完整客户端。

## 默认PVF与历史运行版本

| 文件 | 用途与验证边界 |
|---|---|
| `work/dfo-lan/bin/wireprobe-pvf.exe` | 8d6f979a联合物品磁盘缓存已确认，三个根入口默认使用；保持54选择项/63投影、历史存档准入及来源自动派生。 |
| `work/dfo-lan/bin/wireprobe-dungeon39.exe` | 原39版归档程序。前一任务的对接记录记载装备显示、重登保留和不崩已经用户确认；本次打包没有重做该实机验收。保留作历史回退。 |
| `work/dfo-lan/bin/wireprobe-handoff-source.exe` | 8d6f979a已确认，与默认一致；完整字段及54/63冷/热启动和Go测试/vet通过，用户确认运行正常且二次启动加快。 |
| `work/dfo-lan/bin/wireprobe-dungeon37.exe` | 历史回退参考；使用它时必须同时选择相匹配的配置。 |

**没有附会导致入城崩溃的38版EXE。** 文件名 `next38-equipment-display.md` 记载的是修复到39版的结果，不代表应该启动38版。

当前确认8d6f979a支持来源自动派生及联合物品缓存，选角/二次启动已确认，日常使用根入口即可。旧2e4bd343不支持空校验配置，仅保留历史匹配组合回退。启动脚本缺少内层manifest时可能重建资源，当前用户已生成be95d64e内层及manifest；原7ef离线采样不扩展为新归档性能数据。

第四批候选首次启动建立runtime/pvf-cache，第二次同源同程序命中；本机单次目录准备40.49→31.15秒，首次建缓存42.23秒，保留堆基本持平。用户手动关闭会话后用--source-build连续两次检查选角及物品/任务/进房。DFO_PVF_CACHE_DIR可指定目录，-禁用；缓存可删除后重建，程序/源/强化策略变化会失效。源码c6b2bace备份于work/dfo-lan/.tmp/pvf-phase4/bin/wireprobe-handoff-source.confirmed-before.exe；其余投影缓存未完成，详见../docs/todo/pvf/PVF启动与内存优化实施计划.md。

## 首次启动

1. 解压到固定目录，如 `D:/DFO-dev`。准备 Windows x64 上可用的 Python 3.10+、PostgreSQL 和 Redis。继续编译还需要 Go 1.26（本包用1.26.5验证）。数据库工具需包含 `initdb.exe`、`pg_ctl.exe`、`createdb.exe`；Redis需有 `redis-server.exe` 及其配套依赖。
2. 向项目提供者取得**完整的、当前能运行的隔离客户端目录**：原工作区 `work/dfo_probe_client`，包括资源和配套文件。可以放到解压目录的同名位置，也可放在其他磁盘。仅复制DFO.exe、PVF、sk.dat三个文件不够。配套校验值见 `client-requirements.json`。
3. 将 `launcher.example.json` 复制为 `launcher.local.json`。编辑 `client_dir` 为客户端目录，相对路径以解压根目录为基准，或填写绝对路径。Windows JSON路径建议用 `/`。
4. 仅在朋友自己的电脑上初始化**新库**。从解压根目录打开 PowerShell，修改下方工具路径再运行：

```powershell
py -3 work/dfo-lan/scripts/bootstrap_local.py --postgres-bin 'D:/tools/pgsql/bin' --redis-bin 'D:/tools/redis'
```

这会在本包 `work/dfo-lan/runtime/storage` 内建立新PG数据目录和随机密码配置，PG端口25438、Redis端口26388。已有 `local.json` 或 `pgdata` 就拒绝初始化。初始化中途失败请查日志和现有数据，不要直接删除目录反复重试。

5. 先检查，再启动：

```powershell
py -3 work/dfo-lan/scripts/launch_local.py --check
```

检查通过后，右键根目录 `Start-DFO.cmd`，以管理员身份运行。脚本启动已有本地存储和默认PVF服务端，然后打开客户端。服务端启动时迁移表结构并建立开发账号 `probe`；角色由客户端创建。不会带入原机6666或LanTest01的存档。

若 `py` 不在PATH，可用 `python` 替代上述命令。双击入口支持 `DFO_PYTHON` 环境变量指向Python.exe；否则依次尝试 `py -3`、`python`。

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

数据库集成回归在 `work/dfo-lan` 下运行 `go run ./cmd/charactercheck`。先确保自己的存储已经配置并启动；它使用临时schema。打包时没有为该检查连接朋友的环境，也没有复制原机数据库。

## 限制与排障

- 目前仍是**回环地址开发服**：频道目录127.0.0.1:7001，游戏监听127.0.0.2的动态端口，固定开发账号。多人局域网账号登录器、共享战斗同步等还不是完成品，不能只把监听改成0.0.0.0就当多人完成。
- 7001占用时检查是否已有会话。每次启动在 `work/dfo-lan/runtime/roles_..._next37/` 下记录 `run.json`、`events.jsonl`、`helper.err`；tag仍叫next37，默认EXE为wireprobe-pvf。
- 请解压后再启动，不要从压缩包内部运行。初始化新库后若移动目录，需更新自己 `runtime/storage/local.json` 的 `postgres_data` 和工具路径。
- 默认关闭内存观察器。日常启动只需要Python标准库；`reference/analysis-tools` 的历史分析脚本可能需要pefile/capstone/unicorn/cryptography，且含原机路径，**不能直接批量执行**。
- 原机启动脚本只作对照，位于 `reference/original-launcher`，不要用它代替本包根目录的新入口。
- 功能和协议详细交接看 `开发对接文档.md`。文件校验看 `MANIFEST.sha256`、`package-manifest.json`。
