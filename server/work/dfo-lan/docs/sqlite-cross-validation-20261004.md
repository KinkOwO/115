# 交叉验证报告：本仓库实现 ↔ `115US-SQLite-源码测试升级包`（20261004）

对象：`C:\Users\Ricar\Downloads\115US-SQLite-源码测试升级包`
（内含 `server/sqlite-source/20261004/` 的完整源码快照、`migrate_sqlite.py`、`build_sqlite.py`、
`stop_environment.py`、`更新SQLite.cmd`、`编译SQLite.cmd`）

验证方式：**比结果而不是比文字**——把两边的 `sql/sqlite/migrations/0001_initial.sql`
各自在一个全新的 SQLite 库里执行，再比较生成的目录（表、列、声明类型、索引），
并逐项核对两边代码实际读写的配置键与存储约定。

## 1. 结论摘要

| 维度 | 结果 |
| --- | --- |
| 表集合 | **一致**（本仓库 56 张，对方 57 张；对方多一张 `mailbox_id_sequence`） |
| 列集合 | **完全一致**：56 张共有表**没有任何列差异** |
| 索引 | **一致**（各 7 个显式索引，无差异） |
| 查询树 | **两边都做了 fork**：同为 13 个文件、文件名一一对应（`sql/sqlite/queries/`） |
| JSON 列 Go 侧类型 | **一致**：两边都用 `[]byte`（对方经 `db_type: JSONTEXT` override） |
| 驱动 | **一致**：都用 `modernc.org/sqlite` |
| 声明类型 | 206 处不同（`BIGINT/SMALLINT/TIMESTAMP` vs `INTEGER`、`BLOB` vs `JSONTEXT`）——见 §3，**实践上等价** |
| 引擎选择契约 | **原本不兼容**（对方靠"存在 `sqlite_path`"，本仓库靠 `driver`）→ **本轮已修** |
| 存档身份标记 | **原本不对称**（对方要求 `PRAGMA application_id=1152026104`，本仓库不写）→ **本轮已修** |
| 邮件号段 | **原本会冲突**（对方用 `mailbox_id_sequence`，本仓库用 `sqlite_sequence`）→ **本轮已修**（对齐） |

一句话：**两套实现在"数据长什么样"上高度一致，在"怎么被选中、怎么标记、怎么发行 id"上原本不兼容；
后者三处已在本轮修掉并加了测试。**

## 2. 本轮为互通所做的修改（均带测试）

| 修改 | 位置 | 为什么 |
| --- | --- | --- |
| 空 `driver` + 有 `sqlite_path` ⇒ 选 SQLite | `internal/database/store.go` 的 `Open` | 对方的迁移工具写出的 `local.json` **没有 `driver` 字段**，原实现会把它当 PostgreSQL，直接连错库 |
| 接受 `busy_timeout_ms` / `max_read_connections` | 同上，`Config` | 对方用这两个键名；本仓库原用 `sqlite_busy_timeout_ms` / `max_connections`。两边任一键名都能驱动同一份配置 |
| 建库时写 `PRAGMA application_id = 1152026104` | `sqlite.go` 的 `stampSQLiteSaveIdentity` | 对方打开既有库时会校验该标记，否则报"file is not a DFO SQLite save"。打上之后**本实现建的库对方能认**；反向本来就没问题（本仓库不校验该标记） |
| `NextMailID` 同步对齐对方的计数器 | `sqlite_adapter_manual.go` 的 `alignMailSequenceTable` | 见 §4，这是唯一会导致**存档数据出错**的分歧 |

测试：`internal/database/sqlite_interop_test.go`、`sqlite_mailseq_interop_test.go`。

## 3. 声明类型的 206 处不同为什么不影响互通

SQLite 的声明类型只决定 **affinity**：

- `BIGINT` / `SMALLINT` / `TIMESTAMP` / `INTEGER` 全是 **INTEGER affinity** ⇒ 行为完全相同；
- JSON 列：本仓库声明 `BLOB`（BLOB affinity），对方声明 `JSONTEXT`（含 `TEXT` ⇒ TEXT affinity）。
  **但两边 Go 侧都绑定 `[]byte`**，SQLite 对 BLOB 值不做转换 ⇒ **实际存储的字节相同**。
  文件名/路径由配置决定（本仓库验收包用 `dfolan.sqlite3`，对方固定 `game.db`），不构成冲突。

也就是说：**同一个库存放的数据两边读得懂**；差异停留在 DDL 文本层面。

## 4. 唯一会损坏存档的分歧（已修）

- 对方：`mailbox_id_sequence(name,value)` 表 + `UPDATE ... value=value+1 ... RETURNING value-1`（等价 PG `nextval`）。
- 本仓库：推进 SQLite 自身的 `sqlite_sequence`（`character_mail` 的高水位）。
- **风险**：在对方迁移出来的库上交替运行两套实现时，**各自只推进自己的计数器**，于是可能**把同一个邮件 id 发两次**。
  这不是显示问题，是存档损坏。
- **修法**：本仓库在发号后，若库里存在对方的表，就把它的 `value` 抬到不低于我们刚发出的 id+1，
  且**只前推不回退**（对方可能已经领先）。没有该表时是空操作，因此对本仓库自建的库零影响。

## 5. 尚未对齐 / 需要你决定

| 项 | 说明 |
| --- | --- |
| 建库语义 | 对方 **Open 从不建库**（另设 `InitializeSQLite` 显式创建），并要求既有库非空、带 app id；本仓库的 `Open` **自动创建并迁移**。两者对"误指向空路径"的容错倾向相反；本轮未改（改了会破坏本仓库的验收流程与既有测试） |
| 库身份校验 | 本仓库**不校验** app id（兼容旧库）；对方**强制校验**。若你希望本仓库也强制，需要先给旧库补标记的迁移步骤 |
| 邮件号段机制 | 本仓库仍以自己的 `sqlite_sequence` 为准（对齐只是不让对方落后）。若你更认对方那张显式表，可以改成本仓库也以它为准——但那样两边的"权威计数器"才真正统一 |
| 迁移工具 | 对方用 Python `migrate_sqlite.py`（约 29.7 KB，含备份/校验/切换 `local.json`）；本仓库用 Go `dfo-tool sqliteconvert`（含行数/JSON 字节/外键校验）。两者功能重叠但都不是对方的子集；**同一份存档只应被其中一个迁移** |
| `storage_format_version` | 对方会校验该键（0 或 2）；本仓库目前不读它。未改，因为本仓库没有对应的版本语义 |

## 6. 复现方式

```powershell
# 用「结果」比 schema：把两边的 0001_initial.sql 各建一个库，比表/列/索引
py -3 server\work\dfo-lan\.tmp\xval-schema2.py

# 互通契约（本轮新增）
go test -count=1 -run 'TestSQLiteInterop|TestSQLiteOpensASaveWithout|TestMailIDs' ./internal/database/
```

