# SQLite 双引擎操作手册

面向运维与业主：怎么在两种存储引擎之间切换、怎么迁移存档、有哪些边界。
设计与决策依据见 [sqlite-dual-engine-design.md](sqlite-dual-engine-design.md)；
去掉便携工具链的进度见 [runtime-without-tools-plan.md](runtime-without-tools-plan.md)。

## 1. 选择引擎：只改一份配置

引擎由存储配置里的 `driver` 决定（`runtime/storage/local.json`）：

```json
{ "driver": "postgres", "postgres_dsn": "postgres://...", "max_connections": 12 }
```

```json
{ "driver": "sqlite", "sqlite_path": "runtime/storage/dfolan.sqlite3", "sqlite_busy_timeout_ms": 5000, "max_connections": 4 }
```

- **`driver` 缺省即 `postgres`**，所以既有配置不需要改动。
- 写成其它值会**明确报错**，不会静默按 PostgreSQL 处理——存储配置写错一个词就连接到另一个库，必须响。
- 换了 `driver` 之后，启动器与停止脚本都会跟着变：`sqlite` 档**不拉起也不需要停止 PostgreSQL**
  （`启动游戏.cmd` 直接开服，`停止游戏环境.cmd` 打印 `Storage: sqlite profile` 后跳过停机）。

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
  **已知取舍**：进程崩溃会留下租约文件，此时 GM 写入会被拒绝——错误信息里**点名了该文件与持有者 pid**，
  确认没有 GM 在写就删除它。崩溃安全的方案（带心跳的租约行）记录在设计文档 §3.3。
  之所以不做"自动清理死进程租约"：标准库无法可移植地判断进程存活，误判会让两个管理员同时写，
  那正是这个锁要防的事。
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
