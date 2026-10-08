# Python 退出清单（启动链已 Go-only，本文件记录剩余步骤与前置条件）

> 业主定调（2026-10-05）：**彻底移除所有外部环境依赖，包括 Python；不要再回退，必须 Go 成功。**
> 2026-10-05 本轮已完成「启动/停止链只走 Go」（提交 `0a73f67a`、`0ba38717`）。
> 本清单只登记**剩下要删/要改**的东西与**前置条件**，不重复已完成的实现说明。

## 1. 已经 Go-only 的部分（不要再改回 Python 回退）

| 环节 | 现状 |
| --- | --- |
| `scripts\storage-route.ps1` 的完整启动链 | 只有 `server\work\dfo-lan\bin\dfolauncher.exe launch [--server-only]`；缺二进制直接失败，不换启动器 |
| `scripts\启动游戏.cmd` | 调 `storage-route.ps1 game-current`（保持活动档，不切路线） |
| `scripts\停止游戏环境.cmd` | 只调 `bin\dfolauncher.exe stop` |
| 强制开关 | 启动时 `DFO_REQUIRE_GO_ISOLATION=1`：Go 宿主/Go WFP 隔离不可用就报错停下，不回退 `probe.exe` |
| Go 执行路径 | 存储 `internal/launcher/storage.go`；内层 PVF `innerpvf.go`；fixture/网关/env `fixture.go`+`gateway.go`+`probeenv.go`；起网关与就绪 `serverrun.go`；客户端宿主+隔离 `clienthost*.go` |

`scripts\storage-route.cmd chain-info` 可随时确认「二进制在不在、会执行什么、带了哪些强制开关」；
`selftest` 会断言启动链里**没有非 Go 候选**。

## 2. 阻塞项：`internal/launcher` 里仍指向 Python 的三处（属另一条并行工作的在制品）

| 位置 | 内容 | 影响 |
| --- | --- | --- |
| `internal/launcher/check.go:125` | 把 `server/work/dfo_probe_tools/channel_probe.py` 列入必需依赖 | 删掉该 `.py` 后 `launch --check` 会失败 |
| `internal/launcher/launch.go:228` | 计划阶段同样按 `channel_probe.py` 存在性取依赖清单 | 同上 |
| `internal/launcher/launch.go:611` | 计划文案仍写「由 helper channel_probe.py 拉起」（网关） | `--dry-run` 输出与实际 Go 执行不符，误导排查 |

**执行路径（`LaunchServer`/`LaunchClient`）已不需要这些 `.py`**：只有「依赖清单 + 计划文案」还提到它们。
业主裁决（2026-10-05）：**等那条并行工作落定后，我再改这三处并执行下面的删除**。

## 3. 待删清单（前置条件满足后执行）

已由 Go 覆盖、可整文件删除（删前确认没有第三方引用：`rg -n '<文件名>' --glob '!**/runtime/**'`）：

- `server/work/dfo-lan/scripts/launch_local.py`（Go：`cmd/dfolauncher launch`）
- `server/work/dfo-lan/scripts/bootstrap_local.py`（Go：启动装配）
- `server/work/dfo-lan/scripts/channel_probe.py`（Go：`serverrun.go` + `gateway.go` + `fixture.go`）
- `server/work/dfo_probe_tools/channel_probe.py`（同上；`probe.exe` 本身就是二进制，不在删除范围）
- `scripts/stop_environment.py`（Go：`dfolauncher stop`，含 SQLite 管理租约回收）
- 两个 Python 测试文件：`scripts/test_environment_storage.py`、`server/work/dfo-lan/scripts/test_postgres_storage.py`
  （删掉被测实现后它们没有意义；若希望保留「双库判定表」的回归，改成 Go 表驱动测试放进
  `internal/database/engine_selection_test.go` 已有同名表）

暂时保留（业主 2026-10-05 裁决：先保证启动链，不动这些）：

- ~~`scripts/gm.py`（已是薄封装，转 `dfo-tool` 成本最低，后续可做）~~ → **2026-10-05 已删除**：
  它依赖的 `scripts/storage_profile.py` 在同轮被删，链路已不可用；命令行的能力由
  `dfo-tool accountlist` / `cmd/admin` / `dfo-tool setlevel` 本身提供，不再需要薄封装。
- ~~`gm-tool/scripts/gmweb.py`（Web GM，Go 化是独立工程）~~ → **2026-10-05 已删除**（连同
  `gm-tool\dashboard\**` 与四个 `.cmd` 入口）：`gm-tool\bin\gmweb.exe` 不在包内，整套本来就跑不起来；
  GM 现在只有启动器内嵌的 Go 实现（启动器仓库 `gm/` → `gmbridge.exe`）。
- `scripts/configure_env.py`（环境配置：其 `.cmd` 入口已被并行工作删除，可能已孤立，需要确认后再决定）

## 4. 验收口径

- 脚本层：`storage-route.ps1 selftest`=19 项通过；`chain-info` 只列 Go 启动器。
- 服务端层（2026-10-05 已实测通过）：`storage-route.ps1 server-postgres` 走新路径起真服务端，
  会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261005_124613_493874_next37`：
  `Server listening on 127.0.0.1:7001 and 127.0.0.2:52821`、`Game server started successfully (PID: 16272)`、
  `ready.json`（channels 50）、`run.json`、以及 fixture 四件套（`channelinfo.bin`/`breakpoints.txt`/
  `fixture.json`/`responses.json`）全部由 Go 写出——**全程没有 Python 参与**。测完只停自己起的网关
  （PG 保持运行、7001 已释放）。
- 客户端层（§0 第 6 条，业主实机）：双击 `scripts\启动游戏-SQLite.cmd` 与
  `scripts\启动游戏-PostgreSQL.cmd`，确认进游戏、选角、进城，且两条路线各看各的存档。
