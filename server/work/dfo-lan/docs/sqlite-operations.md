# SQLite 双引擎操作手册

面向运维与业主：怎么在两种存储引擎之间切换、怎么迁移存档、有哪些边界。
设计与决策依据见 [sqlite-dual-engine-design.md](sqlite-dual-engine-design.md)；
去掉便携工具链的进度见 [runtime-without-tools-plan.md](runtime-without-tools-plan.md)。

## 1. 选择引擎：只改一份配置

引擎由存储配置里的 `driver` 决定（`runtime/storage/local.json`；模板见
`runtime/storage/local.example.json`，**该模板当前就是 SQLite 档、与本机环境一致**）：

```json
{ "driver": "postgres", "postgres_dsn": "postgres://...", "max_connections": 12 }
```

```json
{ "driver": "sqlite", "sqlite_path": "C:/Game/dof/115us/115/server/work/dfo-lan/runtime/storage/dfolan.sqlite3", "sqlite_busy_timeout_ms": 5000, "max_connections": 4 }
```

### 1.1 没有 `driver` 时的唯一规则（2026-10-05 收口）

`driver` 可以省略，但省略时的选择**只有一条规则**，服务端与 Go 启动器按同一次序判定
（2026-10-05 起 Python 那三个镜像已随脚本删除）：

| 次序 | 配置里有什么 | 选中 | 说明 |
| --- | --- | --- | --- |
| 1 | 显式 `driver` | `driver` 的值 | 写了就照写；写成其它值**明确报错** |
| 2 | 没有 `driver`，但有 `postgres_dsn` | **PostgreSQL** | 文档中的默认引擎；配置里点了 DSN 就不该去开别的文件 |
| 3 | 没有 `driver`，也没有 DSN，但有 `sqlite_path` | **SQLite** | SQLite 升级包（20261004）的迁移工具写出的形状：只有 `sqlite_path` |
| 4 | 三个都没有 | PostgreSQL（并报配置不完整） | 不发明一个库出来 |

规则的真源是 `internal/database.EngineForConfig`；镜像在
`internal/launcher.StorageConfig.DriverName`（Go 启动器）与 `gm-tool/scripts/gmweb.py`，
两边各有同一张表的用例（`internal/database/engine_selection_test.go`、
`internal/launcher/launcher_test.go`）。

> **为什么要写死到这一步**：次序 2 与 3 曾经相反（有 `sqlite_path` 就选 SQLite），
> 于是「PostgreSQL 在跑、配置里还留着上一次试 SQLite 的 `sqlite_path`」这种混合档会让
> 启动器起 PostgreSQL、服务端却打开一个空 SQLite 文件——报给业主就是
> **「pgsql 端无法登录」**（账号当然找不到）。
>
> 现在两边都由 `EngineForConfig` 决定；服务端启动日志会**逐次写明它真正打开的库**：
> `storage: engine=postgres target=dfo_lan config=runtime/storage/local.json`
> （SQLite 档的 `target` 是库文件路径）。GM 命令行也会打印同一行结论。
>
> 另外：`local.json` 允许带 **UTF-8 BOM**（记事本 / `Set-Content -Encoding UTF8` 的默认行为）。
> 启动器一直容忍它，服务端此前不容忍——直接死在 `invalid character 'ï'`，表现为「无法登录」。
> `database.LoadConfig` 现在剥掉 BOM，并把出错文件名写进错误里。

- **`sqlite_path` 必须是绝对路径**（2026-10-04 起服务端明确拒绝相对路径，否则会在进程当前目录下建库）。
- **`driver` 缺省即 `postgres`**，所以既有配置不需要改动。
- 写成其它值会**明确报错**，不会静默按 PostgreSQL 处理——存储配置写错一个词就连接到另一个库，必须响。
- 换了 `driver` 之后，启动器与停止脚本都会跟着变：`sqlite` 档**不拉起也不需要停止 PostgreSQL**
  （`scripts/启动游戏.cmd` 直接开服，`scripts/停止游戏环境.cmd` 打印 `Storage: sqlite profile` 后跳过停机）。
- **PostgreSQL 档**必须写 `postgres_dsn`，并且（走 Python 启动路径时）写 `postgres_bin` / `postgres_data`
  指向便携 PostgreSQL；`scripts/配置环境.cmd`（`scripts/configure_env.py`）会补齐这两个路径，
  并**清掉该档里残留的 SQLite 专有键**（`sqlite_path` 等），避免下一次选择含糊。

### 1.2 两条路线各自的启动入口（2026-10-05）

全链（服务端 / 启动器 / Web GM / GM 命令行）只读**同一份活动档**
`runtime/storage/local.json`，所以「切换路线」＝把该路线的档写成活动档。
`scripts\storage-route.cmd` 负责这件事（逻辑在 `scripts\storage-route.ps1`，只写配置档，不启动任何程序）：

```powershell
scripts\storage-route.cmd show                 # 当前路线、连的是哪个库、另一条路线档在不在
scripts\storage-route.cmd use sqlite           # 切到 SQLite 路线（无需 PostgreSQL）
scripts\storage-route.cmd use postgres         # 切到 PostgreSQL 路线
scripts\storage-route.cmd stop-postgres        # 停掉本仓库 pgdata 上的 PostgreSQL
scripts\storage-route.cmd selftest             # 自检（临时目录里跑，不碰本机真实配置）
```

四个启动入口在切好路线后交给统一入口，因此提权、探针、客户端编排仍只有一套实现：

| 路线 | 游戏全链 | 只起服务端 |
| --- | --- | --- |
| SQLite | `scripts\启动游戏-SQLite.cmd` | `scripts\启动服务端-SQLite.cmd` |
| PostgreSQL | `scripts\启动游戏-PostgreSQL.cmd` | `scripts\启动服务端-PostgreSQL.cmd` |

- 路线档放在 `runtime/storage/local.sqlite.json` 与 `local.postgres.json`（**运行期状态，不入库**）：
  切换时当前活动档会先按它自己的引擎存回对应路线档，再写入目标路线档，所以手工改过的键不会丢。
- **两个游戏入口各自独立可用，而且只走 Go**（业主 2026-10-05 定调：脚本先能独立跑通全链，再谈接进
  启动器；随后明确**彻底移除所有外部环境依赖（含 Python），不再回退，必须 Go 成功**）：
  它们纯 ASCII、自己提权（WFP 需要管理员），不依赖 `启动游戏.cmd` 之类正在被改造的中间入口；
  全链由仓库内的 Go 启动器执行 `bin\dfolauncher.exe launch [--server-only]`——
  存储（`storage.go`）→ 内层 PVF（`innerpvf.go`）→ fixture/网关/env（`fixture.go`/`gateway.go`/`probeenv.go`）
  → 起网关与就绪轮询（`serverrun.go`）→ 客户端宿主与 WFP 隔离（`clienthost*.go`）。
  启动时强制 `DFO_REQUIRE_GO_ISOLATION=1`：Go 隔离不可用就报错停下，**不静默回退 `probe.exe`**
  （`DFO_FORCE_PROBE_EXE=1` 只留给排障/验收）。`scripts\storage-route.cmd chain-info` 报告二进制是否在位
  与强制开关；`selftest` 断言启动链里没有非 Go 候选。
- 路线档缺失时按**本机**路径生成：SQLite 用绝对 `sqlite_path`；PostgreSQL 优先沿用本机既有的
  `local.json.pg-backup`（保留其中真实 DSN），并把 `postgres_bin`/`postgres_data` 修成本机实际存在
  的路径（便携 PG 在仓库外 `..\tools\pg`），缺 DSN 时才用项目默认值并明确提示核对。
- **两条路线的存档互相独立**：SQLite 是单文件、PostgreSQL 是 `pgdata` 里的库。换路线不会带着角色走；
  搬运存档用 `dfo-tool sqliteconvert`（只支持 PostgreSQL → SQLite 单向）。
- PostgreSQL 路线的入口会调 `bin\dfolauncher.exe start-storage` 起库（`driver=sqlite` 时它是空操作，
  见 `internal/launcher.StartStoragePlan`）；停止仍由 `scripts\停止游戏环境.cmd` 按活动档的 driver 决定，
  所以「已切回 SQLite 但 PostgreSQL 还在跑」时用 `storage-route.cmd stop-postgres` 收尾。
- **四个入口的 `.cmd` 只有 8 行纯 ASCII**：路线切换、中文提示与调用中文名统一入口
  （`启动游戏.cmd` / `启动服务端.cmd`）都在 BOM 的 `storage-route.ps1` 里。原因是实测的 cmd 解析坑：
  UTF-8 中文进执行行（尤其 `-File "scripts\<中文>.ps1"`）或裸 LF 行尾，会被 cmd 按控制台代码页拆成
  乱码命令（`'hell' is not recognized ...`），整条启动链误判失败。规则与证据见根 `AGENTS.md` §0.4.2。
- **PG 路线的起库预检有上限、有进度，而且不再会「卡住」**（2026-10-05 实机：
  PG 日志已 `database system is ready to accept connections`，控制台却停在「拉起 PostgreSQL」，
  最后只能 Ctrl+C）。根因是 Windows 上 `pg_ctl start` 会留一个 `cmd.exe` 包装器当 postgres 的父进程，
  它**继承调用者的 stdout/stderr**：任何「把子进程输出接成管道再等 EOF」或「共享控制台再等它」的
  写法都会一直等下去。现在：
  * `storage-route.ps1` 用 **WMI 创建进程**（`Win32_Process.Create` 不继承调用者句柄）+ 命令行内部
    重定向到 `runtime\storage\pg-ctl.out.log` / `postgres.log`，**只轮询端口**判断就绪
    （每 5 秒报一次进度，上限 60 秒）；数据目录里 pid 已死的 `postmaster.pid` 会先被清掉并打印。
  * Go 侧 `internal/launcher.StartStorage` 不再用 `CombinedOutput()`（管道），改成把子进程输出写进
    日志文件的 `Run()` 并同样以端口为判据。
  * 独立入口：`scripts\storage-route.cmd preflight-postgres` 只做起库预检，不开游戏。
    实测：PG 已停 → 预检 **3.1 秒返回、PG 1.9 秒就绪**（此前同一操作永不返回）。

## 2. 首次在 SQLite 上启动

服务端在启动时打开（必要时创建）`sqlite_path`，并**一次性应用全部 schema 分节**；
台账保证重复启动幂等，所以第二次启动不会重放 DDL。
`Migrate*` 那些按域推进的入口在 SQLite 上是**诚实的空操作**（PostgreSQL 保留它们，是因为它的
schema 是增量长起来的）。

SQLite 文件放在 `runtime/` 下，与其它运行产物一样**不入库**。

## 3. 把现有 PostgreSQL 存档迁到 SQLite

```powershell
cd server\work\dfo-lan
$env:CGO_ENABLED='0'
..\..\..\tools\go\bin\go.exe run ./cmd/dfo-tool sqliteconvert `
  --config runtime\storage\local.json `   # 指向 PostgreSQL 档的配置（源永远读它）
  --out    runtime\storage\dfolan.sqlite3 # 必须是尚不存在的新文件
```

- **单向**：只做 PostgreSQL → SQLite。SQLite 是单机目标，PostgreSQL 在业主接受迁移前仍是权威，
  因此没有反向路径需要维护。
- 转换器**逐表校验行数**、**逐行校验 JSON 字节**（用哈希在行序上比较），全部拷完后跑
  `PRAGMA foreign_key_check` 校验引用完整性。任何一项不符就报错，不会留下一个"看起来成功"的库。
- 目标文件必须**不存在**：在已有库上转换会与其中的行冲突，正确做法是拒绝而不是合并两份存档。
- 输出里若出现 `WARNING: ... absent from the PostgreSQL schema`，说明**源端 schema 比目标端旧**
  （真实部署启动时会跑完全部迁移，正常不出现）。此时不能声称转换完整。
- `storage_migrations`（迁移台账）**不参与拷贝**：它记录的是"某个引擎已应用了哪些分节"，
  目标库在建表时已写入自己的台账。

## 4. 边界与已知取舍

- **管理守卫（GM 互斥）**：PostgreSQL 用会话级 advisory lock，连接断开（含进程崩溃）由数据库自动释放；
  SQLite 没有 advisory lock，改用数据库旁的**租约文件**，释放即删除。
  **已知取舍**：进程崩溃会留下租约文件，此时 GM 写入会被拒绝——错误信息里**点名了该文件与持有者 pid**。
  收尾有两条路（2026-10-05 补）：
  1. **停止链自动收**：`dfolauncher stop` / `scripts\停止游戏环境.cmd` 在 SQLite 档下**最后**规划一步
     `lease-clear`——先把服务端进程强杀（这一步本来就是强杀），再读租约里记录的 pid，
     **只有平台明确回答「该 pid 不存在」时才删除**；pid 还活着（多半是 Web GM 在写）或问不出来
     （权限不足等）一律保留并说明原因（实现与判据见 `internal/launcher/adminlease.go`）。
     于是「停 → 立刻启」不再撞 60 秒 TTL。
  2. **手动收**：崩溃后没有走过停止链时，`scripts\storage-route.cmd clear-guard` 用同一套判据清一次；
     租约里读不出 pid（空文件/内容异常）时两条路都**不动**它，等 TTL 或由业主确认后手工删除。
  之所以不做「过期就自动抢锁」：守卫面对并发索取者必须假设最坏，误判会让两个管理员同时写；
  停止链只在「业主明确要求停掉这套环境、且刚强杀过服务端」的语境里动手，并且判据是平台的明确否定。
  崩溃安全的方案（带心跳的租约行）记录在设计文档 §3.3。
- **`NextMailID`**：PostgreSQL 从序列取号，SQLite 改为**推进 `sqlite_sequence` 高水位**，
  因此"附件 id 先分配、与消息 id 共用号段"的不变式在两个引擎上都成立。
- **`date` 列**：表示形式由**查询**决定而非声明——`character_fatigue*`、`account_tower_progress`
  等按 ISO 文本读写，其余按整数微秒。转换器按同一张表迁移，不会两套混用。
- **PostgreSQL 专属、SQLite 上明确报错的功能**：测试 fixture 的临时 schema 隔离、诊断查询
  （`DiagnosticQuery`/`DiagnosticExec`）。它们本就只服务 PostgreSQL 开发流程，拿不到连接池时会
  返回明确错误，而不是悄悄什么都不做。

## 5. 验证清单

```powershell
cd server\work\dfo-lan
$env:CGO_ENABLED='0'; $go='..\..\..\tools\go\bin\go.exe'
& $go test -count=1 ./internal/database/      # schema 对齐、ASCII 门禁、双引擎契约、转换器
& $go test -count=1 ./internal/archtest/      # 依赖边界
& $go run ./cmd/dfolauncher check --root ..\..\..   # 只读依赖检查（不启动任何东西）
```

需要真实 PostgreSQL 的用例（双引擎契约的 `postgres` 子测试、转换器端到端）由
`DFO_TEST_POSTGRES_DSN` 选择**隔离测试库**后运行；未设置时它们自动跳过，不会碰玩家库。

### 5.1 GM 命令行在两种档上都能用（2026-10-05）

```powershell
cd <仓库根>
scripts\GM.cmd list                                   # 活动档：账号 + 点券 + 角色概览
scripts\GM.cmd set --name <角色> --level 50 --preview  # 改等级先预览
```

- 读取走 `dfo-tool accountlist`（引擎中立的只读概览），**不再用 Python 直连 SQLite**，
  所以同一组 GM 命令在 PostgreSQL 档上同样可用；写操作走 `cmd/admin` 与
  `dfo-tool setlevel`（本来就按 `Open` 选引擎，带审计与幂等键）。
- 输出第一行就是**它实际连的库**（`存储: postgres <库名>` / `存储: sqlite <文件>`）；
  PostgreSQL 档在库没监听时会明确提示先启动服务端，而不是抛一个连接错误。
- Web GM（`gm-tool`）走同一条规则：SQLite 档不再尝试拉起 PostgreSQL，只回报库文件位置。
