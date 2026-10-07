# SQLite 双引擎落地方案（S0–S5）

状态：**方案 + S0 已部分执行**（2026-10-04）。

- 方案部分以只读取证为准，未改 SQL / schema / 存档。
- **S0 已执行**：sqlc v1.31.1 安装并验证（§4.2 B1）；`internal/archtest` 守卫缺口修补 G2/G3（§1.1、§7.2）；
  `scripts/Generate-SQL.ps1` 多引擎改造（§4.2 B2）；sqlc 双引擎 spike → **判定 A 路径**（§4.5）。
- 仍在 S0：契约/ADR 措辞收口（§8 S0 ⑤）、以及业主对 §11 小项的确认。
日期：2026-10-04
上位文档：`mod-sqlite-redis-minimal-plan.md`（2026-10-05 已删除）（四目标总交接，本文只覆盖其目标 ②）
规则真源：[architecture-contract.md](architecture-contract.md)、[architecture.md](architecture.md) ADR-002、根 `AGENTS.md`

---

## 0. 决策锁定（本轮业主拍板）

| 项 | 结论 | 直接后果 |
| --- | --- | --- |
| SQLite 定位 | **双引擎**：PostgreSQL 保持默认引擎与多人服主库；SQLite 作为单机/工具引擎（单写者） | 不删 pgx / PG 路径；不做双向同步；写并发用 `BEGIN IMMEDIATE` 串行化，而非行锁 |
| 驱动 | **`modernc.org/sqlite`**（纯 Go、无 CGO，便于便携分发） | `CGO_ENABLED=0` 现状不变；可交叉编译；二进制约 +10 MB 量级，换掉 919.8 MB 便携 PG |
| 最低版本 | **SQLite ≥ 3.39**（原计划 ≥3.35） | 必须支持 `IS DISTINCT FROM`（见 §3）；`modernc.org/sqlite` v1.60.1 内置 **3.53.4**，满足 |
| 遗留问题 ②③④ | 驱动与定位已定；**不彻底放弃 PG** | S3 采用"PG 用 sqlc + SQLite 分叉"，见 §4 |

被本轮推翻的既有表述（已就地登记，需随 S1 一并修订）：
`docs/architecture.md:26-27` 引用的 `database-dual-engine-plan.md` **已在上轮删除**（悬空链接，本轮修正）；
`docs/database-sqlc-migration.md（2026-10-05 已删除）:74`「不提前引入双引擎接口」与 `:260`「当前不提供双引擎支持」。

**本轮验证边界**：仅文档与取证。未运行服务、未访问玩家库（25438）、未启动客户端、未触碰 `runtime/storage/**`。

---

## 1. 既有约束（不可协商，均已被自动化守卫固化）

| # | 约束 | 证据 |
| --- | --- | --- |
| C1 | L3 领域**禁止** import `internal/database`；领域需要存储能力时**自己声明接口**，由 `internal/database` 实现并在 bootstrap 注入 | `internal/archtest/contract_test.go:38,152`；`internal/character/store.go:11-46` 是现成范式 |
| C2 | `pgx` / `database/sql` / `sqlcgen` **只能出现在 `internal/database`（含子包）** | `internal/archtest/persistence_test.go:50-65` |
| C3 | `internal/database` 的**导出方法签名**不得泄漏 pgx / `database/sql` / sqlcgen 类型 | `persistence_test.go:85-104` |
| C4 | `Store` / `Tx` 不得有**导出字段**、不得嵌入能力、不得暴露 `Exec/Query/Begin/Commit/Queries/...` | `persistence_test.go:105-150` |
| C5 | `TestFixture` 仅测试与 `charactercheck` 可用；`DiagnosticQuery` 仅 `dbq` 工具可用 | `persistence_test.go:67-84` |
| C6 | 存档兼容最高优先级；PVF 唯一内容真源；不猜包；实机由业主操作 | 根 `AGENTS.md` §0 |

| C7 | 构建门禁：`go test ./...` + `go vet ./...` + `go build -trimpath`，CI 在 Windows runner 上跑 `server/Build-Server.ps1` | `server/Build-Server.ps1:10-14`；`.gitlab-ci.yml:26-41` |

**推论**：全部引擎代码只能落在 `internal/database` 及其子包；`Store` 的对外形状（**202 个导出方法**）应尽量保持不变，改造发生在包内与包外持有者的类型上。

**实测引用规模（更正上轮交接口径）**：`*database.Store` 共 **61 处、44 个文件**，其中 **43 个文件在 `internal/database` 之外**
（上轮文档记的"32 个文件"已过时）。分布：`cmd/wireprobe/**` 20 文件、`internal/workflow/**` 10 文件、
`internal/toolcmd/charactercheck/**` 14 文件、`internal/admin/grant.go`、`cmd/gmtool/main.go`、
`internal/character/store_test.go`（清单见附录 A）。

### 1.1 ⚠️ 必须先补的守卫缺口（否则分层保证静默失效）

`persistence_test.go:53` 只禁止 `github.com/jackc/pgx/` 与 `database/sql` 出现在 `internal/database` 之外，
而 `:56-65` 的"私有 import 别名表"也只有这两者 + `sqlcgen`。

⇒ **`modernc.org/sqlite` 目前既不在禁止驱动名单，也不在私有别名表**：
SQLite 代码一旦落在 `internal/database` 之外（或 `internal/database` 的导出签名里出现 `sqlite.*`），
守卫**不会报错**。这是 S0/S1 必须最先补的两处扩展（见 §7.2 G2/G3），
且必须在任何 SQLite 代码落库**之前**完成。

---

## 2. 引擎边界设计：一份逻辑、两份 SQL

### 2.1 问题

`Store` 是**具体类型**，私有持有 `db *pgxpool.Pool` + `queries *sqlcgen.Queries`（`internal/database/store.go:32-36`），
**202 个导出方法**分布在约 100 个文件中，包外 61 处引用。每个方法体里都内嵌同一套**事务协议**：
账号→角色→金库的锁顺序、幂等键回执、JSON `apply` 回调、错误映射。

若按"每个方法 `if engine == postgres {…} else {…}`"双写，就会得到 **202 份互相漂移的逻辑**，
且"行为等价"无法机械证明。**因此双写必须收敛到 SQL 与少量语义原语上，而不是收敛到方法体上。**

### 2.2 设计（推荐）

`Store` 仍无导出字段，但私有成员改为引擎无关能力：

```go
type Store struct {
    db      engine      // 方言无关：连接/事务生命周期、幂等重试
    queries querySet    // 手写、引擎中立的查询接口（唯一被方法体引用的 SQL 面）
    locks   lockDialect // 锁 / 编号 / 时间语义原语
    adventureEnabled bool
}
```

- **`querySet`**：手写接口，方法签名**只用 Go 原生类型**（`int64/int32/uint16`、`string`、`*string`、`[]byte`、
  `json.RawMessage`、`time.Time`、`[]int64`、`*int64`、`bool`）。规模以最终清单为准（按现有 SQL 面估算 120–180 个方法）。
- **两个适配器**：`postgresQueries`（包装 `sqlcgen` + pgx，做 `pgtype`/`int32`/`int16` ↔ 原生转换）与
  `sqliteQueries`（包装 `sqlcgen_sqlite` 或手写实现）。二者都满足同一 `querySet`。
- **202 个方法体保持单一实现**，只调用 `querySet` + `locks`；事务协议、锁顺序、幂等键、JSON 回调**单源**。
- 方言差异因此集中到三个可控面：**SQL 文本**、**`lockDialect` 原语**、**适配器类型转换**，三者都可并排差分评审。

### 2.3 为什么不直接让两个生成的 `Querier` 共用接口

sqlc 双引擎生成的 `DBTX` 结构**不兼容**，无法共享一个接口：

| 面 | postgresql / pgx/v5 | sqlite / database/sql |
| --- | --- | --- |
| `DBTX` 返回 | `pgconn.CommandTag` / `pgx.Rows` / `pgx.Row` | `sql.Result` / `*sql.Rows` / `*sql.Row` |
| `WithTx` 入参 | `pgx.Tx` | `*sql.Tx` |
| nullable | `pgtype.Int8`（调用点显式构造，如 `internal/database/mailbox.go:160`） | `sql.NullInt64` 或（开启开关后）`*int64` |
| 整数宽度 | `int32` / `int16`（如 `sqlcgen/core.sql.go:75,82`） | 一律 `int64` |

证据：`internal/database/sqlcgen/db.go:14-32`。
⇒ sqlc 双生成的价值是**类型安全的扫描代码**，不是"共用接口"。共用接口由 §2.2 的手写 `querySet` 提供。

### 2.4 S1/S2 实测更正（本轮取证推翻了两个原判）

对 202 个导出方法与 61 处引用做过实测后，有一项原判**不成立**、一项**被替换**：

| 原判 | 实测 | 结论 |
| --- | --- | --- |
| S2 的活是"把 `pgx.Tx` 从事务回调参数里挪走"（上轮交接文档口径） | **0 个导出方法的签名出现 `pgx.`/`pgtype.`/`sqlcgen.`/`pgxpool.`**；唯一的 pgx 签名是**未导出**的 `internal/database/adventure.go:268` | S2 的驱动类型收窄**基本已由现有守卫达成**（C3），S2 缩小为复核 + 少量收尾 |
| — | 真正的契约扩张点是 **30 个导出方法带 `func(...)` 回调**（闭包类型本身就是接口契约），其中 2 个是 `func(*Tx, ...)`：`character_event.go:45`、`account_material_event.go:31` | S1 必须把这 30 个回调签名一并纳入接口设计；`*database.Tx` 已在包外 4 处出现（`cmd/wireprobe/roster_background_flow.go:65`、`internal/workflow/shop.go:63,118,136`），需先定稿其能力面 |

补充事实（影响 S1 拆分）：

- **`Tx` 只有 3 个导出方法**（`shop_purchase.go:69,94`、`roster_background.go:98`），`newTx` 仅在包内构造。
- Store 上有 **26 个 `Commit*` 家族回调方法：25 个自己开事务，1 个不是**——
  `odyssey_honor.go:14 CommitOdysseyHonorMail` 故意跑在 `CommitCharacterEventTx` 交出的 `*Tx` 上；
  **这个例外是 `Tx` 被导出的唯一原因**，S2 定稿能力面时必须保留它。
- **1 处硬断言**：`cmd/wireprobe/booster_flow.go:246` 把接口参数向下断言成 `*database.Store`，
  只为调用 `odyssey_weapon_box.go:83 selectOdysseyWeapon`；断言失败会**静默退化**成通用错误分支（`booster_flow.go:195`）。
  ⇒ S1 先给 `selectOdysseyWeapon` 换窄接口。
- **`TestFixture` 是最大的具体类型耦合点，但它是"合法的例外"**：`fixture.go:24` 用类型别名
  `type fixtureStore = Store` 后 `:27` 嵌入具体指针，从而**提升全部 202 个方法**；`:48`/`:57` 直接用 `.db`/`.queries`。
  它被 14 个 `charactercheck/*_check.go` 与 2 个 wireprobe 测试使用。
  **建议**：`TestFixture` 保持具体类型（守卫 C5 已把它限定在测试与 `charactercheck` 内），
  G1 守卫相应地**排除测试文件与 `charactercheck`**，不强行接口化。
- **21 个导出方法在包外零调用**（例：`StoryDigestStore.AdvanceStoryDigest` 是死导出）⇒ S1 拆分接口时不必为它们造接口。
- **计数陷阱**：PowerShell `Select-String` 默认**大小写不敏感**，`^func \(s \*Store\) [A-Z]` 会多算 8 个未导出方法（210 而非 202）；
  统计请用 ripgrep 或 `-cmatch`。

### 2.5 S1 落地的两个关键决定（2026-10-04 实测）

**① `cmd/wireprobe` 用单一 `persistentStore`，而不是每个 flow 一个窄接口。**
理由是硬约束而非偏好：网关把**同一个** store 句柄经 `worldSession.store`、`client.gameStore` 与 ~15 个自由函数传递，
并且**还要把它赋进其它包声明的接口**（`world.Store`、`character.Store/ProgressionStore/FatigueStore`、
`quest.Store`、`workflow.{wear,loot,quest,item}Store`）。Go 要求源接口覆盖目标接口，逐 flow 窄接口会膨胀成
一堆互相缠绕的赋值兼容约束（子代理实测：按其"只看本文件调了啥"的做法**根本编译不过**，因为
`cmd/wireprobe` 会直接调用 `WearService.Store.AccountMaterials`、`ShopService.Store.AccountMaterials`、
`QuestService.Store.Quests`）。组合根本就该看到最宽的面；其下每层仍只声明自己需要的。
方法集是**机械生成**的（`internal/database` 真实签名逐字提取 + 本包调用并集 + 赋值目标接口方法集 − DDL/生命周期面），
签名**不要手抄**。

**② typed-nil 是这次改造最容易翻车的地方，必须显式处理。**
`bootstrap.go` 的 `gameStore` 在**未配置存储**时是 nil 指针，而 `client_dispatch_account.go:196` 与
`client_dispatch_inventory.go:510` 靠 `client.gameStore == nil` **关闭**整条功能。把字段直接改成接口，
nil 指针会变成**非 nil 的 typed-nil 接口**，`== nil` 变 false ⇒ 这些功能会在无存储模式下**静默打开**。
落地方式：保留 `var gameStore *database.Store`（只用于 DDL/生命周期与构造），另设
`var runtimeStore persistentStore`，**只在 `if startup.CharacterStorage != ""` 块内**赋值
（`runtimeStore = s`），返回结构用 `gameStore: runtimeStore`。这样无存储时是**真 nil 接口**，两个开关照旧生效。
后续任何"具体指针 → 接口字段"的赋值都必须走同一条规则。

---

### 2.6 S2 结论：引擎相关面清点（S3/S4 的处理清单）

S2 之后，`internal/database` 的**非测试代码里只剩这些**驱动相关内容，其余都已收敛为引擎中立形式：

| 符号 | 站点 | 位置 | S3/S4 必须怎么处理 |
| --- | --- | --- | --- |
| `pgx.ErrNoRows` | 1 | `driver_errors.go`（`isNoRows` 内） | 已同时接受 `sql.ErrNoRows` ⇒ SQLite 落地**不需要再改任何调用点** |
| `pgx.Tx` | 7 | `tx.go` 字段与构造、事务回调签名 | 收进 `*database.Tx`，不外泄 |
| `pgx.BeginFunc` | 6 | 事务助手 | 需引擎中立等价物（SQLite：`BEGIN IMMEDIATE` + busy 重试） |
| `pgx.TxOptions` | 3 | `store.go:93`（默认）、`diagnostic.go:24`（`ReadOnly`）、**`roster_background.go:21`（`RepeatableRead` + `ReadOnly`）** | ⚠️ **SQLite 没有隔离级别**：`RepeatableRead+ReadOnly` 的语义必须另找机制证明等价（WAL 读快照 / `BEGIN IMMEDIATE`），这是 S3/S4 的语义风险点 |
| `pgx.Identifier` | 2 | `fixture.go:47,81`（schema 引号转义） | S4 把测试隔离改成"每测试一个数据库文件"后整体消失 |
| `database/sql` | 1 | `driver_errors.go`（仅为 `sql.ErrNoRows`） | 位于 `internal/database` 内，符合既有守卫 |

可核查指标：S2 后 **18 个文件不再 `import pgx`**——引擎相关面在缩小，而不是扩散。
## 3. 方言差异逐项设计

### 3.1 本次直接统计（`internal/database/sql/postgres/**`，17 文件 93,214 字节）

| 构造 | 次数 | 构造 | 次数 |
| --- | --- | --- | --- |
| `FOR UPDATE` | 25 | `FOR NO KEY UPDATE` | 1 |
| `ON CONFLICT` | 57 | `FOR SHARE` | 2 |
| `RETURNING` | 17 | `::` 转型 | 220 |
| `jsonb` | 71 | `GENERATED` | 4 |
| `timestamptz` | 68 | `CREATE TABLE` | 57 |
| `INSERT INTO` | 78 | `REFERENCES` | 60 |
| `UPDATE` | 92 | `DELETE FROM` | 13 |
| `ANY(` | 5 | `unnest` | 2 |
| `pg_` 函数 | 2 | advisory 锁 | 2 |
| `DO $$` 块 | 2 | `SEQUENCE` | 1 |
| `MATERIALIZED` | 1 | `LATERAL` | 1 |

> 上一轮交接文档记的 `FOR UPDATE 25 / ON CONFLICT 57 / RETURNING 16 / :: 150 / jsonb 21 / GENERATED 21`
> 与本次实测部分不一致（`::` 220、`jsonb` 71、`GENERATED` 4、`RETURNING` 17）：上一轮按**列定义**计，本次按**词频**计。
> 以本文数字为排期依据，逐项重写清单见 §3.2 与附录 B。

### 3.2 必须重写项（不能机械翻译）

| # | PG 用法 | SQLite 侧设计 | 风险 / 判据 |
| --- | --- | --- | --- |
| D1 | `FOR UPDATE` / `FOR NO KEY UPDATE` / `FOR SHARE`（28 处，27 条查询） | SQLite 无行锁：**写事务一律 `BEGIN IMMEDIATE`**（DSN `_txlock=immediate`，见 §5），全局单写者下锁顺序自动串行化；`FOR SHARE` 的只读串行化改用 WAL 读快照 | **不能机械删除锁**；账号限购、存档原子性、回执幂等必须在契约测试里证明等价 |
| D2 | `GENERATED ALWAYS AS IDENTITY`（4 列：`accounts.id` `characters.id` `cash_inventory.id` `gm_mail.id`） | `INTEGER PRIMARY KEY AUTOINCREMENT`；转换时对齐序列高水位 | 角色/账号 id 必须**不重编号**；删除后不得复用 |
| D3 | `CREATE SEQUENCE mailbox_id_seq` + `nextval`（`0001_initial.sql:576,578`） | 单行计数器表 `UPDATE seq SET v=v+1 WHERE name='mailbox_id_seq' RETURNING v`，事务内分配 | **头号语义风险**，见 §6.2：保持"附件 id 在消息插入前分配、共享同一号段、附件 id < 消息 id、高水位 `max(id)`" |
| D4 | `jsonb` 列（10 表 11 列） | 存 **TEXT**（明确写 `TEXT`，**不要写 `JSONB`**：SQLite ≥3.45 把 `jsonb` 当关键字且有不同语义，是移植陷阱）；JSON 逻辑回 Go | 写前由 Go 统一 `json.Marshal`，**禁止依赖库侧规范化**（PG `jsonb` 会规范化键序/去重键，TEXT 不会）→ 直接影响存档哈希 |
| D5 | JSON 运算符 `->`/`->>`/`@>`/`\|\|`/`jsonb_set`/`jsonb_build_object`/`to_jsonb`/`jsonb_each`/`jsonb_array_elements`/`jsonb_typeof` | `json_extract` / `json_set` / `json_patch` / `json_each` / `json_type` / `json_object`；`to_jsonb(row)` 无对应物 → Go 侧构造；`@>` 包含（`commerce.sql:62,65`）→ `EXISTS(json_each(...))` 或 Go 循环 | `json_set` 与 `jsonb_set` 语义有细微差异（数组下标、文本保留），必须做往返验证 |
| D6 | `::` 转型（220） | 分四类改写：`sqlc.arg(x)::type` 去转型；`::text::date` → `date(?)`；`::timestamptz` → 见 §5 时间方案；`::jsonb` → `json(?)` | 逐条核对，注意 SQLite 动态类型与整数范围 |
| D7 | `ON CONFLICT`（57） | 语法支持（≥3.24），但 `DO UPDATE` **必须带冲突目标**；`ON CONFLICT DO NOTHING` 无目标可用 | `settings.sql:26,81` 用到 `IS DISTINCT FROM` → **最低版本必须 ≥3.39** |
| D8 | `RETURNING`（17） | 支持（≥3.35）；`INSERT/UPDATE ... RETURNING` 均可用 | 与 §0 的 ≥3.39 一并满足 |
| D9 | `ANY(array)`（5）/ `unnest`（2）/ `bigint[]` 列（1，`account_bleeding_mine_teams.members`） | 数组参数 → `IN (SELECT value FROM json_each(?))`（传 JSON 文本）或展开为 `IN (?,?,…)`；`unnest` → `json_each`；数组列 → TEXT 存 JSON 数组 + `json_array_length=4` 校验 | 参数从 `[]int64` 改为 JSON 文本，适配器负责编码 |
| D10 | `DO $$` 旧表/旧列探测（`0001_initial.sql:233,470`，含 `information_schema`、`to_regclass`、动态 `EXECUTE`） | SQLite 无 `DO`：探测移入 Go 迁移运行器（`migrations.go:48`），用 `PRAGMA table_info` / `sqlite_master` 判定后执行普通语句 | 旧存档升级路径必须保留原语义（探测失败不执行、成功才执行） |
| D11 | `ALTER TABLE ... ADD COLUMN IF NOT EXISTS`（6）/ `ALTER COLUMN`（2） | SQLite 不支持 `IF NOT EXISTS`，且不能加无默认值的 `NOT NULL` 列，`ALTER COLUMN` 完全不支持 → 同样移入 Go，由 `PRAGMA table_info` 守卫 | 迁移幂等性由 Go 保证 |
| D12 | advisory 锁（`core.sql:118` `pg_try_advisory_lock_shared`、`migrations.sql:3` `pg_advisory_xact_lock`） | 迁移锁由 `BEGIN IMMEDIATE` 取代；GM 互斥见 §3.3 | 语义：非阻塞失败、跨进程互斥、崩溃即释放 |
| D13 | `now()`（39）/ `interval '15 days'` / `'epoch'::timestamptz` | `CURRENT_TIMESTAMP` / `datetime('now','+15 days')`；时间列存储见 §5（推荐整数微秒） | 日/周边界计算（疲劳、周常）保持**在 Go 里**做 |
| D14 | `date` 列（6）+ `day::text`（11） | TEXT/整数存储；`day::text::date` → `date(?)` | Go 侧扫描类型与比较语义必须一致 |
| D15 | `bytea`（8） | `BLOB` 原样支持；`octet_length` → `length`；`DEFAULT ''::bytea` → `DEFAULT x''` | 无差异风险 |
| D16 | `DELETE ... USING`（1，`quests.sql:34`） | 改 `DELETE ... WHERE ... IN (SELECT ...)` | 无语义差异 |
| D17 | `MATERIALIZED` CTE + 数据修改 CTE（`repairs/*.sql:25,37`） | 去 `MATERIALIZED`；数据修改 CTE 拆成同一事务内两条语句 | `repairs/` 整体是 PG 专用运维脚本，见 §3.4 |
| D18 | `search_path` 测试隔离 | 改为**每测试一个临时数据库文件** | `fixture.go:48` 的 `CREATE SCHEMA` + `fixture.go:57` 的 `FixtureSchema` 校验在 SQLite 下改为 `PRAGMA database_list` / 文件路径校验 |
| D19 | `current_database()` / `current_schema()` | `PRAGMA database_list` 或由 Go 返回配置路径 | `core.sql:96` `DatabaseName`、`fixtures.sql:3` `FixtureSchema` 列为**引擎特有诊断** |
| D20 | `IS DISTINCT FROM`（7） | 原样支持（≥3.39） | **这是把下限从 3.35 提到 3.39 的唯一原因** |
| D21 | `GREATEST`（4）/ `FILTER (WHERE)`（5）/ `UPDATE ... FROM`（9）/ `INSERT ... SELECT`（12）/ 部分索引 / 表达式索引 | 原样支持 | `LoadFatigue`（`progression.sql:46`）是本组最复杂查询（`ON CONFLICT DO UPDATE` + `EXCLUDED` + `GREATEST` + `RETURNING`），必须有专项差分测试 |
| D22 | `LIMIT` / `OFFSET` 顺序 | **SQLite 要求 `LIMIT` 在 `OFFSET` 之前**（PG 两者皆可） | 实测：`ORDER BY x OFFSET ? LIMIT 1` 直接语法报错；已改写为 `LIMIT 1 OFFSET CAST(sqlc.arg(roster_slot) AS INTEGER)` |
| D23 | **SQL 文件含非 ASCII 字符** | ⚠️ **工具约束（本轮实测）**：sqlc v1.31.1 的 sqlite 引擎会把含非 ASCII 的语句解析成乱码，报出 `extraneous input ')'` / `'d'` / `'t'` 这类单字符错误 | 同一查询：注释里带 `§` 或中文 ⇒ FAIL；纯 ASCII（甚至 23 行纯 ASCII 注释）⇒ OK。⇒ **两个方言树的 `.sql` 必须保持纯 ASCII**（PG 树本来如此） |
| D24 | JSON 参数的 Go 类型与存储类（**S4 实测后修正**） | `sqlc.arg(jsonCol)` ⇒ `json.RawMessage`（与 PG 侧一致）；`CAST(sqlc.arg(jsonCol) AS TEXT)` ⇒ `string`。**硬约束其实在读方向**：列存 TEXT 时驱动返回 Go `string`，而 Go 1.26 的 `database/sql` **拒绝把 `string` 扫进 `json.RawMessage`**（实测原文：`unsupported Scan, storing driver.Value type string into type *json.RawMessage`）⇒ TEXT 方案会让生成的行类型**直接不可用** | **最终决定：JSON 列存 BLOB**（`state BLOB` + `CHECK(json_valid(...))`），参数**不加 CAST**。这样 `json.RawMessage` 在**读写两个方向都成立且零适配器转换**，与 PG 侧类型完全对齐；`json_valid`/`json_extract`/`json_set` 对 BLOB 同样有效（已在驱动的 SQLite 3.53.4 实测）。代价：`typeof()` 是 `blob`，用 sqlite 浏览器人工看是二进制；但存进去的就是 Go 交给它的一模一样字节，符合 D4「写前由 Go 统一 Marshal、禁库侧规范化」 || D25 | 裸表达式 / 多余括号 | sqlc 推不出类型 ⇒ `interface{}`，**进不了统一 `querySet` 接口** | 实测：`max(max_fame,sqlc.arg(x))` ⇒ `interface{}`；`(coalesce(max(coalesce(a,b)),0)+1)` ⇒ `interface{}`；加 `CAST(... AS <type>)` 后分别为 `int64`。**R-1 规则确认：表达式列与表达式内参数一律显式 CAST** |
| D26 | `sqlc generate` 输出 LF | 本工作树是 CRLF（`core.autocrlf=true`、无 `.gitattributes`） | 生成后 `git status` 会把 15 个 PG 生成文件标成改动，但 `git diff` **为空**（纯行尾差异）。⇒ 每次生成后需把生成物刷回 CRLF；根治办法是加 `.gitattributes`（待业主定） |
| D30 | 整数宽度与**表达式类型推断**（S3a 实测） | sqlc 的 sqlite 引擎：**列引用尊重 `db_type` override**（`SMALLINT`→int16、`INTEGER`→int32 已实测）；但 **CAST 目标恒为 int64**（`CAST(x AS INTEGER/SMALLINT/BIGINT)` 三者结果一样），且**聚合不继承列宽**（`max(col)`→int64，`max(裸表达式)`→`interface{}`） | ⇒ ① SQLite DDL 的整数列**按 PG 声明镜像宽度名**（运行时等价），并在 `sqlc.yaml` 用 db_type override 映射；identity 列因 AUTOINCREMENT 必须是 `INTEGER`，用列级 override 钉回 int64。② 无法对齐的 4 类位置登记在门禁的 `acceptedDivergences` 中：`WithTx` 的事务句柄（`pgx.Tx` vs `*sql.Tx`）、`0::smallint` 投影、`::integer` 参数、`coalesce(max())+1` 的 int32 —— **每个对应一个手写适配器方法**；出现未登记偏差门禁即失败，登记项失效也会失败（防陈旧） |

### 3.3 GM 互斥锁（`admin_guard.go`）的 SQLite 实现

现状：`admin_guard.go:23` 在**专用池连接**上取 `pg_try_advisory_lock_shared(11520260922)`，
持锁整个会话，释放时**关闭连接**而不是归还池（`:20-21`），保证锁不泄漏；调用方依赖"取不到锁 = 已有 GM 写入"。

SQLite 无 advisory 锁。两个方案：

| 方案 | 互斥 | 非阻塞 | 崩溃释放 | 代价 |
| --- | --- | --- | --- | --- |
| **A. 租约行 + `BEGIN IMMEDIATE`**（建议） | 跨进程 | 是（条件更新 0 行即失败） | 需**租约超时 + 显式接管** | 只加一张表，无新依赖；要求实现接管逻辑 |
| B. 独立锁文件独占打开 | 跨进程 | 是 | 由操作系统保证 | 需平台相关文件锁语义；Windows 需 `LockFileEx` 或独占共享模式 |

**提议 A**，并在方案落地时把"租约超时值 + 接管条件"写进配置；方案 B 作为 A 不可用时的回退。此项列入 §11 待确认。

### 3.4 PG 专用清单（SQLite 下**明确拒绝**，不做"看似可用"）

| 项 | 位置 | SQLite 行为 |
| --- | --- | --- |
| `DiagnosticQuery` / `DiagnosticExec` | `internal/database/diagnostic.go`（只读事务 + 关闭连接清理会话锁） | 拒绝并给出可诊断错误（或另实现 `sqlite_master` 子集，见 §11） |
| `dbq` 工具 | `internal/toolcmd/dbq`（守卫白名单） | 同上 |
| `repairs/dark_knight_combo_slots_v1.sql` | 含 `SET LOCAL lock_timeout` / `LOCK TABLE ... SHARE ROW EXCLUSIVE` / `RAISE EXCEPTION` / `MATERIALIZED` | **保持 PG 专用**，作为运维脚本运行 |
| `FixtureSchema` / `DatabaseName` | `fixtures.sql:3` / `core.sql:96` | 引擎分支实现（§3.2 D19） |

---

### 2.7 S3b 定稿：querySet 的canonical 形状与两段式落地（2026-10-04 实测）

**已交付（新增文件，编译期已证）**：

- `internal/database/queryset.go`（由 `.tmp/gen-queryset.go` 从 PostgreSQL 生成包派生）：
  `type querySet interface` 共 **281 个方法**，即 PG 生成面去掉 `WithTx`。canonical 形状
  **就是 PG 的形状**（Store 既有代码已经在用的那套类型）。
- **`var _ querySet = (*sqlcgen.Queries)(nil)` 编译通过** ⇒ PostgreSQL 侧**零适配器代码**即可接入。
- 两个守卫：`TestQuerySetCoversGeneratedSurface`（方法集合双向对齐，防止新查询加进来而接口没重生成）
  与 `TestPostgresEngineNeedsNoAdapter`（形状一致性）。

**本轮实测推翻的一个设想**：原计划"两个生成包各嵌入自己的 `*Queries` 即可满足同一接口"，
**不成立**。生成的 param/row 是**包内类型**（`sqlcgen.CreateCharacterParams` vs
`sqlcgensqlite.CreateCharacterParams`），名字相同但**不是同一个类型**，Go 的嵌入只提升方法、不做类型转换。
因此凡是签名里出现结构体的方法都需要一次显式转换。所幸 Go 允许**同构结构体直接转换**，
所以绝大多数是"一行转换"，只有 41 处已登记分歧需要逐字段转换。

**因此 SQLite 适配器 = 三段**：

1. 嵌入 `*sqlcgensqlite.Queries`；
2. **41 处**逐字段转换（`acceptedDivergences` 逐条对应：整数宽度、JSON `[]byte` ↔ `json.RawMessage`、
   数组 JSON 文本、`pgtype` ↔ 指针、`RETURNING` 的 `ColumnN` 改名、整数微秒 ↔ `time.Time`）；
   其中数组与时间转换需要返回 error，故这些方法体需要错误处理；
3. **5 个引擎专属实现**：`DatabaseName`（`pragma_database_list`）、`NextMailID`（`sqlite_sequence`，
   并记录邮件号段不变式差异）、`TrySharedAdminGuard`（§3.3 租约）、`FixtureSchema`（临时库名）、
   `LockMigrations`（空操作，`BEGIN IMMEDIATE` 已提供同一保证）。

**补充（适配器生成后实测）**：转换器必须**按字段名**映射，不能按下标。一类此前没预料到的分歧是
**字段顺序不同**——Go 要求字段名、类型、**顺序**都一致才允许结构体直接转换，而移植后的 SQLite 查询列序
可能与 PG 不同（例：`AbandonQuestParams` 在 PG 为 `CharacterID, AccountID, QuestID`，在 SQLite 为
`CharacterID, QuestID, AccountID`）。按名字映射可同时解决重排与类型差异。
实测规模：生成 277 个方法，**241 个可直接编译**，**36 个需要转换器**，**5 个需手写**（见上）。

**事务缝（本阶段唯一的大改）**：`s.queries.WithTx(tx)` 在 **约 50 处**被使用，且返回**具体类型**
`*sqlcgen.Queries`。共享接口无法表达事务句柄，因此改为一个极小的引擎缝：

```go
type engine interface {
    queries() querySet                        // 非事务句柄
    inTx(ctx context.Context, fn func(querySet) error) error  // 事务句柄
    close() error
}
```

PG 侧：`pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error { return fn(s.queries.WithTx(tx)) })`；
SQLite 侧：`db.BeginTx` 后 `fn(newSQLiteQueries(tx))`。这 ~50 处调用点位于**被跟踪文件**中。

**风险（须业主知悉）**：未提交期间，对被跟踪文件的改动可能被整体回退——2026-10-04 20:14 已发生过一次
（模块内所有被跟踪文件被打回 HEAD，仅未跟踪文件存活）。因此本阶段建议：**先完成 SQLite 适配器
（可全部放在新增文件里，零风险），再单独做 Store 的事务缝改造（唯一触碰被跟踪文件的一步）**，
并在改造前确认可以提交。

### 3.5 引擎专属查询：最终清单（S3a 实测，13/13 文件已移植）

SQLite 查询树已铺满 13 个文件，**275/282 个生成方法**。剩下 7 个不是遗漏，每个都有明确归属：

| 方法 | 归属 | 依据 |
| --- | --- | --- |
| `CharactersWithAdventure`、`CharacterSeason` | **可移植，下轮补** | 需 `json_set`/`json_object`；结果是表达式列，会产生 `[]byte` vs `json.RawMessage` 偏差，由适配器转换 |
| `DatabaseName` | 引擎专属，写手写适配器 | PG 用 `current_database()`；SQLite 等价物是 `pragma_database_list` |
| `NextMailID` | 引擎专属，写手写适配器 | PG 用 `nextval('mailbox_id_seq')`；SQLite 的 `AUTOINCREMENT` 在 INSERT 时才分配，见下方行为差异 |
| `TrySharedAdminGuard` | 引擎专属，**需设计**（§3.3） | PG advisory lock；SQLite 无对应物，方案是租约行/文件锁 |
| `FixtureSchema` | PG 专属，不移植 | `current_schema()` 目录诊断。注意：`fixtures.sql` **本身没有** `CREATE SCHEMA`/`SET search_path`/标识符引号——那些在 Go 侧（`internal/database/fixture.go:48/57/81`）；SQLite 用「每测试一个临时数据库文件」实现隔离（D18） |
| `LockMigrations` | PG 专属，不移植 | `pg_advisory_xact_lock(...)`；`BEGIN IMMEDIATE`（`_txlock=immediate`）已提供事务级、崩溃即释放的写序列化（D12） |

**必须让业主知悉的行为差异（邮件 id 分配）**：PG 侧 `NextMailID` 从共享序列取号，附件 id 先于消息 id 分配、
两者共用同一号段。SQLite 侧 `character_mail.id INTEGER PRIMARY KEY AUTOINCREMENT` 只在 INSERT 时分配，
**无法复现"预分配"与"附件/消息共用号段"这一不变式**。用 `max(id)+1` 或 `sqlite_sequence` 冒充会改变该不变式，
因此不做猜测式移植。若 SQLite 要成为单机默认，邮件功能需要业主确认可接受的替代语义
（例如：接受 id 在插入时分配、附件号段独立），或明确 SQLite 下不支持该不变式。

### 3.6 S4b 主体：Store 接线清单（2026-10-04 实测，一次做完才可编译）

引擎缝（`engine`：`queries()` / `inTx()` / `close()`）与 SQLite 适配器已就位并验证；
把 `Store` 接到缝上是**唯一大范围触碰被跟踪文件**的一步。实测规模如下（不是估算）：

| 项目 | 数量 | 改法 |
| --- | --- | --- |
| `Store.db *pgxpool.Pool` + `Store.queries *sqlcgen.Queries` | 1 | 换成 `engine engine` + `queries querySet`（`queries` 保留，约 200 处 `s.queries.X(...)` 因此无需改动） |
| `s.db.Begin(ctx)` 手工事务 | **47** | 改为 `s.engine.inTx(ctx, func(q querySet) error { … })`：`defer tx.Rollback(ctx)` 与 `tx.Commit(ctx)` 一并消失（这是净简化） |
| `pgx.BeginFunc(ctx, s.db, …)` | **7** | 同上 |
| `sqlcgen.New(tx)` | **37** | 改为直接用回调拿到的 `q` |
| `pgx.Tx` 形参的辅助函数 | **14 处 / 6 个函数**（`lockCharacter`、`lockAdventure`、`saveAdventure`、`commitAdventureExperience`、`insertSystemMailTx`、`newTx`） | 形参改为 `q querySet` |
| `*Tx` 回调面（`CommitCharacterEventTx` 等） | **10** | `tx.go` 的 `Tx` 持有 `querySet` 而非 `pgx.Tx`，`newTx(q, account, character)` |
| `s.db.Acquire`（管理守卫） | 2 | 收进引擎：`holdAdminGuard(ctx) (func(), error)`（PG：独占连接 + advisory lock；SQLite：租约文件 + 释放时删除） |
| `s.db.Exec` / `s.db.BeginTx`（fixture） | 3 | 收进引擎的 fixture 支持（PG：临时 schema；SQLite：每测试一个数据库文件，D18） |
| `s.db.Close()` / `s.db == nil` 判空 | 1 + 3 | `engine.close()` / `queries == nil` |
| `Open` 建立连接 | 1 | 按配置 `driver` 分派：`postgres`（pgxpool）或 `sqlite`（`openSQLite` + `migrateSQLiteAll`） |

**纪律**：这批改动必须**一轮做完**（包必须能编译），且改完立即跑 `go build`/`go vet`/
`internal/database` 与 `internal/archtest`，再与基线对照失败集合。转换是机械的，
因此可用脚本重放——考虑到 20:14 已发生过一次"模块内被跟踪文件整体回退"，
建议提交后再做，或做完立刻验证并保留脚本。

## 4. S3 前置门禁：sqlc 双引擎 spike（决定 A/B 路径）

### 4.1 已核实的事实（pinned 版本 v1.31.1）

| 事实 | 来源 |
| --- | --- |
| `engine` 取值包含 **`sqlite`**；`sql_package` 为 `database/sql` | [sqlc v1.31.1 配置文档](https://docs.sqlc.dev/en/v1.31.1/reference/config.html) |
| `emit_pointers_for_null_types`：**"only supported for PostgreSQL if sql_package is pgx/v4 or pgx/v5, and for SQLite"** ⇒ 两边都能生成 `*string`/`*int64` | 同上 |
| 多引擎配置可用**全局 `overrides`**，且该项支持 `engine:` 字段区分引擎 | 同上 |
| 生成门禁硬校验版本：`sqlc version` 必须等于 `v1.31.1` | `scripts/Generate-SQL.ps1:6-9` |

### 4.2 两个必须先清掉的阻塞

| # | 阻塞 | 事实 | 处理 |
| --- | --- | --- | --- |
| B1 | **本机没有 sqlc v1.31.1** | `Get-Command sqlc` 失败；`tools/` 无 sqlc；`go/bin`、`tools/gopath/bin` 均无 ⇒ `Generate-SQL.ps1` 当前**无法运行** | ✅ **已解决（2026-10-04）**：`go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1`（`GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=off`）安装到 `C:\Users\Ricar\go\bin\sqlc.exe`，`sqlc version` = `v1.31.1`；基线 `-Check` 通过（见 §4.5） |
| B2 | `Generate-SQL.ps1` 只支持**单** SQL 块 | `:20-22` 用字符串替换重写 `sql/postgres/{migrations,queries}` 两条路径与 `internal/database/sqlcgen` 输出；新增第二个 `sql:` 条目后，SQLite 的路径/输出不会被重写 ⇒ `-Check` 的 scratch 校验必然失败 | ✅ **已解决（2026-10-04）**：改为**无表**通用重写——解析 yaml 中**每一条** `schema`/`queries`/`out` 并逐条重定位（输入指回仓内、输出落 scratch），加"任一声明路径未被重定位即报错"的断言；`sqlcgen` 前缀碰撞由"替换带引号的整 token"规避；缺 sqlc 时给出可操作报错。单引擎 `-Check` 仍绿且**不改动已入库生成物** |

### 4.3 已知风险

- **schema 必须分叉**：`sqlc.yaml:4` 的 `schema` 指向 migrations 目录，而 `DO $$` 块与
  `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` 会让 sqlite 引擎解析失败 ⇒ 需新增 `sql/sqlite/migrations/`
  （不是只加 queries）。
- **override 在 sqlite 引擎的有效性存疑**：sqlc 曾有"列 override 对 SQLite 引擎无效"的缺陷记录
  （[sqlc#1985](https://github.com/sqlc-dev/sqlc/issues/1985)）⇒ 必须在 v1.31.1 上**实测**，不能假设。
- `ANY(...)` / `unnest` / `FOR UPDATE` / `LOCK TABLE` / `SET LOCAL` 在 sqlite 引擎下是解析或类型推断错误，
  这些查询注定要重写（§3.2 已列）。

### 4.4 spike 内容与判定（0.5–1 工程日）

1. 取 `characters` + `character_mail`（或 `character_events`）两张表 + 4 条代表性查询：
   ① 锁查询改写版（`LockAccount`）② `ON CONFLICT + EXCLUDED + GREATEST + RETURNING`（`LoadFatigue` 型）
   ③ JSON 读写 ④ 列表/`ANY` 改写版（`CompletedQuestIDs` 型）。
2. 双生成：`out: internal/database/sqlcgen`（pgx/v5）与 `out: internal/database/sqlcgen_sqlite`（database/sql），
   `emit_pointers_for_null_types: true`，per-engine `overrides`。
3. 写约 20 行手写 `querySet` 子集 + 两个适配器，**用编译期断言**证明两个生成包都能满足同一接口。
4. 产出"sqlite 解析/推断失败清单 + override 是否生效"。

**判定**：失败面 > 20% 或列 override 在 v1.31.1 对 sqlite 无效 → 走 **B 路径**。

| 路径 | 内容 | 取舍 |
| --- | --- | --- |
| **A（优先）** | PG 与 SQLite 都由 sqlc 生成（pgx/v5 + database/sql），各自薄适配器 | 两侧都有类型安全扫描；代价是 B2 的脚本改造 + 解析失败项要手写兜底 |
| **B（回退）** | PG 继续用 sqlc；SQLite 侧**手写 SQL + 统一扫描辅助**，实现同一 `querySet` | 少一个生成器、完全可控；代价是 SQLite 侧无类型安全，靠契约测试兜底 |

**两条路径都不改变 §2 的接口缝设计**，因此 S1/S2 可以先做，不被 spike 阻塞。

### 4.5 spike 实测结果（2026-10-04 已执行 → 判定 **A 路径**）

环境：sqlc **v1.31.1**（本轮安装）。样本：4 张表的 SQLite DDL（`accounts`/`characters`/`character_fatigue`/
`character_mail`）+ 11 条查询，覆盖 §4.4 的四类风险。复现物在 `.tmp/spike/`（gitignored）。

| 验证项 | 结果 | 证据 |
| --- | --- | --- |
| `engine: "sqlite"` 解析既有查询风格 | ✅ **7 条全过** | 含 `ON CONFLICT ... DO UPDATE ... EXCLUDED ... CASE ... RETURNING`、`json_extract`、`json_each`、部分索引、`CHECK(json_valid())`、`AUTOINCREMENT` |
| `emit_pointers_for_null_types` 对 **SQLite** 生效 | ✅ `SenderID *int64` | 这是与 pgx 侧签名对齐的**关键杠杆**（两侧都支持该开关） |
| **列级 override 对 SQLite 是否生效**（sqlc#1985 的疑虑） | ✅ **生效**：`characters.state` / `character_mail.assets` → `json.RawMessage` | **v1.31.1 上不存在该缺陷**，S3 最大未知项被消除 |
| `DATETIME` → `time.Time` | ✅（经 `db_type: "DATETIME"` override） | 非空 → `time.Time`；可空 → `*time.Time`（`pointer: true`） |
| `ON CONFLICT DO NOTHING`（无冲突目标） | ✅ 解析通过 | 对应 PG 侧 `core.sql:25`、`inventory.sql:19` 的用法 |
| `FOR UPDATE` / `FOR SHARE` / `FOR NO KEY UPDATE` | ❌ **语法错误** | `sqlite\queries_lock.sql:1:1: extraneous input 'FOR' expecting {...}` ⇒ 28 处锁查询**必须重写**（D1），坐实"不能机械翻译" |
| 裸表达式列/参数的类型推断 | ⚠️ → `interface{}` | `json_extract(...) AS level` 与 `json_each(?)` 都推不出具体类型 |

**由此产生两条强制改写规则**（补充 D5/D9）：

| 规则 | 内容 | 证据 |
| --- | --- | --- |
| **R-1 显式 CAST** | 任何**裸表达式**列或参数（`json_extract`/`json_each`/聚合）必须写成 `CAST(... AS <type>)`，否则生成 `interface{}`，进不了统一 `querySet` 接口 | `CAST(json_extract(state,'$.level') AS INTEGER) AS level` → `level int64`；`json_each(CAST(sqlc.arg(ids) AS TEXT))` → `ids string` |
| **R-2 参数命名** | 参数被函数包裹时（`lower(?)`）sqlc 用**函数名**当参数名 ⇒ 必须用 `sqlc.arg(name)` 固定参数名，否则两侧签名对不上 | 实测生成 `LockAccount(ctx context.Context, lower string)` |

**判定：走 A 路径** —— SQLite 侧继续由 sqlc 生成（`sql_package: database/sql`），不必退回手写 SQL。
但必须清楚：**SQLite 侧 query 文本仍是"手工镜像改写"**（锁、`::`、jsonb 运算符、`ANY/unnest`、`DO` 块都要改，见 §3.2），
sqlc 的收益是**类型安全扫描 + 生成物一致性门禁**，不是"免改写"。

**A 路径的两个前提**：① 每个引擎的 schema/queries 目录必须分叉（`DO` 块与 `ALTER ... IF NOT EXISTS`
会让 sqlite 解析失败，见 §4.3）；② 生成门禁脚本必须支持多引擎（§4.2 B2，本轮已改造并验证）。

---

## 5. 运行时接入：连接、PRAGMA、时间、配置与启动链

以下参数名与语义**直接取自驱动源码/文档**（本机 `modernc.org/sqlite@v1.59.0` 已缓存，
`driver.go:93-219`、`sqlite.go:355-404`），不是推测。

### 5.1 建议 DSN

```
file:<path>?_txlock=immediate
           &_busy_timeout=5000
           &_foreign_keys=1
           &_journal_mode=WAL
           &_synchronous=NORMAL
           &_timezone=UTC
           &_inttotime=1
           &_time_integer_format=unix_micro
           &_dqs=0
```

| 参数 | 作用 | 为什么必须显式给 |
| --- | --- | --- |
| `_txlock=immediate` | 每个事务以 `BEGIN IMMEDIATE` 开始（取值 `deferred`(默认)/`immediate`/`exclusive`） | 取代 `FOR UPDATE`：写事务立即拿写锁，避免"读事务中途升级写锁"导致的 `SQLITE_BUSY` 死局 |
| `_busy_timeout=5000` | 锁冲突时的等待上限 | 与 `_txlock=immediate` 配合把冲突变成有界等待 |
| `_foreign_keys=1` | 打开外键 | PG 一直强制 FK（schema 有 59 处 `REFERENCES`）；SQLite 默认**关闭** |
| `_journal_mode=WAL` `_synchronous=NORMAL` | 并发读 + 崩溃安全 | 单机单写者场景的标准组合 |
| `_timezone=UTC` | 时间读写统一 UTC | 日/周边界与存档时间可比 |
| `_inttotime=1` + `_time_integer_format=unix_micro` | 时间列以**整数微秒**存储与扫描为 `time.Time` | 与 PG `timestamptz` 的**微秒精度精确对齐**，整数比较与排序无歧义（备选：`_time_format=sqlite` 的定宽 TEXT，二者只能选一，由契约测试钉住） |
| `_dqs=0` | 关闭双引号字符串兼容怪癖 | 移植期把误用变成**解析错误**而不是静默当字符串 |

驱动行为要点（同样来自源码）：
- **shorthand 键会被校验**，拼错直接连接失败，且失败的 DSN 不会留下"已执行到一半的 PRAGMA"
  （`driver.go:120-123`）——只有 `_pragma` 是逐字执行、不校验的（`:125-129`），故 `_pragma` 来源必须可信。
- **连接归还池后不会被重置**：PRAGMA 类状态必须走 DSN 或连接钩子，**不能在池上 `Exec` 一次了事**
  （包文档 "Connection-scoped state outlives the caller that set it"）。
- **`database/sql` 默认不限制连接数**，每连接自带 page cache 与 libc 线程态
  （包文档 Performance 段）⇒ 必须 `SetMaxOpenConns`/`SetMaxIdleConns` 有界；
  SQLite 侧建议把 `MaxConnections` 映射为 `SetMaxOpenConns(4)`，写序列化由 `BEGIN IMMEDIATE` 保证、**不靠池大小**。
- **最低版本校验**：启动时读 `sqlite_version()`，低于 **3.39** 直接拒绝启动（防将来换驱动/降级导致 `IS DISTINCT FROM` 静默失效）。
- **依赖脆弱性**：`modernc.org/libc` 必须与驱动 `go.mod` 钉住的版本**完全一致**（本例 v1.75.7，本机已缓存）；
  禁止随手 `go get -u`。
- **性能**：CPU 密集负载比 C 版慢 1.3–2.0x；对 `ORDER BY/GROUP BY/WHERE` 用到的列建索引，
  用 `EXPLAIN QUERY PLAN` 复核 `USE TEMP B-TREE`。单机场景可接受。

### 5.2 重试与错误映射

- `SQLITE_BUSY` / `SQLITE_BUSY_SNAPSHOT`：**仅对未提交事务**做有界退避重试（建议 5 次，10ms→160ms 抖动）；
  幂等键（`character_events`、`account_vault_events`、`cash_orders`、`admin_grants`）保证重试不产生重复发放。
- `sql.ErrNoRows` → `ErrNotFound`：`store.go:41-46` 的 `storageError` 扩展一处即可（现有逻辑只认 `pgx.ErrNoRows`）。

### 5.3 配置扩展（向后兼容）

`Config` 现状只有 3 个键（`store.go:16-20`），且 `LoadConfig` = `os.ReadFile` + `json.Unmarshal`
**不拒绝未知键**（`store.go:22-30`）⇒ 新增键对既有 `local.json` 零影响。建议形状：

```json
{
  "driver": "postgres",
  "postgres_dsn": "…",
  "max_connections": 12,
  "postgres_schema": "",
  "sqlite_path": "runtime/storage/dfo_lan.sqlite3",
  "sqlite_busy_timeout_ms": 5000,
  "sqlite_max_open_conns": 4
}
```

| 规则 | 内容 |
| --- | --- |
| `driver` 缺省 | **必须是 postgres**：`Open` 当前在 `PostgresDSN == ""` 时硬失败（`store.go:49-51`），保持该兼容语义 |
| 未知 `driver` 值 | 启动即失败，**不静默回落**（项目铁律：禁止静默换源） |
| 旧键 | `postgres_dsn`/`max_connections`/`postgres_schema` 语义不变；`postgres_bin`/`postgres_data` **Go 完全不读**，仅 Python 用 |
| 池上限映射 | PG：`cfg.MaxConns = MaxConnections`（仅当 `>0`，`store.go:56-58`）；SQLite：同值映射到 `SetMaxOpenConns`/`SetMaxIdleConns`，但需按 §5.1 有界（SQLite 侧另有 `sqlite_max_open_conns`） |
| profile 不含存储键 | `configs/pvf-default.json` 只有 `binary`+`environment`，`repair_profile.py:30-33` 强制该校验；存储路径由 `channel_probe.py:243-244` 的 `-character-storage` 传入（可用 `DFO_CHARACTER_STORAGE` 覆盖，`:453-459`） |

**会被配置面变更打破的既有测试（必须同轮处理）**：

- `server/work/dfo-lan/scripts/test_postgres_storage.py:81` 断言 `local.json` 键集**恰好等于** 4 个键
  ⇒ `bootstrap_local.py` 一旦输出 `driver` 键，该测试失败；
- `server/work/dfo-lan/scripts/test_postgres_storage.py:54` 断言 `pg_ctl` 参数序列、`:79` 断言 `initdb/pg_ctl/createdb` 顺序；
- `scripts/test_environment_storage.py:58` 断言 25438 监听检查。

### 5.4 启动链与打包分叉（Python 编排侧）

启动顺序（实测，硬依赖）：**PG 起 → 服务端 `Open`+`Ping` 成功 → 写 `ready.json` → 起 `probe.exe` → 客户端**。
依据：`launch_local.py:239` 起 PG（**仅 TCP 探活，无 SQL ping**）→ `channel_probe.py:581` 起服务端二进制 →
服务端在 `bootstrap.go:489` 调 `database.Open`，`store.go:67` `Ping` 失败即 `main.go:36` `log.Fatal`，
进程退出且**永不写 `ready.json`** → `channel_probe.py:583-592` 轮询超时。

`driver=sqlite` 时需要分叉/跳过：

| 位置 | 现状 | SQLite 处理 |
| --- | --- | --- |
| `scripts/launch_local.py:44-60` | 无条件 `urlparse(cfg["postgres_dsn"])` 并拒绝非回环 | 按 `driver` 分支 |
| `scripts/launch_local.py:51-55` | 缺 `storage/local.json` 直接失败 | 改为要求 `sqlite_path` 可写 |
| `scripts/launch_local.py:63-92` | `start_storage`：`pg_ctl -D <pgdata> -l postgres.log -w -t 30 start` | 整体 no-op |
| `scripts/launch_local.py:223-226,238-242` | `--check` 打印 PG 状态；`--storage-only` 语义 | 按驱动分支 |
| `scripts/bootstrap_local.py:1-45` | **整文件 PG 专用**（`initdb`/`createdb`/`pg_ctl`、默认 25438、写 4 键 `local.json`） | 需 SQLite 分支（建库文件即可，无 initdb） |
| `scripts/configure_env.py:266-303` | 硬编码 `tools/pg/pgsql/bin`、`pgdata`、字面 DSN、`pg_ctl.exe`+`PG_VERSION` 校验 | 按驱动分支；`redis_*` 清理逻辑保留 |
| `scripts/stop_environment.py:67-99,112-117` | `pg_ctl stop -m fast` 做 checkpoint；25438 仍开则 `taskkill postgres.exe` | 改为 `Close` 时 `PRAGMA wal_checkpoint(TRUNCATE)` |
| `build-publish.ps1:43-49,67` | 发布包**拷贝 `tools/pg`** 并断言 `initdb.exe` 存在 | 按驱动决定是否打包 PG——**这正是"环境最小化"的收益点** |

另：**测试库 25439 在已提交脚本中并不存在**（仅见于 `docs/database-sqlc-migration.md（2026-10-05 已删除）:115,127` 与上轮交接文档），
无可复用测试集群；SQLite 路线下改为**每测试一个临时数据库文件**（§7.1）。

### 5.5 构建 / CI 事实

| 事实 | 结论 |
| --- | --- |
| `CGO_ENABLED=0` 是工具链默认，repo 内无任何脚本覆盖 | `modernc.org/sqlite` 直接可用；`mattn/go-sqlite3` 需在所有入口翻 CGO 并带 C 工具链 ⇒ **本轮选型正确** |
| 无 `vendor/`，`GOFLAGS` 为空 | 新增依赖走模块缓存；`modernc.org/libc` 版本须与驱动一致（§5.1） |
| `sqlc` 生成门禁是**可选**的（`Build-Server.ps1:7-9` 需显式 `-CheckSQL`） | CI **默认不检查生成物漂移**；S3 后建议把 `-CheckSQL` 纳入 CI，否则双引擎 SQL 会静默漂移 |
| GOPROXY 默认 `proxy.golang.org`（本机不可达） | 拉 SQLite 依赖与 sqlc 时用 `GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=off` |
| **工作树是 CRLF**（`core.autocrlf=true`，无 `.gitattributes`），但 `gofmt`/`go fmt` 一律改写成 LF | ⚠️ **不要在本仓跑 `gofmt -w` / `go fmt`**：它会把整包文件的行尾刷成 LF，污染其它 agent 正在修改的文件，并让 `git status` 出现大量无关变更。新文件请写成 CRLF。另注：`gofmt -l` 在本仓对**未改动**的文件也会报警，因此不能当门禁用 |

---

## 6. 存档兼容与 PG→SQLite 单向转换（最高优先级）

### 6.1 不变式（必须逐条证明）

1. 全表**主键集合与行数**在转换前后一致。
2. JSON 列的**原始字节**不变（含未知字段）——转换**只搬运不重编码**。
3. 身份不重编号：`accounts.id` / `characters.id` / `wire_id` / `fixed_slot` 与来源一致。
4. 邮件**共享号段**：消息与附件同号段、附件 id 先分配且 < 消息 id、投递高水位 `coalesce(max(id),0)` 语义保持
   （`mailbox.go:26-27,47-57,143-160`；断言见 `mail_events_postgres_test.go:83`）。

### 6.2 已识别的一处**可观察差异**（需业主知悉或决策）

PG 序列是**非事务性**的：`SendMail` 回滚也照样烧掉号；
而"单行计数器表在事务内自增"在回滚时会**退回**，于是号段**间隙**分布不同。
唯一性、单调性、附件/消息次序、高水位**都不受影响**，受影响的只是"回滚留下的空洞"。

- 选项 ①（建议）：登记为**接受的差异**，写进文档与契约测试注释；
- 选项 ②：若要求完全复现烧号行为，则在**独立 autocommit 语句**里分配 id（脱离主事务），代价是崩溃时可能多烧号。

### 6.3 转换工具（`dfo-tool sqliteconvert`，走既有 4 入口，不新增 cmd 目录）

| 步骤 | 内容 |
| --- | --- |
| 读 | PG **只读** DSN；逐表按主键稳定顺序读取 |
| 写 | 新建 SQLite 文件；JSON 列按原始字节写入；`timestamptz`→unix_micro；`bytea`→BLOB；数组列→JSON 文本；identity→`AUTOINCREMENT` 并把 `sqlite_sequence` 对齐到 PG 序列当前值 |
| 守卫 | 逐行校验 `uint64`（Cera 等）不超 int64 上限，越界即中止并报告（不得静默截断） |
| 校验 | ① 逐表行数 ② 主键集合 ③ 抽样行 JSON 字节 SHA256 ④ `dfo-tool charactercheck`（引擎中立契约套件）⑤ 54 域只读准备 |
| 反向 | **明确不做 SQLite→PG**，写进工具帮助与文档，避免出现第二真源 |

迁移台账沿用现有 checksum 语义（`migrations.go:54`：换行归一化后 SHA256），新增 `sql/sqlite/migrations/` 与同构台账表。

---

## 7. 测试与门禁

### 7.1 契约套件引擎化（S5）

**耦合规模（实测）**：依赖 `DFO_TEST_POSTGRES_DSN` 的 Go 文件 **44 个**（43 个测试 + `fixture.go`），
分布：`internal/database` 21、`cmd/wireprobe` 11、`internal/character` 6、`internal/workflow` 2、
`internal/quest`/`internal/loot`/`internal/cashshop`/`cmd/gmtool` 各 1。

隔离机制现状（`fixture.go:33-84`）：随机 schema 名 → `CREATE SCHEMA` → 夹具池设 `search_path` →
用 `current_schema()`（`fixtures.sql:2-3`）断言隔离真的生效 → 清理用 `DROP SCHEMA ... CASCADE`。
另有 **约 18 个 database 测试与 3 处 wireprobe 测试绕过夹具**、内联同一套 `CREATE SCHEMA`/`DROP SCHEMA`
（其中 `cmd/wireprobe/primer_transform_flow_test.go:37,41,245,249,358,362` 走 `DiagnosticExec`）。

引擎化改造清单：

| 项 | 现状 | SQLite 侧 |
| --- | --- | --- |
| `OpenTestFixture` | 强制 `DFO_TEST_POSTGRES_DSN`（`fixture.go:33-37`） | 按 `DFO_TEST_ENGINE` 分派；SQLite 用 `t.TempDir()` 下的一次性库文件 |
| `Reopen`/`Storage`/`Close` | `Close` = `DROP SCHEMA CASCADE`（`:71-84`） | 关闭并删除临时文件 |
| 隔离断言 | `FixtureSchema` = `current_schema()` | `PRAGMA database_list` / 文件路径校验 |
| `sqlcTestStore` | 自建 schema（`sqlc_postgres_test.go:20,32-39`） | 同文件库 |
| 内联 `CREATE/DROP SCHEMA`（约 18+3 处） | 直接 pgx DDL | 改为调用引擎无关夹具构造器 |
| `RejectGraduation` | 用 CHECK 约束注入失败（`fixture.go:143-156`，`fixtures/*.sql`） | 需 `sql/sqlite/fixtures/` 等价物（重建表 + CHECK，或 `PRAGMA ignore_check_constraints` 反向使用）——S4 独立子任务 |
| `commerce_postgres_test.go:20` | 唯一伸手进 pgx 内部读 `search_path` 的测试 | 需改写为引擎无关断言 |

**两引擎同批绿**是唯一验收判据；任一引擎特有的行为必须写进文档，不得用"跳过测试"掩盖。

### 7.2 新增架构守卫（S1 交付）

| 守卫 | 内容 | 现状 |
| --- | --- | --- |
| G1（新增） | `internal/workflow`、`internal/toolcmd/**`、`cmd/**` 的**非测试**代码中不得出现 `*database.Store` 作为字段/参数类型 | 现有 61 处引用、44 文件；S1 后应归零（测试文件除外） |
| **G2（必须先补）** | 把 `modernc.org/sqlite` 加入"只能出现在 `internal/database`（含子包）"的驱动名单 | `persistence_test.go:53` 目前只覆盖 pgx 与 `database/sql` ⇒ **静默缺口，见 §1.1** |
| **G3（必须先补）** | 把 `modernc.org/sqlite` 加入私有别名表，使 `persistence_test.go:88-104` 的"导出签名不得泄漏驱动类型"规则覆盖它 | 别名表 `:56-65` 目前只有 pgx/`database/sql`/`sqlcgen` |
| G4（新增） | 领域声明的接口由 `*database.Store` 满足的编译期断言 | ✅ **已实现（2026-10-04）**，并**实现更正**：`cmd/wireprobe` 是 `package main`，**无法**从 `internal/database` 反向断言，所以断言按**消费方分布**而非单点集中——`internal/workflow/store_contract_test.go`（11 个接口）、`internal/admin/store_contract_test.go`、`cmd/wireprobe/store_contract_test.go`。先例：`internal/database/character_contract_test.go:9-11`（`package database_test` + `var _ X = (*database.Store)(nil)`） |

**顺序要求**：G2/G3 **必须在任何 SQLite 代码落库之前**补上，否则"引擎实现只能在 `internal/database`"
这条保证会在第一次提交时就悄悄失效。

### 7.3 阶段门禁

每阶段（含每个 commit）：`go test -count=1 ./...` + `go vet ./...`；
S1/S2 的"行为零变化"判据 = **同一批 PG 集成测试在改动前后同样通过**；不覆盖业主工作区改动。

---

## 8. 阶段划分（S0–S5）

| 阶段 | 交付 | 门禁 | 可回滚性 |
| --- | --- | --- | --- |
| **S0 前置** | ① 安装 sqlc v1.31.1 ② `Generate-SQL.ps1` 多引擎改造 ③ §4.4 spike + A/B 判定报告 ④ **补 G2/G3 守卫缺口（§1.1）** ⑤ 修订 ADR-002 / `database-sqlc-migration.md` / 契约 §5 措辞 | spike 报告给出 A 或 B；生成门禁 `-Check` 通过；G2/G3 生效（用一条临时 SQLite 引用验证守卫会报错） | 纯文档 + 工具链 + 守卫，回滚无风险 |
| **S1 抽接口缝**（唯一真正的架构改造） | 以 **61 处引用 / 44 文件**为清单抽出**消费方声明**的窄接口（沿用 `character.Store` 范式，见附录 A），把 `internal/database` 之外的 43 个持有者改为接口，并收进 **30 个回调签名**与 4 处 `*database.Tx`；修掉 `booster_flow.go:246` 的唯一硬断言；新增 G1/G4 守卫（**排除测试文件与 `charactercheck`**，`TestFixture` 保持具体类型） | **行为零变化**：PG 集成测试同批通过；新守卫通过 | 按领域分 6–8 个独立 commit，逐个可回滚；**不动 SQL、不动方法体** |
| ✅ **S1 已完成（2026-10-04）** | 实际落地：`internal/admin`（`admin.Store`=1 方法）、`cmd/gmtool`（`gmStore`=11 方法）、`internal/workflow`（9 个窄接口）、**`cmd/wireprobe`（组合根单一 `persistentStore`，127 方法，理由见 §2.5）**；`booster_flow.go:246` 改为 `store.(persistentStore)`；G1 守卫 `TestConcreteStoreBoundary` + G4 断言（workflow 11 / admin / wireprobe） | **已证**：`go build ./...`=0、`go vet ./...`=0、`internal/archtest` 全绿；**全量 `go test -count=1 ./...` 失败集合与改动前基线逐名完全相同**（仍只有 2 个既有 `cmd/wireprobe` 失败，无新增） | 未 commit（等业主收口） |
| **S2 驱动类型移出公共 API（范围已缩小）** | 实测 **0 个导出方法**泄漏 pgx/pgtype/sqlcgen/pgxpool（§2.4）⇒ 本阶段缩小为：复核 C3/C4 覆盖、定稿 `*database.Tx` 的 3 个导出方法（保留 `CommitOdysseyHonorMail` 跑在外部 `*Tx` 上的唯一例外）、`storageError` 接纳 `sql.ErrNoRows` | 依赖守卫证明"领域侧零 pgx/database/sql/sqlite" | 小改动，独立 commit |
| ✅ **S2 已完成（2026-10-04）** | ① 抽出唯一的"无行"谓词 `isNoRows`（新文件 `internal/database/driver_errors.go`）：**42 处** `errors.Is(X, pgx.ErrNoRows)` 全部改走它，`storageError` 也改用它 ⇒ `pgx.ErrNoRows` 现在全仓只在该文件出现**一次**；顺带使 **18 个文件不再需要 `import pgx`**（此前只为止 ErrNoRows 而引入）。② `Tx` 导出面复核：仅 3 个方法（`CountShopPurchases`/`RecordShopPurchase`/`UnlockRosterBackground`），唯一例外 `CommitOdysseyHonorMail` 跑在外部 `*Tx` 上**保持不变**。③ 产出**引擎相关面清点**（§2.6）作为 S3/S4 的处理清单 | **已证**：`go build ./...`=0、`go vet ./...`=0、`internal/archtest` 全绿（含 G1 与"导出签名不得泄漏驱动类型"边界）；在隔离测试库 25439 上全量 `go test -count=1 ./...` 的**失败集合与改动前基线逐名相同**（仍只有 2 个既有 primer-transform 失败，无新增） | 未 commit |
| ✅ **S3a 已完成（2026-10-04）：SQLite schema 分叉铺满 56/56 表** | `sql/sqlite/migrations/0001_initial.sql` 覆盖全部 38 个段落（段落名与 PG 逐字一致，便于两边台账直接对照）：identity→`AUTOINCREMENT`（同时替掉全 schema 唯一的序列 `mailbox_id_seq`）、jsonb→`BLOB`+`CHECK(json_valid)`、`jsonb_typeof`→`json_type`、bytea→`BLOB`、`bigint[]`→JSON 数组 BLOB + `json_array_length`、boolean→`BOOLEAN` 存 0/1、timestamptz/date→整数微秒、`octet_length`→`length(CAST(x AS BLOB))`、`FOR UPDATE` 全删、`ALTER ... ADD COLUMN IF NOT EXISTS` 折进建表；DO 块与历史修复 DML **明确不搬**并写明理由 | **已证（真实执行，非推断）**：① `TestSQLiteSchemaCoversPostgres` —— 逐表逐列与 PG 对照，**56 表 / 7 索引完全一致**，无遗漏也无多余；② 真实 SQLite 执行整份 DDL —— **56 表建成、`foreign_key_check` 干净**；③ `TestSQLiteMigrationSectionsCoverFile` —— 显式段落列表与文件**零漂移**；④ `TestSQLDialectFilesAreASCII` —— 两个方言树 19 个 `.sql` **全 ASCII**（D23 由测试机械化执行，不再靠人记）；⑤ 端到端测试改为跑**全量**迁移（台账 37 行、建表数 == PG 表数）；⑥ `sqlc generate` 解析整份 schema exit 0；⑦ 全量 `go test` 失败集合与基线逐名相同 | 未 commit |
| 🔄 **S4a 已完成（2026-10-04）：SQLite 引擎接入 + 真实驱动端到端跑通** | 新增 `internal/database/sqlite.go`：① `sqliteDSN`（`_txlock=immediate` / `_busy_timeout` / `_foreign_keys=1` / `_journal_mode=WAL` / `_synchronous=NORMAL` / `_timezone=UTC` / `_inttotime=1` / `_time_integer_format=unix_micro` / `_dqs=0`；查询串**手工拼接**，否则 `url.Values` 会把 `journal_mode(WAL)` 的括号百分号编码）② `openSQLite`（有界连接池：默认无上限，且每个连接自带页缓存）③ 把 PG 的迁移段落切分**泛化为 `migrationSection(fsys, initial, name)`** 供两引擎共用，避免台账身份与校验和规则分叉 ④ `migrateSQLite`（段落 + sha256 + `storage_migrations` 台账，幂等；复用 S2 的 `isNoRows`）⑤ `migrateSQLiteAll`。依赖 `modernc.org/sqlite v1.59.0`（纯 Go，CGO 保持关闭；`libc v1.75.7` 与驱动 pin 一致） | **新增端到端测试 `TestSQLiteCoreSliceRoundTrip` 全绿**（真实驱动，非 mock）：`sqlite_version=3.53.4`；三条 PRAGMA 断言确实生效（`journal_mode=wal` / `foreign_keys=1` / `busy_timeout=5000`）；**外键真的拦住了孤儿角色**；迁移重复执行不报错且台账 2 行；角色 `state` **逐字节往返一致**且 `typeof='blob'`；`created_at` 存储类为 `integer` 且 `time.Time` 微秒往返正确；`deleted_at` 读出 `*time.Time`；事件复合主键**真的拒绝重放**；归档后名册隐藏、`CharacterAllocation` 计数与下一序号正确；**删档后 id 单调不重用**。本轮据此修正 D24 | 未 commit |
| **S4 SQLite 引擎 + 迁移 + 编排分叉** | `Open` 按 `driver` 分派；DSN/PRAGMA/pool/busy 重试（§5）；SQLite 迁移台账；`sqliteconvert` + 校验（§6.3）；fixture 引擎化（§7.1）；GM 互斥（§3.3）；**启动链分叉 + 打包分叉 + 受影响的 Python 测试（§5.3–5.4）**；诊断工具降级策略（§3.4） | 账号限购、角色存档、回执原子性、邮件号段四项专项证明；转换后 `charactercheck` 通过；`driver=sqlite` 可无 PG 启动 | 引擎分派默认仍 `postgres`，SQLite 可整体关闭 |
| **S5 分叉控制与收口** | 契约套件双跑；两引擎差异写入本文档；**实机由业主操作**（PG 默认路径回归 + SQLite 单机启动） | 两引擎全绿；业主实机确认 | 保留 PG 默认，随时回退 |

**纪律（新增，建议写入 `server/AGENTS.md`）**：S4 之后**任何新增/修改 SQL 必须同轮补齐两方言**，
否则不允许合并；这是双引擎长期不漂移的唯一保障。

---

## 9. 风险登记

| # | 风险 | 影响 | 缓解 / 判据 |
| --- | --- | --- | --- |
| R1 | 锁与幂等语义**无法机械等价**（D1/D12/§3.3） | 存档损坏、重复发放、限购失效 | 保留锁顺序；`BEGIN IMMEDIATE` 单写者；四项专项证明（S4 门禁） |
| R2 | JSON 字节等价（D4） | 存档哈希/未知字段漂移 | 禁止库侧规范化；写前 Go 统一 marshal；转换按原始字节；抽样 SHA256 校验 |
| R3 | sqlc sqlite 解析覆盖不足 / override 失效（§4.3） | S3 返工 | S0 spike 提前暴露；B 路径兜底 |
| R4 | `modernc.org/libc` 版本脆弱 | 编译/运行异常 | go.mod 锁同版本；禁止随手升级；`go mod verify` |
| R5 | 时间语义（§5.1） | 日/周边界、过期判定错 | 整数微秒对齐 PG 微秒；边界计算留在 Go；契约测试覆盖 |
| R6 | 性能（1.3–2.0x CPU） | 单机启动/落库变慢 | 单机可接受；建索引；`EXPLAIN QUERY PLAN` 复核 |
| R7 | 诊断/运维工具 PG 专用（§3.4） | 运维误判 | 明确拒绝 + 可诊断错误；`repairs/` 保持 PG 专用 |
| R8 | 双引擎长期维护成本 | 两方言逐渐漂移 | §8 纪律 + 双跑契约套件 + `-Check` 生成门禁 |
| R9 | 迁移**单向不可逆**（§6.3） | 不能回退到 PG | PG 库在转换期保持只读不动；SQLite 是**新增**产物，PG 仍是默认引擎 |
| R10 | fixture 语义差异（§7.1） | 测试假绿 | `RejectGraduation` 等注入手段逐项在 SQLite 侧重建 |
| R11 | **守卫缺口**：`modernc.org/sqlite` 未被禁用驱动名单与私有别名表覆盖（§1.1） | 分层保证静默失效，问题在几十个文件之后才暴露 | S0 先补 G2/G3，并用一条临时引用验证守卫确实报错 |
| R12 | 配置面 / 编排面变更打破既有 Python 测试与发布包（§5.3–5.4） | 启动链回归、发布包仍带 919.8 MB PG | 同轮更新 `server/work/dfo-lan/scripts/test_postgres_storage.py`、`scripts/test_environment_storage.py` 与 `build-publish.ps1` |
| R13 | 生成门禁**不在 CI 默认路径**（需显式 `-CheckSQL`，§5.5） | 双引擎 SQL 与生成物静默漂移 | S3 后把 `-CheckSQL` 纳入 CI |

---

## 10. 工作量与优先序

| 阶段 | 估计（工程日） | 说明 |
| --- | --- | --- |
| S0 | 0.5–1 | 含 sqlc 安装、`Generate-SQL.ps1` 改造、spike、守卫缺口修补 |
| S1 | 2–4 | 43 个包外持有者改接口，按领域分 6–8 个 commit |
| S2 | 0.5 | 大部分已被现有守卫保证 |
| S3 | 3–6 | 17 个 PG SQL 文件 ↔ SQLite 镜像；取决于 A/B |
| S4 | 3.5–6 | 引擎 + 转换工具 + fixture + GM 互斥 + 启动链/打包分叉与 Python 测试 |
| S5 | 2–3 | 双跑 + 文档 + 实机 |
| **合计** | **11.5–20.5** | 不含实机等待与业主回归时间 |

**优先序**：S0 → S1 → S2 → S3 → S4 → S5。
**硬规则**：**S1 未收口前不得开始写 SQLite SQL**（否则同时改两层、回归无法定位），这一条沿用上轮交接口径。

**本轮明确不做**：MOD 四层（另行 P0–P6）、Redis 残留清理（可独立低风险提交）、Python 编排 Go 化。

---

## 11. 待业主确认的剩余小项（不阻塞 S0/S1）

1. **邮件号段间隙差异**是否接受（§6.2 选项 ①/②）。
2. **GM 互斥**用租约行（方案 A，建议）还是锁文件（方案 B）。
3. **`dbq` / `DiagnosticQuery`** 在 SQLite 下是"完全拒绝"还是"实现 `sqlite_master` 子集"。
4. SQLite 数据库文件的**默认路径与命名**（建议 `runtime/storage/dfo_lan.sqlite3`），以及是否随发布包分发。
5. 是否把 §8 的"改 SQL 必须双写"纪律正式写入 `server/AGENTS.md`。
6. 是否把 `sqlc -CheckSQL` 生成门禁纳入 CI（当前仅在手动 `-BuildServer -CheckSQL` 时执行，§5.5）。
7. `TestFixture` 保持具体类型（§2.4 建议）是否接受——它是测试/`charactercheck` 专用，接口化收益低、改动面大。

---

## 附录 A：S1 契约清单（接口名 → 方法 → 调用点）

完整逐方法清单（每个方法的源文件 `file:line` + 包外调用点数）见配套取证文档
**`s1-store-interface-extraction-input.md`（2026-10-05 已删除）**（459 行）。本附录给摘要与结论。

**规模**：202 个导出方法 / 61 个非测试文件（58 个含方法）/ 8 个未导出方法 / 3 个 `*Tx` 导出方法。
其中 **165 个**归入 §A.1 的 35 个候选接口，**37 个**是迁移与 DDL 面（§A.2）。

### A.1 候选接口分组（按包外调用密度排序摘录）

| 建议接口 | 方法数 | 高密度方法（包外调用点数） |
| --- | --- | --- |
| `CharacterEventStore` | 6 | `CommitCharacterEvent`(87)、`CharacterEventReceipt`(32)、`CommitCharacterPremiumEvent`(7) |
| `AccountCharacterStore` | 13 | `DevelopmentAccount`(34)、`Characters`(33)、`CreateCharacter`(27)、`DeleteCharacter`(5) |
| `QuestStore` | 13 | `Quests`(23)、`CompleteQuestObjective`(7)、`AcceptQuest`(5)、`AbandonQuest`(5) |
| `UnifiedOptionStore` | 13 | `AccountUnifiedOptions`(3)、`CharacterUnifiedOptions`(2)、`AccountHotkeys`(2)、`CharacterHotkeys`(2) |
| `SkinStore` | 10 | `ListSkins`(8)、`SelectSkin`(3)、`SkinFavorites`(3) |
| `GamepadStore` | 9 | `ResolveGamepadPayload`(2)、`ClearAccountCharacterGamepadSettings`(2) |
| `VaultStore` | 8 | `LoadVault`(6)、`CommitAccountVault`(6) |
| `CashPurchaseStore` | 7 | `PurchaseCashToBag`(5) |
| `AdventureStore` | 7 | `PrepareAdventure`(7)、`CommitAdventure`(4) |
| `PremiumStore` | 6 | `HasActivePremium`(4)、`HasGrowthPremium`(4) |
| `MailStore` | 6 | `SendMail`(2)、`MutateMailbox`(2) |
| `OathStore` | 6 | `EquippedOathSelection`(6) |
| `TowerStore` | 5 | `ReadTowerProgress`/`ReserveTowerEntry`/`AdvanceTowerFloor`/`TowerGriefProgress`(各 1) |
| `FatigueStore` | 4 | `ConsumeRoomFatigue`(12)、`LoadFatigue`(5) |
| `AccountMaterialStore` | 4 | `AccountMaterials`(10)、`CommitAccountMaterialEvent`(5) |
| `BirthStore` | 3 | `BirthStage`(8)、`AdvanceBirth`(6)、`StartBirth`(4) |
| 其余 19 个接口 | 各 1–3 | `WorldStore`(3)、`BleedingMineStore`(3)、`BlackPurgatoryStore`(3)、`OmenStore`(3)、`GMMailStore`(3)、`GrantStore`(3)、`RosterBackgroundStore`(2)、`TutorialStore`(2)、`FavorStore`(2)、`SkillLockStore`(2)、`IspinsStore`(2)、`WarpFavoriteStore`(2)、`OdysseyStore`(2)、`ProfileSkinStore`(1)、`ShopPurchaseStore`(1)、`FameStore`(1)、`MoonRewardStore`(1)、`ProgressionReadStore`(2)、`StoryDigestStore`(1，**零调用死导出**) |

**声明位置由消费方决定（R2 强制）**：上表是**逻辑分组**，物理上接口必须声明在**需要它的那个消费包里**
（`internal/workflow` 声明自己用到的那部分、`cmd/wireprobe` 声明自己的、`internal/{character,quest,world}` 已有的沿用），
由 `cmd/wireprobe/bootstrap.go` 注入具体 `*database.Store`——这正是仓库既有范式（附录 A.3）。

### A.2 保持具体类型的面（不进领域接口）

`Migrate` + 36 个 `Migrate*`/DDL 方法（`InitializeGame`、`UpgradeSecondaryVaultCapacity`、`MigrateSaveIdentity` 等）、
`DiagnosticQuery`/`DiagnosticExec`（仅 `dbq`）、以及 `Open`/`Close`/`LoadConfig` 生命周期面。
消费者只有 `cmd/wireprobe/bootstrap.go` 与 `internal/toolcmd/**`、`cmd/admin`、`cmd/gmtool`，
属于 L4 组合/工具层，**允许**持具体类型（R6）。

### A.3 既有范式（S1 直接复用，不另发明）

仓库已有 **13 个**由 `*database.Store` 满足的窄接口，惯例是"消费方声明小接口 + bootstrap 注入具体实现"：

`character.EventStore`/`Store`/`ProgressionStore`/`FatigueStore`（`internal/character/store.go:11,18,32,40`，
编译期断言在 `internal/database/character_contract_test.go:9-11`）、`quest.Store`（`internal/quest/store.go:26`，8 方法）、
`world.Store`（`internal/world/service.go:19`，1 方法，有仓库内 fake）、`workflow.VaultLedger`、`workflow.equipmentEventStore`、
`wireprobe.boosterEventStore`、`inventory.PremiumStore`（经适配器 `workflow.PremiumReader`）、
`character.odysseyHonorStore`，以及 2 处内联匿名接口。

⇒ S1 的动作是**照此扩展**，而不是引入新的抽象层或共享 model 包。

## 附录 B：SQL 方言普查

- 统计对象：`server/work/dfo-lan/internal/database/sql/postgres/**`（17 文件 93,214 字节）。
- 词频统计见 §3.1（本次直接统计口径）。
- 逐项重写清单见 §3.2 D1–D21。
- 按查询难度分档（281 条具名查询）：**约 168 条**可直接移植（纯文本/整数 CRUD）；
  **约 60 条**改类型/转型即可；**51 条**需要语义重写（27 条锁查询、4 条邮件编号、4 条 jsonb 运算符、
  4 条 `unnest`/`ANY`、2 条 advisory/identity、`repairs/` 整文件等）；**2 条 + 1 文件**为 PG 专用诊断。

## 附录 C：PG 专用清单

见 §3.4。另：`sql/postgres/repairs/dark_knight_combo_slots_v1.sql` 整体保持 PG 专用；
`fixtures/*.sql`（`allow_graduation` / `reject_graduation`）需要在 SQLite 侧重建等价注入手段。

## 附录 D：证据索引

**仓库内（只读取证）**
- 契约与守卫：`internal/archtest/contract_test.go:38,152`（R2）、`internal/archtest/persistence_test.go:50-150`（C2–C5）
- Store 形状：`internal/database/store.go:16-36,41-46`；`internal/database/tx.go:12-21`
- 领域接口范式：`internal/character/store.go:11-46`；断言 `internal/database/character_contract_test.go:9-11`
- 迁移运行器：`internal/database/migrations.go:20-112`（含 checksum `:54`、`InitializeGame:99-111`）
- 测试夹具：`internal/database/fixture.go:33-84,143-156`
- 生成门禁：`scripts/Generate-SQL.ps1:6-9,20-22`；`server/Build-Server.ps1:7-9`；`sqlc.yaml:1-26`
- 架构文档：`docs/architecture.md:22-29`（ADR-002）、`docs/architecture-contract.md:38-51,144-167`、
  `docs/database-sqlc-migration.md（2026-10-05 已删除）:69-83,260`

**外部权威（本轮实测）**
- [sqlc v1.31.1 配置文档](https://docs.sqlc.dev/en/v1.31.1/reference/config.html)（`engine: sqlite`、
  `emit_pointers_for_null_types` 支持 SQLite、全局 overrides 的 `engine` 字段）
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)（纯 Go 无 CGO、各平台内置 SQLite 3.53.4、
  连接态不重置、`SetMaxOpenConns` 必须有界、CPU 1.3–2.0x、libc 版本必须一致）
- 驱动源码（本机缓存 `modernc.org/sqlite@v1.59.0`）：`driver.go:93-219`（DSN 参数与校验顺序）、
  `sqlite.go:385-391`（`_txlock` 取值校验）
- [sqlc#1985](https://github.com/sqlc-dev/sqlc/issues/1985)（SQLite 引擎的列 override 存疑）
