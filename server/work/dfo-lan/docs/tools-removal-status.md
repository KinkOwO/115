# 移除 tools 依赖：当前状态与待决事项

一句话：**代码侧两种引擎都已就绪并验证**，`tools/pg` 只差你在 SQLite 上实机跑一次；
`tools/python` 只差 CLI 编排的落点决定（GUI 启动器没有 CLI 模式，所以源码路径仍需 Python）。
详细的依赖清单与三轮修正见 [runtime-without-tools-plan.md](runtime-without-tools-plan.md)；
操作手册见 [sqlite-operations.md](sqlite-operations.md)。

## 已完成并验证

| # | 事项 | 证据 |
| --- | --- | --- |
| 1 | **SQLite 适配器**：嵌入 `sqlcgensqlite` + 277 生成方法 + 36 逐字段转换器 + 5 个引擎专属实现 | `var _ querySet = (*sqliteQueries)(nil)` 编译通过；`sqlite_adapter_test.go` 真实驱动端到端（含 JSON 往返、int32 收窄、`NextMailID` 取号） |
| 2 | **PG 侧零适配器** | `var _ querySet = (*sqlcgen.Queries)(nil)` 编译通过 |
| 3 | **引擎缝**：`queries()` / `begin()` / `holdAdminGuard` / `close()` | `engine_test.go`：提交落库、回滚撤销、回调错误原样返回、引擎专属方法在事务内外都可用 |
| 4 | **Store 接到缝上**（77 个文件，机制化替换） | 全量 `go test ./...` 失败集合与基线**逐名相同**（仅 2 项既有 primer 失败） |
| 5 | **`Open` 按 driver 分派** + SQLite 一次性建表 | `sqlite_store_test.go`：Store 级端到端、重开幂等（数据仍在）、未知 driver 报错、`rawPool` 在 SQLite 上正确拒绝 |
| 6 | **PG→SQLite 转换器** + 校验 | `sqliteconvert_test.go`：逐表行数、逐行 JSON 字节哈希、`foreign_key_check`，并用服务端自己的 `Open` 读回。开发中抓出并修掉 3 个真实缺陷 |
| 7 | **转换器入口** | `dfo-tool sqliteconvert --config <pg档> --out <新文件>`；目标必须不存在 |
| 8 | **双引擎契约套件** | `TestEngineContract/sqlite` 与 `/postgres` 同批通过（幂等、取号 +1、守卫获取/释放/再获取、迁移入口安全） |
| 9 | **启动链与停止链按 driver 分叉** | `launch_local.py` 行为验证（sqlite→`pg=None`、缺 `sqlite_path` 拒绝、未知 driver 拒绝）；`stop_environment.py` 同理 |
| 10 | **打包与配置不再无条件要求** | `build-publish.ps1` 按实际打包内容决定（PS 解析器验证 565 tokens 无错）；`configure_env.py` 只给 postgres 档写 `postgres_bin` |
| 11 | **Go 启动器地基** | `cmd/dfolauncher`：`stop`/`check`/`start-storage`，共 **10 条测试**；对真实配置 dry-run 验证 |

## 验证门禁（谁保证上面那些"已完成"是真的）

这些是自动化门禁，不是一次性检查——它们会在每次改动后重新判定，所以结论不会随时间失效：

| 门禁 | 覆盖的失效方式 |
| --- | --- |
| `TestEngineContract`（sqlite + postgres 子测试） | 两个引擎行为漂移：幂等、取号 +1、守卫获取/释放/再获取、迁移入口可安全调用 |
| `TestSQLiteQueryTreePreparesAgainstSchema` | **编译期看不到的 SQL 错误**：列名笔误、移植时漏掉的表、语法差异。逐条 `Prepare` 全部 277 条语句 |
| `TestEveryMigrationEntryPointIsEngineNeutral` | 将来有人新增直接连驱动的迁移入口——那会**只在 SQLite 启动时**失败 |
| `TestSaveIdentityMigrationUsesGeneratedQueriesOnly` | 存档身份归一（唯一在 SQLite 上**不是**空操作的启动迁移）被改成直连驱动而静默失效 |
| `sqlite_schema_test.go` | SQLite schema 与 PostgreSQL 漂移、分节漂移、SQL 文件出现非 ASCII（sqlc 的 SQLite 语法会因此拒绝） |
| `generated_parity_test.go` / `queryset_test.go` | 两个生成树的方法集合或签名漂移（含 41 处**逐条注明理由**的允许分歧）；`querySet` 覆盖面的双向对齐 |
| `sqlite_adapter_test.go` / `engine_test.go` / `sqlite_store_test.go` | 适配器、引擎缝、`Open` 分派的运行时行为（真实驱动，无 PostgreSQL） |
| `sqlite_admin_lease_test.go` | 管理守卫的崩溃安全：过期回收、续租、释放幂等、释放后不复活 |
| `sqliteconvert_test.go` | 转换器的行数、JSON 字节与引用完整性校验（隔离库，仅在该库上跑） |

运行方式见上一节；需要真实 PostgreSQL 的用例由 `DFO_TEST_POSTGRES_DSN` 指向**隔离测试库**后才会执行，
未设置时自动跳过。

## 已交付的两条命令（工具链移除是可执行的，不是计划）

| 命令 | 作用 | 验证方式 |
| --- | --- | --- |
| `移除tools.cmd [源] [目标]` | 把 `tools` **移动**到仓库外（默认 `..\115-tools-moved`），不删除 | 在替身目录上验证：移动生效、源不存在时 `Nothing to move`、目标已存在时 `Refusing to move`；**真实 tools 全程未被触碰** |
| `还原tools.cmd [目标] [源]` | 移回来 | 同上：替身 `marker.txt` 回到原位并输出 `Restored "…" from "…"`；目标缺失时 `Nothing to restore` |

两者都写成**无分支**的直线脚本（去掉了一个让我三次补丁失败的分支），因此行为可被替身目录完整证明。
移动前脚本会打印影响清单，避免"移走了才发现起不来"。

## 最后一轮的状态证据

- 全量 `go test ./...`（隔离库 25439，起→跑→停）的失败集合与基线**逐名相同**：仅
  `cmd/wireprobe` 的 `TestPrimerTransformFlowIntegration` 与
  `TestPrimerTransformFlowDoesNotMultiplyCrystals` 两项**既有**失败，**无新增**。
- `go build ./...`=0、`go vet ./...`=0。
- `internal/launcher` **21 条测试全过、0 跳过**（含对真实 `wireprobe-pvf.exe` 的参数探测、
  以及对 `channel_probe.py` 源码的环境决策同步门禁）。

## 待决事项（都需要你拍板）

### ① SQLite 实机验收 —— `tools/pg`（919 MB）的唯一阻塞

`.tmp/sqlite-acceptance/` 里有 `local.json` 与逐步说明（含备份、替换、回退）。
替换后：启动器不再拉起 PostgreSQL，停止脚本跳过停机，游戏在 `runtime/storage/dfolan.sqlite3` 上运行。
验收范围建议：选角 → 进城 → 打副本 → **退出重进**（验证持久化）。

### ② CLI 编排落点 —— `tools/python` 的阻塞

发布版（GUI 一键启动器）**运行时零 Python 调用**，但源码/开发路径的三个入口
（`启动游戏.cmd --source-build`、`启动服务端.cmd`、`启动游戏-奥德赛.cmd`）仍调 `launch_local.py`，
而 GUI 启动器**只有 `--gm-window` 一个命令行参数**，无法替代它们。二选一：

- **A'**：给相邻启动器 `115us-dfolauncher` 加 CLI 模式（需要改那个仓库）；
- **B'**：在本仓库 `cmd/dfolauncher` 内完成（已有 `stop`/`check`/`start-storage`）。

无论选哪个，能力清单相同：**存储启动 → 探针拉起 → PVF 准备 → 服务端拉起 → 等待网关 → 客户端拉起**。
存储启动这一段已完成。

### ③ 是否允许改另一个仓库

把 `115us-dfolauncher/internal/check/check.go:114-116` 的 `tools\python\python.exe`
从 **LevelError** 降级（并把 199-202 行的四个 `.py` 入口降为可选），GUI 启动路径就完全不需要
`tools/python`。那是独立 git 仓库，我不擅自动。

## 复核命令

```powershell
cd server\work\dfo-lan
$env:CGO_ENABLED='0'; $go='..\..\..\tools\go\bin\go.exe'
& $go build ./...                                     # 0
& $go vet ./...                                       # 0
& $go test -count=1 ./internal/database/ ./internal/archtest/ ./internal/launcher/
& $go run ./cmd/dfolauncher check --root ..\..\..     # 只读依赖检查
& $go run ./cmd/dfolauncher stop --dry-run            # 只打印，不终止任何进程
```

需要真实 PostgreSQL 的用例由 `DFO_TEST_POSTGRES_DSN` 指向**隔离测试库**后运行；
未设置时自动跳过，不会接触玩家库。基线失败集合（未启用该变量时也一致）为
`cmd/wireprobe` 的 `TestPrimerTransformFlowIntegration` 与
`TestPrimerTransformFlowDoesNotMultiplyCrystals` 两项既有失败。
