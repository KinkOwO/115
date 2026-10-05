# scripts/ — 仓库级脚本

本目录存放**仓库级脚本**（管环境、启动、打包、停止、提交门禁），与各组件自带的脚本目录分开。
`2026-10-04` 从仓库根目录收拢到这里；同日晚些时候**根目录的运行/维护 `.cmd` 也全部移入本目录**
（根目录不再有任何 `.cmd`，见根 `AGENTS.md` §0.4.1）。

## 文件

| 文件 | 作用 | 谁调用 |
| --- | --- | --- |
| `check-commit-hygiene.ps1` | **提交前门禁**：检出「本地缓存/构建产物入库」与目录规范违规；退出码 2 = 需业主二次确认（根 `AGENTS.md` §0.3.1） | 任何提交前手动跑：`pwsh -NoProfile -File scripts/check-commit-hygiene.ps1` |
| `storage-route.ps1` / `storage-route.cmd` | **双库双路线切换器 + Go 启动链调用**：`show` 看当前路线、`use sqlite`/`use postgres` 切换、`stop-postgres` 停 PG、`preflight-postgres` 只做起库预检、`clear-guard` 清过期 SQLite 管理租约、`chain-info` 报告 Go 启动器与强制开关、`selftest` 自检，以及四个入口实际调用的 `game-*`/`server-*`（逻辑见 `server/work/dfo-lan/docs/sqlite-operations.md` §1.2） | 四个路线启动入口内部调用；也可手动 `scripts\storage-route.cmd show` |
| `configure_env.py` | **已删除（2026-10-05，去 Python）**：环境配置改由启动链自己写 `runtime/storage/local.json`（`storage-route.ps1 use …` + `dfolauncher init-storage`） | — |
| `build_publish_zip.py` | 打发布包 | 根 `build-publish.ps1:59` |

> **2026-10-05：本目录的 Python 运行脚本已全部移除**（`configure_env.py`、`storage_profile.py`、
> `stop_environment.py`、`test_environment_storage.py`）。启动/停止/存储判定全部由仓库内 Go 启动器
> （`server\work\dfo-lan\bin\dfolauncher.exe`）承担；仍保留的 Python 只有 `gm.py`（GM 命令行，
> 它的 `set`/`history` 子命令尚无 Go 等价物）与 `build_publish_zip.py`（打包，属另一条工作）。

### 写 `.cmd` 的硬要求（2026-10-05 实机踩坑后定）

1. **执行行保持纯 ASCII**：中文只放进 `rem`，需要中文提示就让带 BOM 的 `.ps1` 打印。
   cmd.exe 用**控制台代码页**解码批处理文本，`chcp 65001` 只对其后**被重新读取的行**生效；
   UTF-8 中文一旦落在带引号的执行行（`-File "scripts\中文名.ps1"`）就会被拆成乱码命令，
   实测报 `'hell' is not recognized as an internal or external command`。
   中文**文件名**没问题（Explorer / PowerShell 调用时按 UTF-16 处理）——有问题的是文件**内容**里的中文执行行。
2. **CRLF + 不带 BOM**：裸 LF 会让 cmd 按行解析错位、把相邻行粘成一条（同一坑的另一半）；
   BOM 会污染首行 `@echo off`。两者门禁都会报 `[环境不匹配]`。
3. 所以四个路线入口 `启动游戏-{SQLite,PostgreSQL}.cmd`、`启动服务端-{SQLite,PostgreSQL}.cmd`
   只有 8 行 ASCII，路线切换、中文提示、调用中文名统一入口（`启动游戏.cmd` / `启动服务端.cmd`）
   全部在 `storage-route.ps1` 里（UTF-8 带 BOM）。

### 各入口原本写在 `.cmd` 里的中文说明（现集中在此）

`.cmd` 一律纯 ASCII 后，原先写在里面的中文注释与提示按入口搬到这里；
**其余行为（环境变量、委托链、退出码）逐字未改**。

#### `启动游戏.cmd` / `启动游戏-奥德赛.cmd`

- 不再设置 `DFO_ODYSSEY_MODE`：游戏模式按**角色存档投影**（建号请求 `options[10]`，剧情 0 / 奥德赛 2），
  与客户端自己读的 per-character 标记一致。
- 需要整档强制时才用 `启动游戏-奥德赛.cmd`（`DFO_ODYSSEY_MODE=1`），或在启动器设置里选强制档。
- 默认使用 `configs/pvf-default.json`；分别保留剧情 / 奥德赛模式，`--json-mode` 是显式回退。

#### `启动服务端.cmd`（装备库 CMD2259 的三个开关，默认都不设）

- `DFO_EQUIPMENT_CRAFT_WINDOW`：**默认 1** = 窗口 3937。若点「制作/变换」没进制作界面，
  把 `rem set DFO_EQUIPMENT_CRAFT_WINDOW=0` 放开再启动，试窗口 2145。
- `DFO_EQUIPMENT_CRAFT_EXECUTE_ON`：`confirm`（默认，同一次操作的第二次请求）/ `first`（第一次就执行，慎用）/ `never`。
  若点了生成没有任何反应、日志也只有一条 2259，可放开 `rem ... =first`。
- `DFO_EQUIPMENT_CRAFT_GENERATE_VARIANT`：生成应答的子分支字节（`payload[5]`）。**默认 1** = 只落成功标志、
  不动窗口状态；设 0 = 强制 `setState` 到状态 3 —— 那个状态客户端自己不会进也没有出口，进去后
  「切换材料」按钮会失灵（实机 2026-09-29 14:36）。一般不用动。

#### `GM.cmd`

- 中文用法横幅由 `scripts\gm.py help` 打印（Python 按 UTF-8 正确解码）；`GM.cmd` 只保留 ASCII 调度。
- 引擎按 `runtime\storage\local.json` 的 `driver` 自动判定；读取走 `dfo-tool accountlist`（只读、引擎中立），
  写操作走 `cmd/admin` 与 `dfo-tool setlevel`（单事务 + 幂等键 + 审计）；改等级按 PVF 累计经验阈值写
  `experience = Thresholds[level-2]`，不改技能点。

#### `配置环境.cmd`

- 该文件此前是 **GBK 编码**（非 UTF-8，违反 §0.4.2），已重建为 UTF-8 无 BOM 的纯 ASCII 调度；
  中文报告与提示由 `scripts\configure_env.py` 输出。

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

其余入口：`启动游戏.cmd`、`停止游戏环境.cmd`、`GM.cmd`、`storage-route.cmd`，
以及提交门禁 `check-commit-hygiene.ps1`（其余入口正被并行工作整理，按其落定为准）。

> 这些 `.cmd` 一律先 `cd /d "%~dp0.."` 回到仓库根，因此内部路径仍按仓库根书写（根 `AGENTS.md` §0.4.2）。
> 构建脚本 `build-publish.ps1` 仍在**仓库根**（被 `一键打包.cmd` 以 `%~dp0..\build-publish.ps1` 调用）。
