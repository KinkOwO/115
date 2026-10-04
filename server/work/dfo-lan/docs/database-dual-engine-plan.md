# 双引擎（PostgreSQL + SQLite）落地计划（交接给新会话）

> 状态：**计划阶段，未改任何代码**。本轮（2026-10-04）只做取证与交接。
> 业主口径（2026-10-04）：**保持双引擎**。
>
> ⚠️ **本条推翻了既有口径，必须显式登记**：本仓此前两次明确"不做双引擎"——
> `docs/architecture.md` ADR-002（"SQLite 适合工具与小型单机，这里不作为多人服主库"）与
> `docs/database-sqlc-migration.md`（"不提前引入双引擎接口，不声称 PostgreSQL SQL 可以直接由 SQLite 执行"）。
> 业主本轮改口要求**双引擎** ⇒ 新会话实施时须同时修订 ADR-002 与 sqlc 文档的这两句，
> 不得让代码与文档互相矛盾（根 §0.2/§0.5 的纪律）。

## 0. 现状事实（2026-10-04 取证，全部只读）

| 事实 | 证据 |
| --- | --- |
| **SQLite 零实现** | `go.mod` / `go.sum` 无任何 sqlite 依赖；Go 源码搜 `sqlite` **0 命中**（全仓命中只在文档与 pgAdmin 自带 Python 库） |
| 配置无驱动/方言选择项 | `internal/database/store.go:32` `Config{PostgresDSN, MaxConnections, PostgresSchema}` |
| **没有可替换的接口缝** | `type Store struct` 是具体类型，`db`/`queries` 私有；`*database.Store` 在包外被 **32 个文件**直接持有 |
| sqlc 只有 postgres 一种 engine | `sqlc.yaml`：`engine: "postgresql"`、`schema: internal/database/sql/postgres/migrations`、`sql_package: pgx/v5`、`emit_interface: false` |
| SQL 全部路径即方言 | `internal/database/sql/postgres/{migrations,queries,repairs,fixtures}`，共 **17 个 .sql、93 KB**；`//go:embed sql/postgres/migrations/*.sql` |
| PG 专有语法用量 | `FOR UPDATE` **25**、`ON CONFLICT` **57**、`RETURNING` **16**、`::` 转型 **150**、`jsonb` **21**、`GENERATED` **21**、`pg_` **3**、`advisory` **3**；文档另记有 `FOR NO KEY UPDATE` 与 `ANY/unnest`、`DO` 旧表探测 |
| pgx 依赖面 | `internal/database/*.go` 里 pgx/pgxpool/pgconn/pgtype 引用 **68 处**（含 `pgx.Tx` 作为事务回调参数类型、`pgx.ErrNoRows` 判定） |
| 运维耦合 | 账号金库等路径有**固定锁顺序**（先账号 `FOR NO KEY UPDATE`、再角色）；启动编排（Python）按 PostgreSQL 拉起/探活；测试夹具用 `CREATE SCHEMA` + `search_path` 隔离；`runtime/storage/local.json` 携带 DSN |
| 直持 `*database.Store` 的文件（32） | `cmd/gmtool/main.go`；`cmd/wireprobe/` 20 个文件（bootstrap、account_materials_flow、booster_flow、cinematic_flow、moon_solo_resources、odyssey_revive、odyssey_rewards、odyssey_weapon_box、omen_state、reward_flow、roster_background_flow、skin_family_flow、skin_flow、skin_selection_flow、skin_storage_flow、synopsis_flow、world_flow + 2 个 _test）；`internal/admin/grant.go`；`internal/character/store_test.go`；`internal/workflow/` 10 个文件（bag_operations、inventory_role、item_equipment、loot_rewards、quest_finish、shop、unseal、vault、wear + inventory_role） |

**可移植性判断（好消息）**：存档语义本身可移植——角色状态/背包/回执都是 **JSON 文本 + 显式字段**，
既有文档已把"保持原 JSON 字段（含未知字段）、回执键与 model、软删除、角色编号、邮件共享序列、
二进制设置、NULL/空集合语义"列为目标。真正的墙是**方言与并发原语**，不是数据模型。

**成本占比（我的评估）**：③ 锁/事务语义 > ① SQL 重写 > ④ 数据转换与编号 > ⑤ 运维与测试。

## 1. 目标与非目标

**目标**：同一份领域代码与领域持久化契约，可在 **PostgreSQL（多人服）** 与 **SQLite（单机/工具）** 上运行；
默认仍是 PostgreSQL；切换只通过配置与构建标签，不 fork 领域代码。

**非目标（本轮不做）**：
- 不做自动 PG→SQLite 双向同步（只做**单向数据转换工具**）。
- 不引入 ORM/Repository/Adapter 分层（既有文档明确"不新增 Repository、Adapter 或按表划分的包"，
  本计划沿用：只抽**已有领域 Store 契约**，不新造架构层）。
- 不在 SQLite 上实现多人并发服主库（ADR-002 的定性不变：单写者）。

## 2. 实施阶段（每阶段可独立提交、可独立回滚）

### S1 抽接口缝（唯一真正的架构改造）

1. **盘点并定稿领域 Store 契约**：以 32 个调用点为清单，抽出**按领域**的接口
   （建议：`CharacterStore`、`AccountStore`、`CashStore`、`MailStore`、`QuestStore`、`VaultStore`、
   `SettingsStore`、`ProgressionStore` …），方法集合**只包含现有调用**，不预留未来方法。
2. **把 `*database.Store` 的持有者改为接口**（32 个文件）：`internal/workflow/*` 与
   `cmd/wireprobe/*` 的字段类型从 `*database.Store` 换成对应接口。
3. **禁止包外嗅探**：延续既有依赖守卫（`internal/archtest`）——
   业务/网关不得 import `sqlcgen`、领域不得 import pgx、不得恢复 generic db 包、
   `Tx` 不得暴露通用 SQL/提交能力；本阶段**新增**两条守卫：
   **① `*database.Store` 不得出现在 `internal/workflow` 与 `cmd/*` 的字段/参数里；
   ② 领域接口不得引用 pgx/pgtype 类型。**
4. 门禁：全量 `go test ./...` + `go vet ./...`；接口抽取前后**行为零变化**（同一批集成测试必须同样通过）。

### S2 pgx 移出公共 API

- **事务能力收窄**：现在事务回调参数是 `pgx.Tx`（`internal/database/*.go` 多处在用）⇒ 改为
  **具体能力对象**（`*database.Tx` 型，只暴露该事务需要的领域操作），延续既有方向
  （文档已记"回调改为具体 `*database.Tx`；查询生成类型和驱动字段私有"）。
- 收尾后 `internal/database` 内部分成两层：**方言无关核心**（事务编排、回执、存档语义）
  与 **postgres 适配**（连接池、pgtype、方言 SQL）。
- 门禁：`go vet` + 依赖守卫必须证明"领域侧零 pgx"。

### S3 sqlc 双引擎配置与 SQL 分叉

- 目录：新增 `internal/database/sql/sqlite/{migrations,queries}`；**既有 PG 文件不改写**（文档已定：
  未来新增升级用新的增量 SQL，不动已应用的初始分段）。
- `sqlc.yaml`：`postgresql` 保持现状；新增一段 `engine: "sqlite"`（`sql_package: "database/sql"`），
  生成到 `internal/database/sqlcgen/sqlite`（与 PG 的 `sqlcgen` 并列，共用同一批 Go 模型别名）。
- **必须重写的差异（逐项核对，不能机械翻译）**：
  | PG 用法 | SQLite 侧处理 |
  | --- | --- |
  | `jsonb`（21） | 存 **TEXT**；JSON 运算**回 Go**（文档已定"JSON 逻辑回 Go"），不在 SQL 里做 `->`/`@>` |
  | `::` 转型（150） | 逐条改写；注意 SQLite 的动态类型与整数范围 |
  | `ON CONFLICT`（57） | SQLite 支持，但需核对冲突目标/`DO UPDATE` 语法与 `excluded` 语义 |
  | `RETURNING`（16） | SQLite ≥3.35 支持；需固定最低版本并在启动时校验 |
  | `FOR UPDATE` / `FOR NO KEY UPDATE`（25+） | **SQLite 无行锁** ⇒ 用 `BEGIN IMMEDIATE` 序列化写事务 + `busy_timeout` 替代；语义等价性必须在契约测试里证明 |
  | `ANY/unnest`、`DO` 旧表探测 | 改写为多次查询或临时表；旧表探测改为读 `sqlite_master` |
  | `GENERATED ... AS IDENTITY`（21） | 改 `INTEGER PRIMARY KEY AUTOINCREMENT` 或显式序列表；**保留邮件共享编号序列语义** |
  | `pg_*` / advisory 锁（3+3） | 改用应用级锁表或 `BEGIN IMMEDIATE`；管理会话锁改为文件锁 |
  | 时间函数 / `timestamptz` | 统一存 UTC 文本或整数；**保持既有 NULL 与时间语义** |
  | `search_path` 隔离 | 改为**数据库文件隔离**（每个测试一个临时文件） |

### S4 SQLite 引擎实现

- `Open` 按配置选择引擎（新增 `Driver`/`Dialect` 字段，默认 `postgres`；DSN/文件路径二选一）。
- 事务实现：`BEGIN IMMEDIATE` + 写序列化 + busy 重试；**保持账号限购、角色存档、回执原子性**，
  文档已明确"不能机械删除锁"。
- 迁移：另建 SQLite 升级脚本 + **PG→SQLite 数据转换工具**（保留存档、回执键、外键与邮件共享编号；
  既有 PG 迁移文件不改写）。
- 运维与测试：替换 PostgreSQL 启动/可用性探测/管理会话锁；处理单写者、busy 重试与连接设置；
  测试夹具从"临时 schema"改"临时数据库文件"。

### S5 分叉控制（决定双引擎能不能长期维护）

- **同一批契约测试跑两遍**：把现有 PG 集成测试（`DFO_TEST_POSTGRES_DSN`）提取为**引擎无关的契约套件**，
  用 `DFO_TEST_SQLITE_PATH`（或 `DFO_TEST_ENGINE=sqlite`）驱动同一批场景。
- 必测清单（沿用文档已列的验证项）：旧存档升级、未知 JSON/大整数/NULL、并发容量/账号限购、
  失败回滚、回执重放、角色编号与 64 位排序序号、邮件共享序列、二进制设置。
- **CI/门禁**：两条引擎都必须过；任一引擎独有行为要写进文档并注明原因。

## 3. 验收与收口纪律

1. 每阶段一个 commit；`CHANGELOG` 记"改了什么 + 哪个引擎验证过 + 没验证什么"。
2. **存档兼容最高优先级**（根 §0.4）：任何 schema 变更必须给平滑迁移；PG 既有存档无损。
3. 实机由业主操作；本会话/新会话不得无人值守起客户端或玩家库（AGENTS §0.6）。
4. 玩家库端口 **25438** 与测试库端口 **25439** 隔离（既有约定），不得对玩家库执行清理 SQL。
5. 默认引擎保持 PostgreSQL；SQLite 上线不得改变默认 profile 行为。

## 4. 需要业主拍板的 3 个问题

1. **SQLite 的定位**：单机/工具用（沿用 ADR-002 的定性，双引擎只是"能跑"）？还是将来可能做多人服主库？
   —— 若后者，S4 的写序列化与 WAL 策略要重新评估。
2. **SQLite 的最低保版本**（决定能否用 `RETURNING`/upsert）：建议 **≥ 3.35**。
3. **驱动选择**：`modernc.org/sqlite`（纯 Go、无 CGO，便于便携分发）还是 `mattn/go-sqlite3`（CGO）？
   —— 本仓是便携 Python+PG 编排的分发方式，倾向纯 Go。

## 5. 建议的新会话起点（最短路径）

1. 先读本文 §0 的 32 文件清单，**只做 S1 的第 1 步**（契约定稿，不改调用点），产出一份
   `领域 Store 契约清单.md`（接口名 → 方法 → 现调用点）。
2. 再用 `internal/archtest` 加两条守卫**先让它们失败**（红），然后逐个文件改持有类型（绿）。
3. S1 全绿后再动 S2/S3；**不要**在 S1 未收口时开始写 SQLite SQL（会同时改两层、无法定位回归）。
