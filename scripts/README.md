# scripts/ — 仓库级脚本

本目录存放**仓库级脚本**（管环境、启动、打包、停止、提交门禁），与各组件自带的脚本目录分开。
`2026-10-04` 从仓库根目录收拢到这里；同日晚些时候**根目录的运行/维护 `.cmd` 也全部移入本目录**
（根目录不再有任何 `.cmd`，见根 `AGENTS.md` §0.4.1）。

## 文件

| 文件 | 作用 | 谁调用 |
| --- | --- | --- |
| `build-bot-client.ps1 -Survey` | 集中观察候选，仅构建与离线测试；帧/通知/阶段/清理/角色事件共用一轮，输出到忽略目录 `build/survey` | [集中清单](../docs/todo/bot-client-v1-research.md)，不启动游戏 |
| `check-bot-survey-log.ps1` | 只读共享读取日志，报告各项 `survey_coverage`；退出 0 只代表数据/恢复完整，2 缺证据，1 格式/检查错误 | 用户停止集中观察并恢复后检查 |
| `build-bot-client.ps1 -PhaseFrame` | 仅编译独立 x64 阶段观察候选并执行离线检查；不启动游戏，输出到忽略目录 `build/phase-frame` | 已授权客户端 DLL 调研时手动构建，操作说明见 [bot 候选](../client-patchs/botclient/README.md) |
| `check-bot-phase-log.ps1` | 只读共享读取阶段探针日志；退出 0 只代表观察完整，2 表示缺证据，1 表示格式/检查错误；不授予执行或退役权限 | 用户完成阶段观察及恢复后检查 |
| `check-commit-hygiene.ps1` | **提交前门禁**：检出「本地缓存/构建产物入库」与目录规范违规；退出码 2 = 需业主二次确认（根 `AGENTS.md` §0.3.1） | 任何提交前手动跑：`pwsh -NoProfile -File scripts/check-commit-hygiene.ps1` |
| `storage-route.ps1` / `storage-route.cmd` | **双库双路线切换器 + Go 启动链调用**：`show` 看当前路线、`use sqlite`/`use postgres` 切换、`stop-postgres` 停 PG、`preflight-postgres` 只做起库预检、`clear-guard` 清过期 SQLite 管理租约、`chain-info` 报告 Go 启动器与强制开关、`selftest` 自检，以及四个入口实际调用的 `game-*`/`server-*`（逻辑见 `server/work/dfo-lan/docs/sqlite-operations.md` §1.2） | 四个路线启动入口内部调用；也可手动 `scripts\storage-route.cmd show` |
| `检查环境.cmd` | **只读环境体检（2026-10-07 重写）**：优先 `bin\dfolauncher.exe check`，没有启动器但有 `tools\go` 时用 `go run ./cmd/dfolauncher check` 跑源码版，两个都没有就打印取法；**没有 Python 回退**（`launch_local.py` 已于 2026-10-05 删除）。退出码 = 体检结论（旧版恒为 0） | 人手动 `scripts\检查环境.cmd`；也可用 `scripts\storage-route.cmd chain-info` |
| `配置环境.cmd` / `configure-env.ps1` | **环境自举（2026-10-07 重写，纯 PowerShell，不需要 Python）**：按 `tools\manifest.json` 把仓库自带的 zip 解到各自的 `target`（`go`→`tools\go`、`gopath-mod`→`tools\gopath`、`server-src`/`server-configs`→`server\work\dfo-lan`、`server-bin`→`server`），解包前按清单核对 `size`+`sha256`；**幂等**（清单条目的 `check` 路径在位就跳过），`-Check` 只读体检、`-Package go,gopath-mod` 选包、`-Force` 才重解 —— git 工作区里目标落在服务端源码树（`server`、`server\work\dfo-lan`）的包（`server-src`/`server-configs`/`server-bin`）**默认一律不解**，另有「包内文件与跟踪文件同名且内容不同就跳过」的机械校验（实测 server-bin 的 testdata 比工作区旧），只有 `-Force` 才会按包覆盖。退出码：0 就绪 / 2 缺包 / 3 校验或解包失败 / 1 脚本自身错误。它替代的是 2026-10-05 删掉的 `configure_env.py` | 人手动 `scripts\配置环境.cmd`（双击）；启动器 / 发布包走同一份 `tools\manifest.json` |
| `build_publish_zip.py` | 打发布包 | 根 `build-publish.ps1:59` |
| `启动modkit.cmd` / `start-modkit.ps1` | **一键启动 modkit 简易页面**（本地小服务 + 单页 UI，浏览器里管 mods 目录：分页列表 / 批量启停删 / 导入导出 zip）。默认管仓库根的 `mods\`（`--scan-depth 2`，认得 `client-mods\*.zip` 与 `examples\<mod>\`）；`-ServerMods` 改管 `server\work\dfo-lan\mods`；`-Open`（**默认不自动打开浏览器**，业主 2026-10-06 要求）`/ -Port / -ModsDir / -Rebuild` 可选；端口已被占用时只把地址打出来，不起第二个实例。**运行 exe 不需要 Go**（`mods\modkit-web\modkit-web.exe`；缺了才去找 Go 编译，本机 Go 在包外 `..\tools\go\bin\go.exe`） | 人手动 `scripts\启动modkit.cmd`；工具本体与说明见 `mods/modkit-web/README.md` |

> **2026-10-05：本目录的 Python 运行脚本已全部移除**（`configure_env.py`、`storage_profile.py`、
> `stop_environment.py`、`test_environment_storage.py`）。启动/停止/存储判定全部由仓库内 Go 启动器
> （`server\work\dfo-lan\bin\dfolauncher.exe`）承担；**唯一的命令行 GM（`GM.cmd` + `gm.py`）也已移除**
> —— 它依赖的 `storage_profile.py` 在去 Python 那一轮被删掉之后就一直跑不起来，而 GM 现在只有
> **启动器内嵌的那一个**（`gm/` → `gmbridge.exe`，Go，见启动器仓库 README「GM 与存储档」）。
> 本目录仍保留的 Python 只剩 `build_publish_zip.py`（打包，属另一条工作）；包外那份便携 Python
> （`..\gm-tool\python`）仍被仓库根的 `build-publish.ps1` 用来打 zip，所以继续保留。

### 写 `.cmd` 的硬要求（2026-10-05 实机踩坑后定）

1. **执行行保持纯 ASCII**：中文只放进 `rem`，需要中文提示就让带 BOM 的 `.ps1` 打印。
   cmd.exe 用**控制台代码页**解码批处理文本，`chcp 65001` 只对其后**被重新读取的行**生效；
   UTF-8 中文一旦落在带引号的执行行（`-File "scripts\中文名.ps1"`）就会被拆成乱码命令，
   实测报 `'hell' is not recognized as an internal or external command`。
   中文**文件名**没问题（Explorer / PowerShell 调用时按 UTF-16 处理）——有问题的是文件**内容**里的中文执行行。
2. **CRLF + 不带 BOM**：裸 LF 会让 cmd 按行解析错位、把相邻行粘成一条（同一坑的另一半）；
   BOM 会污染首行 `@echo off`。两者门禁都会报 `[环境不匹配]`。
3. 所以各入口都只有几十行 ASCII 调度，路线切换、中文提示、提权全部在 `storage-route.ps1`
   里（UTF-8 带 BOM）。2026-10-07 落定的入口与委托关系（PostgreSQL 已于 2026-10-05 随引擎移除，
   `启动游戏-PostgreSQL.cmd` / `启动服务端-PostgreSQL.cmd` 不再存在）：
   - `启动游戏-SQLite.cmd` / `启动服务端-SQLite.cmd` → `storage-route.ps1 game-sqlite` / `server-sqlite`（显式路线）
   - `启动游戏.cmd`（剧情默认）→ `storage-route.ps1 game-current`（不换路线）
   - `启动游戏-奥德赛.cmd`（强制奥德赛档）→ 同一 `game-current`，只是多设 `DFO_ODYSSEY_MODE=1`
   - `启动服务端.cmd`（仅服务端）→ `storage-route.ps1 server-sqlite`

### 各入口原本写在 `.cmd` 里的中文说明（现集中在此）

`.cmd` 一律纯 ASCII 后，原先写在里面的中文注释与提示按入口搬到这里；
**其余行为（环境变量、委托链、退出码）逐字未改**。

#### `启动游戏.cmd` / `启动游戏-奥德赛.cmd`

- 不再设置 `DFO_ODYSSEY_MODE`：游戏模式按**角色存档投影**（建号请求 `options[10]`，剧情 0 / 奥德赛 2），
  与客户端自己读的 per-character 标记一致。
- 需要整档强制时才用 `启动游戏-奥德赛.cmd`（`DFO_ODYSSEY_MODE=1`），或在启动器设置里选强制档。
- 默认使用 `configs/pvf-default.json`；分别保留剧情 / 奥德赛模式，`--json-mode` 是显式回退。
- **2026-10-07 重写（去 Python 收尾）**：两个入口都只做 ASCII 调度 + `bin\dfolauncher.exe` 在位检查，
  然后调 `storage-route.ps1 game-current`（**不换路线**，路线只由 `*-SQLite.cmd` 这类路线入口决定）。
  奥德赛入口是两者唯一的差别：它多设 `DFO_ODYSSEY_MODE=1`（整档强制），并在窗口标题里标明。
  旧版那条 `..\115us-dfolauncher\bin\dfolauncher-cli.exe` / `launch_local.py`（Python）回退已删除。

#### `启动服务端.cmd`（装备库 CMD2259 的三个开关，默认都不设）

- `DFO_EQUIPMENT_CRAFT_WINDOW`：**默认 1** = 窗口 3937。若点「制作/变换」没进制作界面，
  把 `rem set DFO_EQUIPMENT_CRAFT_WINDOW=0` 放开再启动，试窗口 2145。
- `DFO_EQUIPMENT_CRAFT_EXECUTE_ON`：`confirm`（默认，同一次操作的第二次请求）/ `first`（第一次就执行，慎用）/ `never`。
  若点了生成没有任何反应、日志也只有一条 2259，可放开 `rem ... =first`。
- `DFO_EQUIPMENT_CRAFT_GENERATE_VARIANT`：生成应答的子分支字节（`payload[5]`）。**默认 1** = 只落成功标志、
  不动窗口状态；设 0 = 强制 `setState` 到状态 3 —— 那个状态客户端自己不会进也没有出口，进去后
  「切换材料」按钮会失灵（实机 2026-09-29 14:36）。一般不用动。
- **2026-10-07 重写（去 Python 收尾）**：本入口改调 `storage-route.ps1 server-sqlite`（显式 SQLite 路线，
  SQLite 是唯一引擎）；删掉了指向 `launch_local.py`（Python）与仓库外 `..\115us-dfolauncher\` 的两条回退。
  同样是 ASCII 调度 + `bin\dfolauncher.exe` 在位检查，提权仍在 `storage-route.ps1` 里做。
  与 `启动服务端-SQLite.cmd` 等价，保留这个通用名只是为了老习惯与新手的直觉。

#### `GM.cmd`（**已删除**，2026-10-05）

- 原来的链路是 `GM.cmd`（纯 ASCII 调度）→ `scripts\gm.py`（Python）→ `dfo-tool accountlist` /
  `cmd/admin` / `dfo-tool setlevel`。它依赖的 `scripts\storage_profile.py` 在「去 Python」那一轮被删除，
  于是 `gm.py` 一跑就 `ModuleNotFoundError`，整条链路已经不可用。
- 现在**只有启动器内嵌的 GM**（`gm/` → `gmbridge.exe`，Go 实现，两条存储档都能用，见启动器仓库
  README「GM 与存储档」）：命令行 GM 不再维护，也没有 Go 等价物的计划（需要命令行时用
  `dfo-tool accountlist` / `cmd/admin` / `dfo-tool setlevel` 本身）。

#### `配置环境.cmd`

- 该文件此前是 **GBK 编码**（非 UTF-8，违反 §0.4.2），已重建为 UTF-8 无 BOM 的纯 ASCII 调度；
  中文报告与提示由 `scripts\configure_env.py` 输出。
- **2026-10-07 重写**：`configure_env.py` 在 2026-10-05「去 Python」那一轮被删除后，这个入口一直调它 ——
  别人 clone 下来双击必挂（`python: can't open file ... configure_env.py`）。现在它只调
  **`scripts\configure-env.ps1`（纯 PowerShell，UTF-8 带 BOM）**，数据源是仓库自带的 `tools\manifest.json`：
  - 按清单把 zip 解到各自 `target`：`go`→`tools`、`gopath-mod`→`tools`（整包不在时先调
    `assemble-gopath-mod.ps1` 拼分片）、`server-src`/`server-configs`→`server\work\dfo-lan`、`server-bin`→`server`；
  - 解包前核对清单的 `size` + `sha256`，不一致直接报错，绝不解半个包；
  - **幂等**：清单条目的 `check` 路径在位就跳过；`-Check` 只读体检（不写盘）、`-Package go,gopath-mod` 选包、
    `-Force` 才重解；
  - **git 工作区保护（2026-10-07 沙箱实测后加，别去掉）**：包里是**发布快照**，工作区里是**源码**，
    两边不一致时解包就是给跟踪文件降级。实测 `tools\tools-server-bin.zip` 里那份
    `cmd\wireprobe\testdata\config_help.json` 比工作区旧（还留着上游已删掉的 `-boostup-challenge`），
    解包后 `cmd/wireprobe` 的 `TestWireprobeConfigHelpAndCLIRejection` 直接 FAIL。因此：
    git 工作区里，目标落在服务端源码树（`server`、`server\work\dfo-lan`）的包**默认一律不解**
    （`server-src` / `server-configs` / `server-bin` 都属此类）；其余包解包前再做一道机械校验 ——
    包内文件与工作区里的**跟踪文件**同名且内容不一致，同样跳过并点名。解包目录（没有 `.git`，
    如玩家整合包）不受限制，照解；
  - 只想编译服务端的人，装好 `go` + `gopath-mod` 就够（`bin\dfolauncher.exe` 自己 `go build` 出来）；
    确实要按包内快照覆盖跟踪文件时才加 `-Force`（脚本会打出将被覆盖的文件数）；
  - 退出码：`0` 全部就绪 / `2` 有包既没就绪也没有可用 zip（要人工取包）/ `3` 校验或解包失败 / `1` 脚本自身错误
    （按保护规则跳过不算错误，会在输出里点名）；
  - 全程不需要 Python、不需要联网；跑完会打印「下一步」（离线编译命令、产启动器、看路线、启动游戏）。
- 例：`scripts\配置环境.cmd -Check`、`scripts\配置环境.cmd -Package go,gopath-mod`、`scripts\配置环境.cmd -Force`。

#### `检查环境.cmd`

- **只读体检**：看依赖、看配置，不启动也不停止任何东西。
- **2026-10-07 重写（去 Python 收尾，同类第 4 个残留）**：旧版在「`bin\dfolauncher.exe` 不在、
  `tools\go` 也不在」时会回退到 `python ... launch_local.py --check` —— 那个文件在 2026-10-05
  已经删掉，所以这条分支一跑就是「找不到文件」。现在只有三条路，**没有 Python 回退**：
  1. `server\work\dfo-lan\bin\dfolauncher.exe check --root <仓库根>`（首选，预编译或本机编好的都走这条）；
  2. 没有启动器但有 `tools\go` 时，用源码跑一次：`tools\go\bin\go.exe run ./cmd/dfolauncher check …`
     （脚本会把 `GOPATH`/`GOMODCACHE` 指到 `tools\gopath`，全新 clone 也能离线体检）；
  3. 两个都没有 → 直接打印两条取法（`configure-env.ps1`，或自己 `go build` 出启动器）。
- **退出码 = 体检自己的结论**（0 = 没问题，非 0 = 报出的问题数对应的码）。旧版无论查出什么都 `exit /b 0`，
  脚本化调用时看不出结果 —— 这次一并改掉。
- 入口里**没有 `if ( … )` 块**：cmd 在块内按解析期展开 `%RC%`，退出码必须在普通行上读（旧版为此专门
  写了 `call :go_check`）。写法照这条来，别再包块。

#### `移除tools.cmd`

- 移动而不是删除：只是把 `tools` 挪到仓库外，随时可用 `还原tools.cmd` 移回来。
- 移走后：`检查环境.cmd` / `停止游戏环境.cmd` 仍可用（走 `bin\dfolauncher.exe`）；
  **启动链也已不依赖 `tools\`**（2026-10-05 起只走仓库内 Go 启动器）；
  PostgreSQL 档会失去便携 PG（`postgres_bin` 指向哪台都行，改路线档即可），SQLite 档不需要它。

#### 停止环境时的 SQLite 管理租约（2026-10-05）

- 强杀服务端会留下 `<db>.admin-guard` 租约，60 秒 TTL 内新服务端会被拒（实机：
  「已有 GM 写入正在进行…由进程 13248 持有」）。**停止链自己收**：SQLite 档下
  `dfolauncher stop` 在强杀之后读租约里记录的 pid，
  只在平台明确回答「该 pid 不存在」时删除；pid 还活着或问不出来一律保留并说明原因。
- 没走过停止链（崩溃）时手动清一次：`scripts\storage-route.cmd clear-guard`（同一套判据）。
- 租约里读不出 pid（空文件/内容异常）时都不删——等 TTL，或由你确认后手工删除。

#### PG 路线起库不再「卡住」（2026-10-05）

- 实机症状：PG 日志已 `ready to accept connections`，控制台却停在「拉起 PostgreSQL」，只能 Ctrl+C。
- 根因：Windows 上 `pg_ctl start` 留一个 `cmd.exe` 包装器当 postgres 的父进程，**继承调用者的
  stdout/stderr**；用管道等它（Go `CombinedOutput`）或共享控制台等它（PowerShell `&`）都会永远等下去。
- 现在的写法：`storage-route.ps1` 用 **WMI 创建进程**（不继承调用者句柄）+ 命令行内部重定向到
  `runtime\storage\pg-ctl.out.log`，**只轮询端口**（上限 60 秒、每 5 秒报进度）；Go 侧改成写文件的
  `Run()`。单独试：`scripts\storage-route.cmd preflight-postgres`（实测 3.1 秒返回、1.9 秒就绪）。
- 数据目录里 pid 已死的 `postmaster.pid` 会被先清掉并打印；pid 还活着则不动。

## 子目录

- `local-fixes/` —— 仓库级的一次性修复/移植脚本（2026-10-03 入库，内含 README）。
- `incremental-package/` —— **另一条并行工作**在建的增量打包子目录，不属于本次整理范围，请勿改动。
## 不在这里的东西（有意为之）

- **JSON 没有收拢**。仓库里的 JSON 是三类完全不同的东西，混在一起会破坏既有契约：
  1. **内容基线**：`server/work/dfo-lan/configs/*.json` —— 根 `AGENTS.md` §0.2 规定 **PVF 是唯一内容真源**，
     这些只是历史对照/审计用途，**不是运行输入**；
  2. **运行配置（路径被写死）**：`server/work/dfo-lan/runtime/storage/local.json`、
     `server/launcher.local.json`、`server/work/dfo-lan/configs/pvf-default.json`
     —— 被 Go 启动器（`storage-route.ps1` + `dfolauncher`）按固定路径读写；
  3. `gm-tool/configs/` 是 GM 工具自己的配置。
- **组件自带脚本目录**保持原位，它们各自被本组件的构建/启动链按路径引用：
  - `server/work/dfo-lan/scripts/`（PVF 导出/审计等开发工具 + `Generate-SQL.ps1`；启动编排已全部收进 Go）
  - `server/work/dfo_probe_tools/`（`probe.exe`；`channel_probe.py` 已于 2026-10-05 删除）
  - `gm-tool/scripts/`

- `incremental-package/` 是**另一条并行工作**的在建子目录，不属于本目录的整理范围，请勿改动。

## 运行入口（全部在本目录）

**两条存储路线各有自己的启动入口**（2026-10-05 业主定调：双库兼容方案要能分别启动）：

| 路线 | 存档位置 | 游戏全链 | 只起服务端 |
| --- | --- | --- | --- |
| SQLite | `runtime\storage\dfolan.sqlite3`（单文件，不需要 PostgreSQL） | `启动游戏-SQLite.cmd` | `启动服务端-SQLite.cmd` |
| PostgreSQL | `runtime\storage\pgdata`（本地 PostgreSQL 实例，端口 25438） | `启动游戏-PostgreSQL.cmd` | `启动服务端-PostgreSQL.cmd` |

四个入口都先让 `scripts\storage-route.ps1` 把活动档切成该路线；PostgreSQL 路线再跑一次**有上限的
起库预检**，然后由脚本调用**仓库内的 Go 启动器**把全链拉起来（业主 2026-10-05 第三次定调：
**彻底移除所有外部环境依赖（含 Python），不再回退，必须 Go 成功**）：

| 环节 | Go 实现 |
| --- | --- |
| 存储 | `internal/launcher/storage.go`（SQLite 文件 / PostgreSQL 实例，`pg_ctl` 输出写文件不过管道） |
| 内层 PVF | `internal/launcher/innerpvf.go` |
| 会话 fixture / 网关 argv / 环境 | `internal/launcher/fixture.go`、`gateway.go`、`probeenv.go` |
| 起网关 + `run.json` + 就绪轮询 | `internal/launcher/serverrun.go` |
| **客户端宿主 + WFP 隔离** | `internal/launcher/clienthost*.go`（`probe.exe` 只作显式回退，脚本里已禁用） |

启动链**只有一条**：`server\work\dfo-lan\bin\dfolauncher.exe launch [--server-only] [其它参数]`。
两个游戏入口在启动时强制 `DFO_REQUIRE_GO_ISOLATION=1`：Go 隔离装不上就**报错停下**，不会静默改用
`probe.exe`。二进制由源码构建（`cd server\work\dfo-lan; go build -trimpath -o bin\dfolauncher.exe .\cmd\dfolauncher`），
运行期不需要 Python、不需要 `..\115us-dfolauncher` 那个外部启动器。
`scripts\storage-route.cmd chain-info` 会报告二进制是否在位、会执行什么命令行、带了哪些强制开关。

**两条路线的存档互相独立**，切换路线不会带着角色走；搬运存档用 `dfo-tool sqliteconvert`
（只支持 PostgreSQL → SQLite 单向）。

其余入口：`启动游戏.cmd`、`启动游戏-奥德赛.cmd`、`启动服务端.cmd`、`配置环境.cmd`、`检查环境.cmd`、
`停止游戏环境.cmd`、`storage-route.cmd`，以及提交门禁 `check-commit-hygiene.ps1`
（2026-10-07 已落定 **奥德赛 / 启动服务端 / 配置环境** 三个入口的 Go 链重写；其余入口仍由并行工作整理，
按其落定为准）。

> 这些 `.cmd` 一律先 `cd /d "%~dp0.."` 回到仓库根，因此内部路径仍按仓库根书写（根 `AGENTS.md` §0.4.2）。
> 构建脚本 `build-publish.ps1` 仍在**仓库根**（被 `一键打包.cmd` 以 `%~dp0..\build-publish.ps1` 调用）。
