# SQLite 存储操作手册

面向运维与业主：库文件在哪、配置怎么写、怎么排障，以及**历史 PostgreSQL 存档（pgdata）怎么救**。

> **2026-10-05：PostgreSQL 支持已整体移除**（业主口径，见根 `AGENTS.md` §0.6）。
> 引擎/连接池/DSN 判定、`pgx` 依赖、`sql/postgres/**`、启动器的 `initdb`/`pg_ctl`/`createdb` 与
> 25438 端口探测、PG 启动脚本与路线档、`tools/tools-pg-bin.zip` 全部删除。本手册因此只讲
> **SQLite 唯一引擎**；`runtime/storage/pgdata/` 变成**只读留档**，读取方式见 §3。
> 双引擎时期的设计与决策依据仍可查 [sqlite-dual-engine-design.md](sqlite-dual-engine-design.md)
> （历史文档，描述的是当时的双引擎形态）。

## 1. 库在哪、配置写什么

活动配置是 `server/work/dfo-lan/runtime/storage/local.json`（模板
`runtime/storage/local.example.json`，**该模板就是 SQLite 档、与本机环境一致**）：

```json
{ "driver": "sqlite", "sqlite_path": "C:/Game/dof/115us/115/server/work/dfo-lan/runtime/storage/dfolan.sqlite3", "sqlite_busy_timeout_ms": 5000, "max_connections": 4 }
```

### 1.1 引擎判定的唯一规则（2026-10-05 收口）

规则的真源是 `internal/database.EngineForConfig`，镜像在
`internal/launcher.StorageConfig.DriverName`，两侧各有同一张表的用例
（`internal/database/engine_selection_test.go`、`internal/launcher/launcher_test.go`）：

| 次序 | 配置里有什么 | 结果 | 说明 |
| --- | --- | --- | --- |
| 1 | 显式 `driver=sqlite` | **SQLite** | 写了就照写 |
| 2 | 没有 `driver`（或为空） | **SQLite（兜底）** | 2026-10-05 业主口径；与启动器兜底一致 |
| 3 | 显式 `driver=postgres` | **明确报错** | `PostgreSQL support was removed (2026-10-05, see root AGENTS.md §0.6)` |
| 4 | 其它 `driver` 值 | **明确报错** | 存储配置写错一个词就连到另一个库，必须响 |

- **档里遗留的 `postgres_dsn` / `postgres_bin` / `postgres_data` 键不会报错**（`encoding/json` 忽略
  未知/已删字段），但**不再有任何代码读它们**。所以旧档可以直接用，只要 `driver` 写的是 sqlite。
- **`driver=postgres` 不会被静默降级成 SQLite**。这是刻意的：把服务端指向另一个库，玩家看到的是
  「存档像丢了 / 登录失败」——比直接报错糟糕得多。
- **`sqlite_path` 必须是绝对路径**：服务端明确拒绝相对路径，否则会在进程当前目录下建库。
- `local.json` 允许带 **UTF-8 BOM**（记事本 / `Set-Content -Encoding UTF8` 的默认行为）：
  `database.LoadConfig` 会剥掉它，并把出错文件名写进错误里。此前不容忍 BOM 时表现为
  `invalid character 'ï'` → 「无法登录」。
- 服务端启动日志会**逐次写明它真正打开的库**：
  `storage: engine=sqlite target=<库文件绝对路径> config=runtime/storage/local.json`。GM 命令行也打印同一行结论。

### 1.2 启动入口

启动链只读**同一份活动档**，因此没有「切路线」这回事了：

| 用途 | 入口 |
| --- | --- |
| 游戏全链（玩家与完整测试，管理员权限） | `scripts\启动游戏-SQLite.cmd` |
| 只起服务端 | `scripts\启动服务端-SQLite.cmd` |
| 停止客户端与服务端（并清理管理租约） | `scripts\停止游戏环境.cmd` |

```powershell
scripts\storage-route.cmd show          # 当前档、连的是哪个库（只读）
scripts\storage-route.cmd clear-guard   # 清一次崩溃残留的管理租约（判据见 §4）
scripts\storage-route.cmd selftest      # 自检（临时目录里跑，不碰本机真实配置）
```

- 入口交给仓库内的 Go 启动器执行 `bin\dfolauncher.exe launch [--server-only]`；
- `local.sqlite.json` 仍可作为同一档位的备份名，但**已没有 `local.postgres.json` 这条路线的消费者**；
- 启动时强制 `DFO_REQUIRE_GO_ISOLATION=1`：Go 隔离不可用就报错停下，**不静默回退 `probe.exe`**
  （`DFO_FORCE_PROBE_EXE=1` 只留给排障/验收）。

## 2. 首次在 SQLite 上启动

服务端在启动时打开（必要时创建）`sqlite_path`，并**一次性应用全部 schema 分节**
（`sql/sqlite/migrations/0001_initial.sql`，按 `sqliteMigrationSections` 逐节执行）；
台账 `storage_migrations` 保证重复启动幂等，所以第二次启动不会重放 DDL。
`Migrate*` 那些按域推进的入口在这里是**诚实的空操作**（它们保留下来只为让大量调用点与每域意图继续可读）。

库文件放在 `runtime/` 下，与其它运行产物一样**不入库**；连带的 `-wal`/`-shm` 与管理租约同样不入库。

## 3. 历史 PostgreSQL 存档（pgdata）的可救路径

`runtime/storage/pgdata/`（本机 66.1 MB）与 `pgdata.stale-*` 是 PG 时代留下的**历史存档**，
现在**没有任何代码能直接读它们**：引擎、`pgx` 依赖与 `dfo-tool sqliteconvert` 都已从当前源码移除。
**绝不删除**——删掉这条路就彻底断了。要读它们，只有一条路：

```powershell
# ① 在旧提交上开一个 worktree（不改动当前工作区）
cd <仓库根>
git worktree add ..\pgdata-rescue <移除 PostgreSQL 的那个提交之前的 SHA>
cd ..\pgdata-rescue\server\work\dfo-lan

# ② 在那边构建还带转换器的 dfo-tool（注意：转换器需要 pgx，所以只能在这个旧提交里构建）
$env:CGO_ENABLED='0'
..\..\..\tools\go\bin\go.exe build -o dfo-tool-rescue.exe ./cmd/dfo-tool

# ③ 单向搬到一个【独立的】新库，不要覆盖正在用的 dfolan.sqlite3
.\dfo-tool-rescue.exe sqliteconvert `
  --config <仓库根>\server\work\dfo-lan\runtime\storage\local.json `
  --out    <某个新目录>\pgdata-rescued.sqlite3

# ④ 用当前版本打开那个文件（把它复制到 runtime\storage\ 下并改 local.json 的 sqlite_path，或直接换档）
```

- **第 ② 步还要自己准备 PostgreSQL 16.4 便携版**：本机已经没有 PG 二进制
  （`..\tools\pg` 与仓库内 `tools\pg` 都不存在），而 `sqliteconvert` 要连上 pgdata 里的库才读得到数据
  （`pg_ctl start -D runtime\storage\pgdata`）。
- 转换器当年的行为（供参考，代码在旧提交里）：**单向 PostgreSQL → SQLite**，逐表校验行数、
  逐行校验 JSON 字节（哈希在行序上比较），全部拷完后跑 `PRAGMA foreign_key_check`；
  目标文件必须**不存在**；`storage_migrations` 台账不参与拷贝。
- 万一 `pgdata` 已经损坏或缺失：git 里没有它的副本（它从未入库），只能从别处的备份恢复 ——
  这也是「绝不删除」这条规则的原因。

## 4. 边界与已知取舍

- **管理守卫（GM 互斥）**：SQLite 没有 advisory lock，改用数据库旁的**租约文件**（`<db>.admin-guard`），
  释放即删除。**已知取舍**：进程崩溃会留下租约文件，此时 GM 写入会被拒绝——错误信息里**点名了该文件与持有者 pid**。
  收尾有两条路：
  1. **停止链自动收**：`dfolauncher stop` / `scripts\停止游戏环境.cmd` 在 SQLite 档下**最后**规划一步
     `lease-clear`——先把服务端进程强杀（这一步本来就是强杀），再读租约里记录的 pid，
     **只有平台明确回答「该 pid 不存在」时才删除**；pid 还活着（多半是 GM 在写）或问不出来（权限不足等）
     一律保留并说明原因（实现与判据见 `internal/launcher/adminlease.go`）。于是「停 → 立刻启」不再撞 60 秒 TTL。
  2. **手动收**：崩溃后没有走过停止链时，`scripts\storage-route.cmd clear-guard` 用同一套判据清一次；
     租约里读不出 pid（空文件/内容异常）时两条路都**不动**它，等 TTL 或由业主确认后手工删除。
  之所以不做「过期就自动抢锁」：守卫面对并发索取者必须假设最坏，误判会让两个管理员同时写。
  崩溃安全的方案（带心跳的租约行）记录在设计文档 §3.3。
- **`NextMailID`**：改为**推进 `sqlite_sequence` 高水位**，因此「附件 id 先分配、与消息 id 共用号段」
  的不变式继续成立。
- **`date` 列**：表示形式由**查询**决定而非声明——`character_fatigue*`、`account_tower_progress`
  等按 ISO 文本读写，其余按整数微秒，不会两套混用。
- **诊断入口现在是 SQLite 实现**：`DiagnosticQuery` / `DiagnosticExec`（`dfo-tool dbq` 与测试准备用）
  直接走引擎自己的 `*sql.DB`，只读事务保证一条诊断语句改不了存档；不再需要「PostgreSQL 专属」的例外说明。
- **测试夹具的隔离单位是库文件**：`OpenTestFixture` 在临时目录里建一个独立库，关闭时连目录一起删；
  不再建 PostgreSQL 临时 schema。因此原本需要 `DFO_TEST_POSTGRES_DSN` 才能跑的那批集成测试
  **现在默认就在跑**（只有确实需要真实 PVF 归档的少数用例仍要求 `DFO_PVF_CORE_TEST_ARCHIVE`）。

## 5. 验证清单

```powershell
cd server\work\dfo-lan
$env:CGO_ENABLED='0'; $go='..\..\..\tools\go\bin\go.exe'
& $go build ./...
& $go vet ./...
& $go test -count=1 ./...                     # 41 个包，全绿才算交付
& $go run ./cmd/dfolauncher check --root ..\..\..   # 只读依赖检查（不启动任何东西）
```

- `internal/archtest` 的分层守卫现在只认 `modernc.org/sqlite` 与 `database/sql`：
  数据库驱动不得出现在 `internal/database` 之外。
- `internal/database` 的 SQLite 用例覆盖：schema 分节与声明一致、建表数一致、SQL 文件 ASCII 门禁、
  引擎契约、备份/还原往返。
- 需要真实 PVF 归档的用例由 `DFO_PVF_CORE_TEST_ARCHIVE` 选择归档后运行；未设置时自动跳过，
  不会碰玩家库。

### 5.1 GM 命令行

```powershell
cd <仓库根>
scripts\GM.cmd list                                   # 活动档：账号 + 点券 + 角色概览
scripts\GM.cmd set --name <角色> --level 50 --preview  # 改等级先预览
```

- 读取走 `dfo-tool accountlist`（引擎中立的只读概览），**不再用 Python 直连 SQLite**；
  写操作走 `cmd/admin` 与 `dfo-tool setlevel`（按 `Open` 选引擎，带审计与幂等键）。
- 输出第一行就是**它实际连的库**（`存储: sqlite <文件>`）。
- 启动器内嵌的 GM 走同一条规则：只回报库文件位置，不再尝试拉起任何数据库服务。
