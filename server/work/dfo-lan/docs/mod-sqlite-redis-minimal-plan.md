# MOD 接入 + SQLite 切换 + 去 Redis + 环境依赖最小化：实施计划与交接（2026-10-04）

> 本文是**给新会话的唯一接手入口**（取代并删除先前的 `mod-layered-interfaces-plan.md`
> 与 `database-dual-engine-plan.md` 两份分头计划）。
>
> **业主口径（2026-10-04，最终）**：只做四件事 ——
> **① MOD 接入（四层接口从 0 开始）② 切换 SQLite ③ 移除 Redis ④ 把代码环境依赖降到最低**。
> 明确**不再考虑**：历史图鉴数据对账、MR 创建（已放弃）。
>
> 本轮（2026-10-04）状态：**只做取证与计划，未改任何 Go 代码/PVF/schema/存档**；实施全部移交新会话。

---

## 0. 前置硬约束（不可协商，来自根 AGENTS）

1. **PVF 是内容唯一真源**（根 §0.2）：mod 只能通过注册面声明内容，**不得**引入第二份 JSON/DB 内容表。
2. **服务端不改写 PVF**（ADR-003）：覆盖只能是内存视图 + 来源哈希，落盘由内容工具负责。
3. **存档兼容最高优先级**（根 §0.4）：换引擎/加 mod 都必须让既有角色存档无损；必须给平滑迁移。
4. **不猜包**（根 §0.3）：新 opcode / 字段布局必须有 IDA 或实机证据，否则只登记缺口。
5. **实机由业主操作**（根 §0.6）：任何会话不得无人值守启动客户端或玩家库。
6. **DLL 日志写自身模块目录**（根 §0.9）：L4 的 ABI 里就要体现，不依赖进程 CWD。
7. 玩家库端口 **25438**、测试库 **25439** 隔离；不得对玩家库执行清理 SQL。

---

## 1. 现状基线（2026-10-04 取证，全部只读）

### 1.1 运行时到底依赖什么

| 依赖 | 事实 | 是否必需 |
| --- | --- | --- |
| **Go 服务端二进制** | `server/work/dfo-lan/bin/wireprobe-handoff-source.exe`（源码候选）/ `wireprobe-pvf.exe`（默认） | **必需** |
| **PVF 内层归档** | `server/work/client-build/Script.inner.pvf`（工作区实际存在；`configs/pvf-default.json` 指向 `../client-build/Script.inner.pvf`） | **必需**（内容真源） |
| **PostgreSQL 16.4** | 便携版在 `tools/pg`（919.8 MB）；`runtime/storage/local.json` 带 DSN + `postgres_bin` + `postgres_data`；实例端口 25438；`pg_ctl.exe` 由启动脚本拉起 | **当前必需 → 目标：SQLite 后一并去掉** |
| **Python 3.11.9** | 便携版在 `tools/python`（288.7 MB）；`server/work/dfo-lan/scripts/` 下 **19 个 .py**（`launch_local.py` 10911 B、`bootstrap_local.py`、`ensure_inner_pvf.py`、`prepare_inner_pvf.py`、`profile_pvf_startup.py`、`repair_profile.py`、`pvf_archive.py`、`pvf_rule_fields.py`、`reinforcement_pvf_reader.py`、`export_*.py` ×3、`audit_config_references.py` + 5 个 `test_*.py`） | **当前必需（编排/启动/探针）→ 目标：Go 化后可选** |
| **channel_probe（Python + probe.exe）** | `server/work/dfo_probe_tools/channel_probe.py` + `probe.exe`（启动链里必需项） | **当前必需** |
| **客户端目录** | `server/launcher.local.json` → `client_dir: C:/Game/dof/115us/DFO`（仓库外）；启动链检查 `DFO.exe` / `Script.pvf` / `sk.dat` | 玩家侧必需（**不在本仓**） |
| **Redis** | **运行时零引用**（见 §4）；但 `tools/redis` 仍占用 **46.1 MB** | **不需要 → 待删** |
| 便携工具链其余 | `tools/go` 224 MB、`tools/gm-tool` 125 MB、`tools/gocache` 6.4 GB、`tools/gopath` 1.96 GB（后两者是构建缓存，非运行依赖） | 开发侧 |

**启动链实际动作**（`scripts/launch_local.py`）：`pg_ctl.exe` 起 PostgreSQL → 起 `channel_probe.py`（配 `probe.exe`）→ 起服务端二进制。仓库根有 9 个 `.cmd` 入口（`启动游戏.cmd` / `启动服务端.cmd` / `停止游戏环境.cmd` / `配置环境.cmd` / 伊斯系列 / 一键打包）。

### 1.2 `.gitignore` 已忽略的大件（说明"仓库拿到什么"≠"运行需要什么"）

`/tools/`、`/server/work/dfo-lan/runtime/*`、`runtime/storage/pgdata/`、`runtime/roles_*/`、
`configs/{server,launcher}.local.json`、`/server/runtime/`、`.tmp/`（还有 `server/work/client-build/` 未跟踪）。

⇒ 目标环境的"最低"应定义为：**除客户端目录与 PVF 外，不再需要任何外部进程（无 DB 服务、无 Redis、无 Python）**。

---

## 2. 目标 ②③④：SQLite 切换 + 去 Redis + 环境最小化

### 2.1 SQLite 切换（最高优先，因为它一次干掉一个外部进程）

**取证结论：SQLite 零实现，且没有接口缝。**

| 检查项 | 事实 |
| --- | --- |
| 依赖 | `go.mod` / `go.sum` 无任何 sqlite 依赖；Go 源码搜 `sqlite` **0 命中** |
| 配置 | `internal/database/store.go` 的 `Config` 只有 `PostgresDSN` / `MaxConnections` / `PostgresSchema` |
| 抽象 | `type Store struct` 是**具体类型**，`db`/`queries` 私有；`*database.Store` 在包外被 **32 个文件**直接持有（`cmd/wireprobe/` 20、`internal/workflow/` 10、`cmd/gmtool/`、`internal/admin/`、1 个测试） |
| sqlc | `sqlc.yaml` 只有 `engine: "postgresql"`、`sql_package: pgx/v5`、`emit_interface: false` |
| SQL | 全在 `internal/database/sql/postgres/{migrations,queries,repairs,fixtures}`（**17 文件 93 KB**），`//go:embed sql/postgres/migrations/*.sql` |
| PG 专有语法 | `FOR UPDATE` 25、`ON CONFLICT` 57、`RETURNING` 16、`::` 转型 150、`jsonb` 21、`GENERATED` 21、`pg_` 3、`advisory` 3；另有 `FOR NO KEY UPDATE`、`ANY/unnest`、`DO` 旧表探测 |
| pgx 引用 | `internal/database/*.go` 内 **68 处**（含 `pgx.Tx` 作为事务回调参数类型、`pgx.ErrNoRows` 判定） |
| 运维耦合 | 账号金库等路径有固定锁顺序（先账号 `FOR NO KEY UPDATE`、再角色）；启动编排按 PG 拉起/探活；测试夹具用 `CREATE SCHEMA` + `search_path` 隔离 |

**可移植性判断（好消息）**：存档语义本身可移植——角色状态/背包/回执都是 **JSON 文本 + 显式字段**，
既有文档已把"保持原 JSON 字段（含未知字段）、回执键与 model、软删除、角色编号、邮件共享序列、
二进制设置、NULL/空集合语义"列为目标。真正的墙是**方言与并发原语**，不是数据模型。

**分阶段（每阶段独立提交、可独立回滚）**

| 阶段 | 内容 | 门禁 |
| --- | --- | --- |
| **S1 抽接口缝**（唯一真正的架构改造） | 以 32 个调用点为清单抽出**按领域**的 Store 接口（`CharacterStore`/`AccountStore`/`CashStore`/`MailStore`/`QuestStore`/`VaultStore`/`SettingsStore`/`ProgressionStore`…，**只含现有调用**）；把 32 个持有者改为接口；`internal/archtest` **新增两条守卫**：① `*database.Store` 不得出现在 `internal/workflow` 与 `cmd/*` 的字段/参数；② 领域接口不得引用 pgx/pgtype | **行为零变化**：同一批集成测试在改动前后同样通过；全量 `go test ./...` + `go vet ./...` |
| **S2 pgx 移出公共 API** | 事务回调参数从 `pgx.Tx` 收窄为具体能力对象 `*database.Tx`（沿用既有方向）；`internal/database` 内部分成**方言无关核心**与 **postgres 适配** | 依赖守卫证明"领域侧零 pgx" |
| **S3 sqlc 双引擎配置与 SQL 分叉** | 新增 `internal/database/sql/sqlite/{migrations,queries}`；**既有 PG 文件不改写**（新增升级走增量 SQL）；`sqlc.yaml` 加 `engine: "sqlite"`（`sql_package: "database/sql"`）生成到 `sqlcgen/sqlite` | 生成一致性检查 + 逐项差异核对（见下） |
| **S4 SQLite 引擎 + 数据转换** | `Open` 按配置选引擎（新增 `Driver`/`Dialect` 字段，**默认仍 postgres**）；写事务用 `BEGIN IMMEDIATE` + 写序列化 + busy 重试；另建 SQLite 升级脚本 + **PG→SQLite 单向转换工具** | 保持账号限购、角色存档、回执原子性（**不能机械删除锁**） |
| **S5 分叉控制** | 现有 PG 集成测试（`DFO_TEST_POSTGRES_DSN`）提取为**引擎无关契约套件**，用 `DFO_TEST_ENGINE` 驱动跑两遍 | 两引擎都必须绿；任一引擎独有行为写进文档 |

**S3 必须逐项重写的差异（不能机械翻译）**

| PG 用法 | SQLite 侧 |
| --- | --- |
| `jsonb`（21） | 存 **TEXT**；JSON 运算**回 Go**（既有文档已定"JSON 逻辑回 Go"） |
| `::` 转型（150） | 逐条改写；注意 SQLite 动态类型与整数范围 |
| `ON CONFLICT`（57） | SQLite 支持，但需核对冲突目标 / `DO UPDATE` / `excluded` 语义 |
| `RETURNING`（16） | SQLite ≥3.35；固定最低版本并在启动时校验 |
| `FOR UPDATE` / `FOR NO KEY UPDATE`（25+） | **SQLite 无行锁** ⇒ `BEGIN IMMEDIATE` 序列化写事务 + `busy_timeout`；语义等价必须在契约测试里证明 |
| `ANY/unnest`、`DO` 旧表探测 | 改多次查询 / 临时表；旧表探测改读 `sqlite_master` |
| `GENERATED ... AS IDENTITY`（21） | `INTEGER PRIMARY KEY AUTOINCREMENT` 或显式序列表；**保留邮件共享编号序列语义** |
| `pg_*` / advisory（3+3） | 应用级锁表或 `BEGIN IMMEDIATE`；管理会话锁改文件锁 |
| 时间函数 / `timestamptz` | 统一 UTC 文本或整数；**保持既有 NULL 与时间语义** |
| `search_path` 隔离 | 改**数据库文件隔离**（每个测试一个临时文件） |

**待业主拍板（S4 前必须定）**：① SQLite 定位是"单机/工具"还是将来也做多人服主库（决定写序列化与 WAL 策略）；
② 最低版本（建议 ≥3.35）；③ 驱动选 **`modernc.org/sqlite`（纯 Go、无 CGO，便于便携分发）** 还是 `mattn/go-sqlite3`（CGO）。
④ 是否**彻底放弃 PG**（单引擎只留 SQLite，可去掉 pgx 依赖与测试库）——这会简化 S1–S5 的规模，但与"双引擎"相反。

### 2.2 去 Redis（运行时已完成，剩清理）

**取证结论：运行时零 Redis。** 证据：

- `go.mod` **无** redis 依赖；`server/work/dfo-lan/**/*.go` 里 `\bredis\b`（词边界）命中**全部是误匹配**
  （`Disjoint`、`disjoint`、`AmplifyGrimoireDispatch`，不是 Redis）。真实命中只有三类**测试断言**：
  - `cmd/gmtool/storage_test.go:15` 与 `internal/database/store_postgres_test.go:17,101`：
    把 `{"redis_address":"…","redis_bin":"…","redis_password":"…","redis_prefix":"…"}` 作为**旧配置**输入，
    断言被**清理掉**；
  - `internal/inventory/protection_ticket_test.go` / `amplify_grimoire_wiring_test.go`：函数名里含 `Disjoint`/`Dispatch`。
- Python 侧同样是**清理逻辑**：`configure_env.py:274` `if not key.startswith("redis_")`；
  `test_environment_storage.py:29-48`、`scripts/test_postgres_storage.py:20` 断言旧 `redis_*` 键被删。
- 唯一残留文案：`docs/测试服登录器.cmd:143` 打印 "PostgreSQL / Redis 都没有启动"（旧提示语）。

**待清理（低风险，建议独立提交）**：
1. 删 `tools/redis/`（**46.1 MB** 便携二进制；`/tools/` 整体被 gitignore，删除只影响本地磁盘，不动仓库内容）。
2. 清掉/改写那句误导文案（`docs/测试服登录器.cmd`）。
3. 保留那几处"旧键被清理"的测试——它们是**回归保护**（证明 legacy 配置不会复活），不要删。
4. `configure_env.py` 的 `redis_` 过滤保留（继续清理历史配置文件里的遗留键）。

### 2.3 环境依赖最小化（目标态与顺序）

**目标态（验收标准）**：拿到客户端目录 + `Script.inner.pvf` + 服务端二进制，
**不需要任何外部进程**（无 PostgreSQL、无 Redis），**不需要 Python**即可启动服务端；
启动器可为本机 exe。

| 优先 | 动作 | 收益 | 依赖 |
| --- | --- | --- | --- |
| 1 | **SQLite 切换**（§2.1） | 去掉 PostgreSQL（919.8 MB 便携包 + 实例管理 + DSN/凭据） | 见 §2.1，工作量最大 |
| 2 | **去 Redis 残留**（§2.2） | 释放 46 MB + 去掉误导 | 无（立即能做） |
| 3 | **Python 编排 Go 化** | 去掉 `tools/python`（288.7 MB）与对 19 个 .py 的依赖 | 需要 channel_probe/probe.exe 与启动编排一起搬（§5 决策 3） |
| 4 | 构建缓存瘦身 | `tools/gocache` 6.4 GB + `tools/gopath` 1.96 GB 属构建缓存，非运行依赖；发布包不应包含 | 打包脚本 |

**注意**：`channel_probe.py` + `probe.exe` 是启动链的必需项，**Go 化时必须一起处理**，否则只搬到一半。

---

## 3. 目标 ①：MOD 四层接口（从 0 开始）

### 3.1 取证结论：四层里只有 PVF 层有底座

| 层 | 现状 | 证据 |
| --- | --- | --- |
| **L1 PVF 函数接口** | ⚠️ 只有**只读归档读取器**，没有可被 mod 调用的函数面 | `internal/catalog/pvf/archive.go`：`Files()` / `IterateFiles()` / `FileInfo()` / `FileCount()` / `ReadText` / `ReadBytes` / `Snapshot()` / `ReleaseReadCaches()`；仓库里唯一的脚本执行能力是 `internal/reward/lua.go`（gopher-lua，服务端自用奖励脚本） |
| **L2 NPK 资源（PVF 索引定位）** | ❌ 仓库内 `*.npk` **文件数 0**，无解析代码 | `dir /s /b *.npk` → File Not Found；全仓（排除工具链）搜 `npk` 只命中 2 处无关文本 |
| **L3 服务端函数接口** | ⚠️ 只有**显式分发链**，无 registry、无外部加载 | `cmd/wireprobe/client_dispatch.go:56` `dispatch` → `dispatchClientType` / `dispatchAccountQueries` / `dispatchCharacterSkills` …（20+ 命名阶段）+ `observedGameRequest` 白名单 |
| **L4 客户端 DLL 接口** | ❌ **仓库内无 DLL 源码** | `client-patchs/` 递归只有 **7 个文件**（`fontsize/` 下 5 个 .py + `apply-stock-fontsize.cmd` + `restore-fontsize.cmd`）；根 `AGENTS.md` §1.1 提到的 `ngstub` / plugin loader **全仓 0 命中**，其引用的 `client-patchs/AGENTS.md` **不存在** |

**外部阻塞**：`client/` 目录**已不在本仓库**（`Test-Path client` → False），
客户端 `DFO.exe` / `Script.pvf` / `sk.dat` 在仓库外（`server/launcher.local.json` → `C:/Game/dof/115us/DFO`），
权威 IDB `DFO.exe.i64` 也不在。
⇒ **L2（NPK）与 L4（DLL）的所有实测取证当前做不了**，必须由业主指定资源位置或另一棵树。

### 3.2 四层边界（先定契约）

| 层 | 职责 | 输入 | 输出 | 消费方 |
| --- | --- | --- | --- | --- |
| **L1** | 让 mod 声明/覆盖内容：注册脚本函数、声明新物品/任务/副本条目或覆盖字段 | PVF 文本脚本（`.etc`/`.cos`/`.equ`/`.qst`…） | 带来源哈希的领域规则 + 诊断 | 服务端 catalog/领域层 |
| **L2** | 让 mod 挂载资源：新贴图/动画/音效/UI，用 PVF 条目定位归档内路径 | NPK/IMG 归档 + PVF 引用 | 客户端可读资源；服务端只需"路径 → 存在/校验和" | 客户端加载器 |
| **L3** | 让 mod 挂服务端行为：新 opcode/命令、既有命令前后置钩子、事件回调、自定义领域动作 | Go 侧注册项 + mod 脚本 | 与官方路径同样的事务/存档/回包语义 | `cmd/wireprobe` 网关 + `internal/workflow` |
| **L4** | 让 mod 在客户端内运行：稳定导出 ABI、回调点（封包/资源/UI）、日志与卸载 | DLL + 清单 | 客户端行为 | 客户端进程 |

### 3.3 实施顺序（垂直切片优先，不做"四层各做完再联调"）

| 阶段 | 交付 | 理由 |
| --- | --- | --- |
| **P0 取证** | ① 客户端与 PVF 的**脚本求值模型**（PVF 是否有可调用函数面）；② 一份**真实 NPK + 加载器行为**；③ 加载器/DLL 源码归属；④ L3 钩子插入点的既有实证 | 三层都缺实测证据（根 §0.3 不许猜） |
| **P1 垂直切片** | 一个样例 mod：**新增一个物品/NPC**（数据走 L1、图标走 L2、服务端行为走 L3；暂不需要 L4） | 用最少层数同时钉住 L1–L3 契约 |
| **P2 L1 完整** | 注册面 + 覆盖（带来源哈希与优先级）+ 冲突报告 + 离线审计器（引用缺失/冲突清单） | 内容侧是一切前提 |
| **P3 L3 完整** | 钩子注册表（Pre/Post/Replace，`Replace` 必须写进启动报告）+ 失败隔离（单 mod 崩溃不拖垮网关）；mod 只能拿既有领域能力，**不得直接持 `*database.Store`**（与 §2.1 S1 同向） | 服务端优先是本仓默认路线 |
| **P4 L2 完整** | 索引定位 + 挂载/覆盖 + 校验；服务端复用做离线审计 | 依赖客户端资源 |
| **P5 L4 ABI** | ABI 定稿（`version`/`init(host_api*)`/`shutdown`/`LogPath`）+ 加载器 + 版本门禁；先只读钩子，逐钩子开放写 | 风险最高，放最后 |
| **P6 收口** | 每阶段独立 commit + `CHANGELOG` + 实机验收由业主操作 | 本仓纪律 |

### 3.4 待业主拍板

1. **加载器与 DLL 源码归属**：本仓新建完整实现，还是接入另一棵树？（本仓 `client-patchs` 只有字体脚本）
2. **能否把 `client/` 与一份真实 NPK 放回本仓**：L2/L4 的全部实测都依赖它。
3. **mod 信任模型**：只允许业主自签，还是允许第三方？（决定 L4 是否要沙箱、L3 是否要权限位）
4. **L1 覆盖语义**：只允许"追加"还是允许"覆盖既有条目"？（覆盖直接影响存档兼容与联机一致性）
5. **首个样例 mod 选什么**（建议：一个新物品 + 一个 NPC 对话）。

---

## 4. 新会话的起手式（最短路径）

1. **先读本文 §0/§1**，再读根 `AGENTS.md` 与 `server/AGENTS.md`（`server/AGENTS.md` 已随上游重构更新：
   `internal/storage` → **`internal/database`**；工具入口收敛为 `wireprobe`/`admin`/`gmtool`/`dfo-tool`
   四个，`go run ./cmd/dfo-tool charactercheck`）。
2. **建议先做两件低风险、可立即交付的**（不必等大改造）：
   - **去 Redis 残留**（§2.2：删 `tools/redis`、改误导文案），独立 commit；
   - **S1 第 1 步只做契约定稿**：产出"领域 Store 契约清单"（接口名 → 方法 → 现调用点），**不改调用点**。
3. **不要在 S1 未收口时开始写 SQLite SQL**（会同时改两层、无法定位回归）。
4. **MOD 侧在拿到 §3.4 的答复前只做 P0 的离线部分**（PVF 求值模型取证可在本仓做；NPK/DLL 必须等资源）。
5. 门禁：每个假设独立 commit；改后跑全量 `go test ./...` + `go vet ./...`；不覆盖业主工作区改动。

## 5. 别踩的坑（本仓既有教训，直接适用）

- **不要 PowerShell 往返含中文的源码文件**（会用 ANSI 读坏编码）；查源码用 read/grep 工具。
- 网关**失败分支不回 Error 包**（2259/2381 前车之鉴：两次都在收到 Error 后 `0xC0000005`）。
- `observedGameRequest` 白名单**不登记**的 opcode，第 `BodySampleLimit(8)` 帧之后 `verified` 不再计算
  ⇒ 新命令"静默失效"，症状与"没实现"一模一样。
- 写操作必须走既有 `CommitCharacterEvent` / `CommitAccountVault` 事务与幂等键，不得自己开事务。
- Go 依赖下载：本机 `proxy.golang.org` **不可达**，用 `GOPROXY=https://goproxy.cn,direct`（`GOSUMDB=off`）。
- 上游 `main` 是**保护分支**（需 Maintainer），普通账号推送会被 403；改动走 fork + MR，
  且需要项目成员身份才能建 MR。
