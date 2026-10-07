# SQLite 查询分叉移植指南

本文档是**手写 SQLite 查询树**的操作规范。目标树：`internal/database/sql/sqlite/queries/`，
源树（只读参考）：`internal/database/sql/postgres/queries/`。目标 schema：
`internal/database/sql/sqlite/migrations/0001_initial.sql`（**先读它确认列类型**）。
已完成的范例：`sqlite/queries/core.sql`（文件头部逐条写明规则，遇到新情况先看它）。

## 0. 铁律

1. **纯 ASCII**。任何非 ASCII 字节（包括注释里的中文、`§`、全角标点）都会让 sqlc v1.31.1 的
   sqlite 引擎把语句解析成乱码，报出 `extraneous input ')'`／`'d'`／`'t'` 这类**单字符**错误，
   指向完全无关的位置。这是最难排查的一类失败（见设计文档 D23）。
2. **保持 `-- name: X :one|:many|:exec` 标记与语义完全不变**，查询名与 PostgreSQL 侧逐字一致；
   生成的方法名要靠它对齐。
3. **不改** `sqlc.yaml`、`internal/database/sqlcgen/`、`internal/database/sqlcgensqlite/`，
   也不碰别人的查询文件。
4. **PG 侧文件只读**，永不修改。

## 1. 类型与表达式映射

| PostgreSQL | SQLite | 说明 |
| --- | --- | --- |
| `FOR UPDATE` / `FOR NO KEY UPDATE` / `FOR SHARE` | **整句删除** | SQLite 无行锁；写序列化由 `BEGIN IMMEDIATE` 全局承担。锁查询保留名字与归属判定 |
| `sqlc.arg(x)::text` | `sqlc.arg(x)` | 若 sqlc 推不出类型再 `CAST(sqlc.arg(x) AS TEXT)` |
| 表达式内的数值参数 | `CAST(sqlc.arg(x) AS INTEGER)` | **规则 R-1**：裸表达式会退化成 `interface{}`，进不了统一接口 |
| **JSON 参数** | **绝不加 CAST** | 加了会让参数类型变 `string`，而 `database/sql` 无法把 `string` 扫进 `json.RawMessage`（D24） |
| 表达式结果列 | `CAST(<expr> AS <type>) AS <alias>` | 连多余的括号都会让 sqlc 推不出类型，必须显式 CAST |
| `x -> 'k'` / `x ->> 'k'` | `json_extract(x,'$.k')` | |
| `jsonb_set(x,'{k}',v,true)` | `json_set(x,'$.k',v)` | 需要 JSON 值时用 `json(...)` 包一层 |
| `jsonb_typeof(x)` | `json_type(x)` | |
| `jsonb_build_object('a',b)` | `json_object('a',b)` | |
| `jsonb_build_array(...)` | `json_array(...)` | |
| `x \|\| y`（jsonb 拼接） | `json_patch(x,y)` | 数组拼接语义不同，需逐条判断 |
| `to_jsonb(row)` | 无直接对应 | 通常应改由 Go 侧 marshal；不要硬凑 |
| `GREATEST(a,b)` / `LEAST(a,b)` | `max(a,b)` / `min(a,b)` | 标量形式 |
| `now()` | `(CAST((julianday('now') - 2440587.5) * 86400000000 AS INTEGER))` | 整数微秒，见 `core.sql` 的 `ArchiveCharacter` |
| `date` / `interval` 运算 | 整数微秒算术 | 语义差见 D29；能用 Go 侧算就不要写进 SQL |
| `x = ANY(sqlc.arg(ids))` | `x IN (SELECT value FROM json_each(CAST(sqlc.arg(ids) AS TEXT)))` | 参数按 JSON 数组文本传入 |
| `sqlc.narg(x)::bigint[] IS NULL OR …` | `CAST(sqlc.narg(x) AS TEXT) IS NULL OR …` | **nil 必须仍然是 SQL NULL**：适配器不得把 nil 序列 marshal 成 `"null"` 文本，否则 `IS NULL` 不成立、`json_each('null')` 匹配 0 行，「不过滤」被静默改成「读 0 行」。空集合（非 nil）仍 marshal 成 `[]`＝匹配 0 行，与 PG 的 `id=ANY('{}')` 一致。实测事故：SQLite 档领取邮件附件全部被拒 `errMailMissing`（`LockMailbox`） |
| `unnest(...)` | `json_each(...)` | |
| `array_agg(x)` | `json_group_array(x)` | 返回类型变 JSON |
| `array_length(a,1)` | `json_array_length(a)` | |
| `octet_length(x)` | `length(CAST(x AS BLOB))` | 保留字节语义（`length()` 对 TEXT 数字符） |
| `LIMIT n OFFSET m` | `LIMIT n OFFSET m`（**顺序不能反**） | PG 两种顺序都行，SQLite 只认 LIMIT 在前（D22） |
| `IS DISTINCT FROM` | 原样保留 | 驱动自带 SQLite 3.53.4，支持 |
| `ON CONFLICT ... DO UPDATE ... WHERE ... RETURNING` | 原样保留 | 已验证可用 |
| `FILTER (WHERE ...)` | 原样保留 | 已验证可用 |
| `GENERATED ALWAYS AS IDENTITY` / `nextval` | 见 schema，查询里通常无需处理 | |
| `current_database()` / `pg_try_advisory_lock*` / `pg_advisory_*` | **引擎专属，不要移植** | 在目标文件里用注释说明「PG-only，暂不移植」并回报 |

## 2. 自检方法（必须做，且只能在自己目录里做）

在 `.tmp/parity-<你的标识>/` 下建自己的验证环境，**不要用仓库根的 `sqlc.yaml`**：

```
.tmp/parity-<id>/sqlc.yaml      # schema 指向 ../../internal/database/sql/sqlite/migrations
                                # queries 指向 "q"（你自己的目录）
                                # out 指向 "out"
.tmp/parity-<id>/q/<file>.sql   # 你正在移植的文件
```

`sqlc.yaml` 的 `overrides` 必须**从仓库根 `sqlc.yaml` 原样复制**（TIMESTAMP→time.Time、
可空指针、`characters.state`/`character_events.outcome`→json.RawMessage 等）。
然后：

```powershell
C:\Users\Ricar\go\bin\sqlc.exe generate -f .tmp\parity-<id>\sqlc.yaml
```

**退出码 0 才算通过**。若报错，先看是不是非 ASCII（铁律 1）或漏了 CAST（R-1）。
通过后把文件复制到 `internal/database/sql/sqlite/queries/<同名文件>`。

注意：`queries` 只指向你自己的目录，这样多个人并行移植时不会互相干扰。

## 3. 回报格式

- 已写入的目标文件路径。
- 移植了多少条查询；若有**未移植**的，逐条列出查询名 + 原因 + PostgreSQL 侧用到的专属功能。
- 任何你不得不偏离上表的地方，说明理由。
- 明确说明：你没有修改 `sqlc.yaml`、`sqlcgen/`、`sqlcgensqlite/` 或别人的文件。


## 4. 宽度与类型对齐（由中心门禁统一收口，移植者不必手工调）

目标 schema 的整数列**已按 PostgreSQL 声明镜像宽度名**（`BIGINT`/`INTEGER`/`SMALLINT`），
`sqlc.yaml` 据此把它们映射回 int16/int32/int64。因此：

- **能被列类型推断的参数不要加 CAST**（例如与列比较、或赋值给列）。加了 CAST 会被强制成 int64，
  反而与 PostgreSQL 不一致。
- 只有**表达式位置**才需要 CAST（否则类型退化成 `interface{}`）。这类位置的宽度差异由中心门禁的
  `acceptedDivergences` 统一登记，并各对应一个手写适配器方法，**不需要你在 SQL 里绕开**。
- **不要为了迁就类型而改变查询语义**。语义第一，类型对齐由门禁收口。
- **可空 JSON 数组参数（`[]int64` → `*string`）的 nil 必须原样保留**：适配器对这类字段只应在
  非 nil 时 marshal 并对空指针赋值（`if v.X != nil { … }`），产物是
  `internal/database/sqlite_adapter_gen.go`（生成器 `.tmp/gen-sqlite-adapter.go` 是**未入库**的
  本地脚手架，已同步该规则；重新生成前请先确认它不会把 nil 写回 `"null"`）。
  `LockMailbox.MessageIds` 是当前唯一的这种位置，回归测试见
  `internal/database/sqlite_mailbox_claim_test.go`。

## 5. sqlc v1.31.1 sqlite 引擎的已知陷阱（实测，务必遵守）

以下每一条都经过真实 sqlc + 真实驱动（modernc sqlite 3.53.4）复现。前两条最危险：
**sqlc 会 exit 0 但产出坏 SQL**，工具链其他环节都不会报警。

| 陷阱 | 症状 | 正确写法 |
| --- | --- | --- |
| `sqlc.arg()` 出现在 `FILTER (WHERE ...)` 内 | **静默**：exit 0，但生成物里留下字面量、既无占位符也无 Go 参数，运行时报 "no such function" | 改写为 `count(CASE WHEN <条件> THEN 1 END)`（语义等价）；`FILTER` 内**没有参数**时是安全的 |
| `sqlc.arg()` 出现在 `ON CONFLICT ... DO UPDATE` 的 SET 或 WHERE 内 | **静默**：整个子句被原样拷贝 | 把参数挪到 `INSERT ... SELECT ... WHERE NOT EXISTS(...)`，使 `DO UPDATE` 完全无参数（行数语义可逐例对照 PG 验证） |
| `IS DISTINCT FROM` | exit 1：`extraneous input 'FROM'` | 用 SQLite 的空安全 `IS NOT`（两列都 NOT NULL 时 `<>` 亦可） |
| `UPDATE ... FROM` / `DELETE ... USING` | exit 1：`extraneous input 'FROM'` / `'USING'` | 改写为同 join、同归属判定的相关 `EXISTS` 子查询；**外层列必须加表名前缀**（`characters` 也持有 `character_id`/`config_version`，否则 sqlc 报歧义） |
| `INSERT ... SELECT ... FROM json_each(...) j ON CONFLICT ...` | 运行时 `near "DO": syntax error`（SQLite 把 `ON` 当 join 约束解析） | 在 `ON CONFLICT` 前插入 `WHERE 1=1`（等价、无副作用） |
| `CAST(sqlc.arg(x) AS JSON)` | **值被静默摧毁**：SQLite 施加 NUMERIC 亲和性，`[1,2]`→0、`{"a":1}`→0 | 用 `json(CAST(sqlc.arg(x) AS BLOB))`（参数变 []byte，值原样保留） |
| JSON1 函数结果写入 BLOB 列 | JSON1 返回 TEXT，TEXT 值无法扫进 `json.RawMessage`（D24） | 结果外套 `CAST(json_set(...) AS BLOB)` |
| `RETURNING <别名>` | 别名不生效，字段名变成 `ColumnN` | 保留取值语义并上报，**不要为改字段名而改查询语义** |

**唯一可靠的自检**：生成之后，在你生成的 `*.sql.go` 里搜索 `sqlc.arg(`，
**排除以 `--` 开头的注释行**（生成物会内嵌查询文档注释，注释里提到这些陷阱是正常的）；
非注释行必须为 0 处。中心门禁已把这条检查固化为测试。
