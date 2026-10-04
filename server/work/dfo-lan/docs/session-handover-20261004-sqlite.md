# 会话交接：SQLite 双引擎与去 Python 依赖（2026-10-04）

> 本文件用于把本轮工作整体移交到新会话。它自包含：现状、证据、我犯过并已更正的误判、
> 下一步的确切做法、运行环境事实、以及待业主决定项。

## 0. 一句话现状

SQLite 双引擎**已落地并提交**，实机已能走到「启动 → 登录 → 进入选角界面」；
**当前唯一阻塞是点击「创建角色」时客户端本地崩溃（`0xC0000005`）**，且客户端在
**发出创建请求（命令 5）之前**就崩了。触发条件已收窄到两个「客户端从未见过」的输入。

## 1. 两个仓库的提交状态

### 本仓库 `C:\Game\dof\115us\115`（main）

| 提交 | 内容 |
| --- | --- |
| `ac092bd7` | SQLite 双引擎落地 + 与 20261004 升级包交叉验证（166 文件、+22272/−486） |
| `e81b5560` | 停止入口优先走 Go 启动器（去该路径的 Python 依赖） |
| `557220be` | `sqlite_path` 必须是绝对路径（否则按进程 cwd 建库，静默建错位置） |
| `3ec50297` | `sql.ErrNoRows` → `ErrNotFound`（修穿脱装备 / 接任务失败） |

工作树中**属于本次任务但尚未提交**的：三个根目录启动入口（`启动游戏.cmd`、
`启动游戏-奥德赛.cmd`、`启动服务端.cmd`）已改为优先调用 Go 启动器并设置 `DFO_ROOT`/`PATH`。
本档案落盘时**连同这三个文件一起提交**。

工作树中**属于其它写者、不要动**：`.gitignore`、`DFO-115US单机一键启动器.exe`、
`server/work/dfo-lan/runtime/storage/pgdata/postgresql.conf`、`使用教程.md`、
`配置环境.cmd`、`scripts/README.md`、`scripts/incremental-package/`、
`打包增量更新.cmd`、`.tmp/`。

### 相邻仓库 `C:\Game\dof\115us\115us-dfolauncher`（**未提交，且有并发写者**）

我的改动（工作树内，未提交）：

- `cmd/launcher/cli.go`（新增：CLI 模式 `--check` / `--source-build` / `--launch` / `--server-only`）
- `internal/cli/cli.go`、`internal/cli/cli_test.go`（参数解析与其测试；放在 internal 是因为
  `cmd/launcher` 带 requireAdministrator 清单，连测试二进制都要管理员权限）
- `internal/run/run.go`（`Session.SourceBuild` 字段并转发 `--source-build`；SQLite 档跳过
  启动 PostgreSQL 与停止 PostgreSQL；`resolvePython` 容忍工具链被移出仓库）
- `internal/config/storage.go`、`internal/config/storage_test.go`（`StorageDriver`：显式
  `driver` 优先，只有 `sqlite_path` 也算 SQLite；9 个用例通过）

⚠️ **并发警告**：该仓库出现了**别的写者**的提交 `c83aa2b feat： 启动器增加命令行入口
（.cmd 入口不再依赖 Python）`，并且我改过的 `internal/check/check.go`（把 `tools\python`
从 `LevelError` 降为 `LevelWarn`）已不再显示为「已修改」——说明那次提交可能已包含或覆盖了它。
**下一步必须先把 `c83aa2b` 与我的改动核对去重，再决定提交哪些**，不要直接 `git add -A`。
构建产物 `bin/dfolauncher-cli.exe`（47.6 MB）已在工作树中。

## 2. 已完成且可复现的验证

- 门禁：`go build ./...`=0、`go vet ./...`=0；全量 `go test ./...`（隔离库 25439，起→跑→停）
  失败集合与基线**逐名相同**（仅两项既有 primer 失败：`TestPrimerTransformFlowIntegration`、
  `TestPrimerTransformFlowDoesNotMultiplyCrystals`）。
- 新增测试：`sqlite_norows_test.go`、`sqlite_path_test.go`、`sqlite_interop_test.go`、
  `sqlite_mailseq_interop_test.go`、`sqlite_store_test.go`、`engine_test.go`、
  `contract_dualengine_test.go` 等。
- 与「115US-SQLite-源码测试升级包（20261004）」交叉验证：两边 schema 逐库比对后表/列/索引一致
  （对方多一张 `mailbox_id_sequence`），206 处声明类型差异只影响 affinity；修掉 3 处不兼容
  （引擎选择靠 `sqlite_path`、配置键名 `busy_timeout_ms`/`max_read_connections`、
  存档身份 `PRAGMA application_id=1152026104`）以及唯一会损坏存档的分歧（邮件号段对齐）。
  报告见 `docs/sqlite-cross-validation-20261004.md`。

## 3. 当前阻塞：点「创建角色」客户端崩溃

证据来自会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261004_224700_762172_next37`：

- 服务端 22:47:35 就绪（`ready.json`/`run.json` 均已写）；客户端 22:47:35→22:47:55；
  `client.log`：`NORMAL_BEFORE_CLEANUP exit=0xC0000005`（访问冲突，客户端本地崩溃）。
- `events.jsonl` 中客户端发出的命令：`433×2、1554、1、1593、171、468、585、8、848、637、407、682`
  —— **没有命令 5**。
- 创建角色的入口：`cmd/wireprobe/client_dispatch_character.go:575 case 5:` →
  `client.characters.Create(ctx, client.developmentAccount, requestData.plaintext)`
  （`case 684` 是改名）。
- 服务端在该阶段已发出：`character_response`(id=2, 48B)、`roster_background_restored`
  （`owned_count: 0`，5 个零槽）、`roster_followup_response`(848, 433×2)、`637` → `{1,0}`、
  登录洪泛 `708/1792/1198`。
- 全新 SQLite 库状态：`accounts: 1`、`characters: 0`、`account_unified_options: 0`、
  `storage_migrations: 37`。

**两个「客户端从未见过」的输入**：① 空角色列表；② 空账号选项（`account_unified_options`）。

已排除的假设：

- 格子数 `protocol.CharacterList(uint16(s.Rules.MaxCharacters), rows)` 里的 `MaxCharacters`
  来自**配置常量**（`internal/character/service.go:231`），不是数据库；
- 「空选项行」本身不是引擎差异：服务端从不给新账号插默认行（只有客户端改设置时
  `SaveAccountUnifiedOption`），PG 上全新账号同样为空——**但**业主的 PG 账号改过设置，
  所以「空选项」对客户端仍是首次。

## 4. 我犯过并已更正的三个误判（下一轮别再踩）

1. **2127 不是「没被应答的重试」**，而是客户端**遥测流**（`cmd/wireprobe/request_scope.go:105`
   注释：每秒数次，属正常）。
2. **`unimplemented_sample` 不是「服务端未实现」**，而是**日志采样标记**
   （`cmd/wireprobe/client_connection.go:301`）。
3. **dispatcher 的 `switch` 用十进制字面量**（`case 5:`、`case 8:`），
   所以按 `\b407\b` 搜不到处理分支 = 确实没有处理分支，不代表它被特殊处理；
   407 属「只采样、不处理」的命令，与本次崩溃无关。

## 5. 下一步（拿到确证再改，遵守「禁止猜包」）

1. **登录阶段包集合差**：把本会话「登录 → 选角」阶段（登录后约 2 秒内）服务端发出的包，
   与能正常玩的旧会话 `..._20261004_221054_428343_next37`、`..._20261004_193431_865803_next37`
   的同一阶段做**集合差**：正常会话发过、这次没发的那个包，或退化了的字段，就是根因。
2. 若差集为空：进 IDA 看创建界面读取支（`145250040` 一族）对「空列表 / 空选项」的读法，
   确认真实期望，再决定补包还是补默认行。
3. 改完重新构建候选（见 §6），交由业主手动验证：**建号 / 穿脱装备 / 接任务 / 退出重进**。

## 6. 运行环境事实（本轮踩过的坑，照做可省一轮）

- `tools` 已被业主移到 **`C:\Game\dof\115us\tools`**（仓库外一层）：
  Go `C:\Game\dof\115us\tools\go\bin\go.exe`（go1.26.0）、
  Python `C:\Game\dof\115us\tools\python\python.exe`。
- 三个根启动入口已优先调用 `..\115us-dfolauncher\bin\dfolauncher-cli.exe`，并显式设置
  `DFO_ROOT`（启动器在仓库**旁边**，靠自身位置推断会找错树）与 `PATH`
  （Go 工具链被移出仓库，而 `serverbuild` 先看 `tools\go`、再看 PATH）。
- **必须带 `--source-build`**：只有候选 `bin\wireprobe-handoff-source.exe` 含本轮改动；
  默认 `bin\wireprobe-pvf.exe` 时间戳 **20:14:58**（早于 SQLite 改动）。不带会直接报
  `storage configuration incomplete`（那是 PostgreSQL 分支的 DSN 检查）。
- 启动器已按 driver 判断存储：SQLite 档打印「存储：SQLite 档，无需启动 PostgreSQL」，
  不再调用 `pg_ctl`；停止同理。
- 存储配置 `runtime\storage\local.json`：`driver=sqlite`，
  `sqlite_path` 为**绝对路径** `C:/Game/dof/115us/115/server/work/dfo-lan/runtime/storage/dfolan.sqlite3`。
  PG 配置备份 `local.json.pg-backup`；旧 SQLite 库备份 `dfolan.sqlite3.first-try`。
- 读日志：`runtime\roles_<tag>\` 下 `events.jsonl`（探针事件：client_frame/server_frame 及各
  结构化 send 事件）、`gateway.err`（服务端 stderr）、`helper.out`（探针转写的客户端日志）、
  `client.log`（客户端退出码）、`ready.json`/`run.json`（就绪标记）。
- 跨仓库构建需 `GOPROXY=https://goproxy.cn,direct`、`GOSUMDB=off`
  （`proxy.golang.org` 在本机不可达），并 `CGO_ENABLED=0`、`GOTOOLCHAIN=local`。
- 实机纪律：不启动客户端、不代跑；测试库可动，**玩家库绝不触碰**（PG 25438）。

## 7. 待业主决定

| 项 | 说明 |
| --- | --- |
| **Python 彻底移除** | 拦路点是 `115us-dfolauncher/internal/run/run.go` 的 `runLauncher`——**所有模式**（含 `compat`）起服务端时都调 `launch_local.py`。两条路：**A 冻结**（PyInstaller 冻结 `channel_probe.py`→`launch_local.py`，快，但 Python 仍是实现语言）/ **B 移植到 Go**（慢，真去掉；`check/compat.go` 是那套参数规则的镜像表，可作对照）。业主尚未选定。 |
| **发布新默认程序** | SQLite 版验收通过后，用 `pwsh -NoProfile -File .\server\Build-Server.ps1 -UpdatePVFDefault` 正式发布；该脚本默认**保留**已验收默认程序。 |
| **另两条 Python 尾巴** | `配置环境.cmd`（`scripts/configure_env.py`）与 `build-publish.ps1` 的打包步骤（因 `tools` 移走**已坏**，建议改用 PowerShell `Compress-Archive`）。 |

## 8. 本轮用到的脚本（`server/work/dfo-lan/.tmp/`，未入库）

`gen-sqlite-adapter.go`（适配器生成器）、`launcher-cli.py`/`launcher-cli2.py`/`launcher-cli3.py`/
`launcher-cli4.py`（CLI 与 internal/cli 拆分）、`launcher-sqlite-storeboot.py`（SQLite 存储判断）、
`launcher-sourcebuild.py`（转发 `--source-build`）、`launcher-storage-test.py`、
`launcher-python-path.py`（解释器查找）、`wire-launch-entries.py`（.cmd 接线）、
`fix-cmd-ascii.py`（.cmd ASCII/PATH 修正）、`fix-norows.py`（ErrNoRows 映射）、
`fix-sqlite-path.py`、`commit-*.py`。
