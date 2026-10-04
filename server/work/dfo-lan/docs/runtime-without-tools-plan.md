# 删掉 `tools/` 仍能启动游戏：路线与验收（2026-10-04）

状态：**方案**。业主目标为「把 `tools/` 目录整体移除，仍能启动游戏」（= 目标 B：真正不再需要
Python / PostgreSQL 进程），取代「把依赖改装到系统」的折中方案 A。

上位文档：[mod-sqlite-redis-minimal-plan.md](mod-sqlite-redis-minimal-plan.md)（四目标总交接，§2.3 环境最小化）、
[sqlite-dual-engine-design.md](sqlite-dual-engine-design.md)（W1 的详细设计与 S0–S5）。

---

## 0. 验收判据（唯一）

1. `tools/` 目录**不存在**；
2. 根 `启动游戏.cmd` / `启动服务端.cmd` / `停止游戏环境.cmd` / `配置环境.cmd` 仍能正常完成各自动作；
3. 除**客户端目录**、**内层 PVF**（`server/work/client-build/Script.inner.pvf`）、**服务端二进制**与
   **配置文件**外，运行机不再需要任何外部进程或语言运行时；
4. 玩家存档无损（`pgdata` → 迁移后的 SQLite 文件，见 W1-S4 的转换工具与校验）。

---

## 1. 现状依赖图（已逐条核实）

```
启动游戏.cmd:21-22 ─→ tools\python\python.exe  …………………………………… 【tools/python】
                      └─ scripts\launch_local.py
                         ├─ :14 → prepare_inner_pvf.py:13 → pvf_archive.py:5-8
                         │        └─ import pefile / cryptography   ← 模块级硬导入（非标准库）
                         ├─ :45-56 读 server\launcher.local.json + runtime\storage\local.json
                         ├─ :63-92 start_storage：pg_ctl -D <postgres_data> … start
                         │        ├─ :70 pg_ctl.exe 来自 cfg["postgres_bin"] ⟵ local.json:4
                         │        │   = tools\pg\pgsql\bin ……………………………………………… 【tools/pg】
                         │        └─ :66-69 / :71 拒绝外部数据目录、要求 PG_VERSION 存在
                         ├─ :194 helper = server\work\dfo_probe_tools\channel_probe.py  ← 在仓库内
                         │        └─ :581 起 bin\wireprobe-pvf.exe（DFO_SERVER_BINARY 可覆盖）
                         │           :583-592 轮询 runtime\<tag>\ready.json（3600/1000 × 50ms）
                         │           :596 写 run.json {server_pid, port}
                         │           :614-630 起 probe.exe ← 在仓库内（645 KB，原生）
                         └─ 起客户端 client_dir\DFO.exe（仓库外）
配置环境.cmd:11-12 ─→ tools\python\python.exe scripts\configure_env.py
                         └─ :266 把 postgres_bin 写死为 <root>\tools\pg\pgsql\bin
                            :27-39 用「同时存在 server/ 与 tools/」当仓库根哨兵（5 处）
停止游戏环境.cmd:6-7 ─→ tools\python\python.exe scripts\stop_environment.py
                         └─ 读 cfg["postgres_bin"] → pg_ctl stop（同样依赖【tools/pg】）
build-publish.ps1:41-46,57-58,67 ─→ 打包时复制 tools\{python,pg}，用 tools\python\python.exe 压缩，
                                     并在清单里断言 tools/python/python.exe 与 tools/pg/pgsql/bin/initdb.exe
```

| tools 项 | 体积（清理后） | 启动必需 | 由谁消除 |
| --- | --- | --- | --- |
| `pg` | 919.8 MB | 是（`pg_ctl` 起库） | **W1**（SQLite 落地） |
| `python` | 288.7 MB | 是（5 个根 `.cmd`） | **W2**（编排 Go 化） |
| `go` | 224.4 MB | 否（仅构建） | 构建迁出便携工具链后（非运行依赖） |
| **合计** | **1,433 MB** | | W1+W2 全部落地后 `tools/` 可整体删除 |

> 已在本会话清理（仅本地磁盘、`/tools/` 本就 gitignore）：`redis` 46.1 MB、`gocache` 6,458.8 MB、
> `gopath` 1,959.6 MB、`gm-tool` 125 MB —— 共约 8.6 GB，`tools/` 从 10,022 MB 降到 1,433 MB。

---

## 2. 两个工作流

### W1：去掉 PostgreSQL（已设计，见 sqlite-dual-engine-design.md）

| 阶段 | 内容 | 消除的 tools 依赖 |
| --- | --- | --- |
| S2 | `*database.Tx` 能力面定稿；`storageError` 接纳 `sql.ErrNoRows` | — |
| S3 | sqlc 双引擎 + `querySet` 接口 + SQLite schema/queries 分叉（A 路径已由 S0 spike 判定） | — |
| S4 | SQLite 引擎接入 + PG→SQLite 单向转换 + 启动链按 `driver` 分叉 | `pg_ctl` 起库不再是默认路径 |
| S5 | 契约套件双跑 + 实机由业主验收 | **`tools/pg` 可删**（919.8 MB）；`bootstrap_local.py` 整文件作废 |

### W2：去掉 Python（本文档主责，规格产出中）

把启动编排搬进 Go。**必须一起搬**（`sqlite-dual-engine-design.md` §5.4 已定调）：
`launch_local.py`（294 行）+ `channel_probe.py`（~700 行）+ `bootstrap_local.py` + `stop_environment.py` +
`configure_env.py`；`probe.exe` 是**原生二进制、已在仓库内**，保留不动。

设计要点（待 W2 规格定稿后细化）：

| 项 | 设计 |
| --- | --- |
| 入口 | 新增 Go 可执行文件（建议 `cmd/launcher` → `bin/dfo-launch.exe`）。注意 `server/AGENTS.md` 的「cmd 入口收敛为 4 个」针对**工具**；启动器与 `wireprobe` 同类，是用户可见入口，且该收敛记录明确「游戏启动器及 GM 构建路径保持」 |
| 根 `.cmd` | 改为调用 `bin\dfo-launch.exe`（保留 `else` 回退链：先仓内 exe，再 PATH） |
| 存储启动 | W1 完成后**无需**起任何进程；W1 完成前仍需 `pg_ctl`（这也是建议 W1 先做的原因，见 §3） |
| 配置/环境 | 复刻 `launch_local.py:95-160` 的 profile 解析、binary 选择、`DFO_*` 合并/剥离规则 |
| 网关/探针编排 | 复刻 `channel_probe.py` 的 argv 构造（`-character-storage`、profile 环境）、`ready.json` 轮询（3600/1000 × 50ms）、`run.json` 两处写入、`probe.exe` 生命周期与安全软件失败分支 |
| **通道夹具** | `channelinfo.bin` 必须**逐字节**复刻（varint/integer/string 编码、protobuf 字段号、XOR/rotate 混淆、CRC fold、header 结构）。这是 W2 最高风险项，Go 侧需要**字节级对照测试**（用现 Python 产物做黄金样本） |
| PVF 准备 | `prepare_inner_pvf` 依赖 `pefile`+`cryptography`（PE 解析 + RSA/AES 解包）。**分期**：① 首版 Go 启动器要求内层 PVF 与清单已存在，缺失时给明确指引并保留 Python 一次性工具；② 后续把 `pvf_archive` 的解包逻辑 Go 化（或让内容工具负责） |

---

## 3. 顺序（建议 W1 → W2）与理由

1. **先 W1 后 W2**：W1 完成前，Go 启动器仍要写一遍 `pg_ctl` 起库、`PG_VERSION` 校验、"拒绝外部数据目录"等逻辑，
   而这些在 SQLite 落地后会**整段删除**。反过来不成立——`channel_probe` 的通道夹具与探针编排与存储引擎无关，做在前面不会白做。
2. **W1 已设计且已验证**：S0 spike 已判定走 A 路径（sqlc 双引擎可行、`emit_pointers_for_null_types` 对 SQLite 生效、
   列级 override 生效），S1 已收口；W2 目前只有本文档的骨架，需要先出规格（产出中）。
3. W2 的三处**预备改动**可提前做（不改架构、互不阻塞）：
   - `configure_env.py`：把「`server/`+`tools/`」哨兵换成中性标志（如 `AGENTS.md`），
     并把 `:266` 的 `postgres_bin` 改成可配置/可探测；
   - `prepare_inner_pvf` 的导入改**懒加载**，使内层 PVF 已存在时不再硬要 `pefile`/`cryptography`
     （这条直接让"系统 Python 也能起游戏"成立）；
   - `build-publish.ps1:67` 的清单断言改成可配置，否则打包会一直要求 `tools/` 存在。

---

## 4. 每阶段门禁

- 每个阶段：`go test -count=1 ./...` + `go vet ./...` 通过；失败集合与基线一致（当前基线：仅 2 个既有
  `cmd/wireprobe` primer-transform 失败）；
- W1 每阶段：契约套件在**两个引擎**上同批通过（S5 起）；
- W2 每阶段：**行为对照**——同一场景下 Go 启动器与 Python 编排产生相同的进程序列、相同的 `ready.json`/`run.json`
  内容（除 PID/时间戳）、相同的 `channelinfo.bin` 字节；
- 收口：按 §0 四条判据验收，实机由业主操作。

---

## 5. 剩余风险

| # | 风险 | 说明 |
| --- | --- | --- |
| R1 | `channelinfo.bin` 字节级复刻 | 混淆 + CRC fold + protobuf-ish 编码；错一位 `probe.exe` 就不工作。缓解：黄金样本字节对照测试 |
| R2 | 轮询/超时语义漂移 | 3600/1000 × 50ms、pg_ctl `-t 30` timeout 40、探针退出码语义 |
| R3 | profile 环境合并规则 | `launch_environment` 里 `DFO_SKILL_RELEASE`/`DFO_ODYSSEY_REWARDS_PILOT` 的条件移除 |
| R4 | 路径语义 | `ROOT` 在 `launch_local.py` 里是 `server/`（不是仓库根），`resolved()` 以其为基准 |
| R5 | 编码/BOM | JSON 以 `utf-8-sig` 读取（容忍 BOM）；控制台输出 UTF-8 重配置 |
| R6 | 进程细节 | `CREATE_NO_WINDOW`、日志重定向、杀进程清单、客户端启动方式 |
| R7 | 打包链 | `build-publish.ps1` 复制 `tools/{python,pg}` 与清单断言 |

---

## 6. W2 规格（已产出）

见 **[python-orchestration-port-spec.md](python-orchestration-port-spec.md)**：`launch_local.py` 与
`channel_probe.py` 的逐项可观察契约（CLI/配置/进程端口编排/超时/环境变量/产物/失败文案），
**`channelinfo.bin` 的逐字节布局**（varint+protobuf 式编码、`rol2(x^0xB5)` 混淆、非 IEEE 的
`crc32.MakeTable(0x4DB89129)` + 折叠 `^0x18`、16 字节 header），`probe.exe` 的 argv 与退出码语义，
`ready.json` 契约，以及按严重度排序的 **22 条移植风险**。

**验收纪律**：规格中所有字节级/时序级断言，Go 实现必须用现 Python 产物做**黄金样本实测对照**后才可下结论
（根 `AGENTS.md` §0.3 不猜包）；不符则改规格，不改测试迁就实现。

## 实测：`tools/python` 退出前必须迁走的编排面（2026-10-04）

按"运行期是否被 `.cmd` 入口或启动器实际调用"清点，**运行期关键**的 Python 共约 1,900 行，
其余为测试与离线导出（可留在 Python，或随工具一起退休）：

| 脚本 | 行数 | 角色 | 迁走优先级 |
| --- | --- | --- | --- |
| `server/work/dfo_probe_tools/channel_probe.py` | **716** | 探针：7001 频道目录 + 连接引导，**独立进程**，启动器会拉起它 | 最高（也是最大风险） |
| `server/work/dfo-lan/scripts/launch_local.py` | 312 | 编排：解析路径、起存储、准备 PVF、拉起探针/服务/客户端 | 高（W2 的主干） |
| `scripts/configure_env.py`（仓库根） | 311 | 配置环境（`配置环境.cmd`） | 中 |
| `server/work/dfo-lan/scripts/ensure_inner_pvf.py` | 195 | 内层 PVF 准备 | 高 |
| `scripts/stop_environment.py`（仓库根） | 132 | 停止环境（已按 driver 分叉） | 中（自包含，适合先做样板） |
| `pvf_archive.py` / `profile_pvf_startup.py` / `repair_profile.py` / `prepare_inner_pvf.py` / `pvf_rule_fields.py` | 119/112/91/93/90 | PVF 读取与 profile 支持 | 高（被 launch_local 间接依赖） |
| `bootstrap_local.py` | 45 | 首次初始化存储 | 低（一次性） |
| `test_*.py`（9 个）/ `export_*.py`（3 个）/ `audit_config_references.py` | 769/356/91 | 测试与离线导出 | 不需要（不属运行期） |

**`.cmd` 入口现状**（迁移后要改成调用 Go 程序）：`启动游戏.cmd`、`启动服务端.cmd`、
`启动游戏-奥德赛.cmd` 都调用 `tools\python\python.exe server\work\dfo-lan\scripts\launch_local.py %*`；
`停止游戏环境.cmd` → `scripts\stop_environment.py`；`配置环境.cmd` → `scripts\configure_env.py`。

**建议的落地形态**：构建一个 Go 启动器（与 `wireprobe` 一起由 `Build-Server.ps1` 产出），
子命令对应现有脚本（`launch`、`server-only`、`stop`、`configure`、`prepare-pvf`），
`.cmd` 改为"存在 Go 启动器就用它，否则回退 Python"。这样迁移期间两套并存、逐段替换、随时可回退。

**建议顺序**（每步都能独立验证，且不阻塞游戏可玩）：
1. `stop_environment.py` → `launcher stop`（自包含、依赖最少，用来定型 Go 启动器的骨架与日志风格）。
2. `launch_local.py` 的**路径解析 + 存储启动 + 依赖检查**（`--check`）先行；此时启动器已能"只检查不启动"。
3. `ensure_inner_pvf.py` + `pvf_archive.py` + `prepare_inner_pvf.py` + `repair_profile.py`（PVF 准备链）——
   服务端已有 Go 侧 PVF 读取能力，可复用而不是重写。
4. `channel_probe.py`（716 行，独立进程、涉及 7001 与连接引导）——**最后做**，且必须先逐字段对照现有
   探针行为（它是实机连接的关键路径，不能靠猜）。
5. `configure_env.py` → `launcher configure`；随后 `.cmd` 去掉 Python 分支，`tools/python` 才可删。

**与双引擎的关系**：`tools/pg` 的删除条件已接近满足（服务端可跑 SQLite、可转换存档、启动器与停止脚本
都已按 driver 分叉），只差业主在 SQLite 上实机跑一次；`tools/python` 则完全取决于本节 1–5 步。

## 关键发现：Go 启动器**已经存在**，它目前在**驱动 Python**（2026-10-04）

清点 `tools/python` 的退出条件时发现相邻项目 `C:\Game\dof\115us\115us-dfolauncher`
（`module dfolauncher`，Go 1.26，含已构建的 `DFO-115US单机一键启动器.exe`）已经是一个完整的 Go 启动器：

| 已由 Go 拥有 | 证据 |
| --- | --- |
| 会话编排骨架 | `internal/run/run.go` **1310 行** + 516 行测试；`run/probe.go` |
| 内层 PVF 准备 | `internal/pvfprep/pvfprep.go` **411 行** + 332 行测试 |
| 存储启动 | `internal/storeboot/storeboot.go` 299 行、`pgpassword.go` 531 行 |
| 进程/端口/服务/环境检查/自更新/GM 桥 | `internal/proc`、`ports`、`svc`、`envfix`、`selfupdate`、`gmbridge`、`check` |
| 图形界面 | `internal/appui`（go-webview2） |

**但它目前仍依赖 Python 完成实际编排**，证据在它自己的 `internal/check/check.go`：
- 第 114–116 行把 `tools/python/python.exe` 列为**错误级**必需项，文案为
  「游戏会话编排依赖便携版 Python（tools/python）」；
- 第 199–201 行把 `server/work/dfo-lan/scripts/launch_local.py`、
  `server/work/dfo_probe_tools/channel_probe.py`、`stop_environment.py` 列为编排入口。

**这对 W2 的意义（计划据此修正）**：
1. 我先前按"1,900 行全都要重写"估算是偏高的。真正还留在 Python 侧的**只有三件**：
   ① `launch_local.py` 的会话编排（起探针+服务+客户端、等网关、注入环境）、
   ② `channel_probe.py`（探针本体）、③ `configure_env.py`。
   内层 PVF 准备（`ensure_inner_pvf.py` 195 行）**在 Go 侧已有等价实现**（`pvfprep`），
   应当**复用而不是重写**——这也正是我在 `profile.go` 里只移植结构部分、并标注"逐键校验未移植"的原因：
   逐键校验的权威实现就在 `pvfprep` 附近，迁移时应以它为准并对齐。
2. 我已在 `dfo-lan` 模块内落地 `cmd/dfolauncher`（`stop`/`check`）。它与相邻项目的 `check`/`storeboot`
   **职责重叠**，后续必须二选一并明确归属：要么把 `dfo-lan` 侧的 `dfolauncher` 并入相邻项目，
   要么让相邻项目改为调用 `dfo-lan` 的 Go 实现。**在本仓库内重复实现第二套启动器是明确的浪费**，
   记录在此以免继续走偏。
3. **跨项目收口条件**：Python 侧真正迁空之后，相邻项目的 `internal/check/check.go` 必须同步删除
   `runtime.python` 这条错误级检查与那三个脚本入口，否则启动器仍会坚持要求 `tools/python`。
   这一步不在本仓库内，需要与那边的改动一起做。

## 完整依赖面（2026-10-04 扫过全部 81 个入口文件，这是最终清单）

除下列位置外，仓库内**没有任何**入口再依赖 `tools/python` 或 `tools/pg`。

### A. 仍依赖 `tools/python` 的入口（决定 `tools/python` 能否删）

| 入口 | 调用的 Python | 状态 |
| --- | --- | --- |
| `启动游戏.cmd:21-22` | `launch_local.py` | 待迁（W2 第 5 步的主体） |
| `启动服务端.cmd:22-23` | `launch_local.py --server-only` | 待迁（同上，同一份脚本） |
| `启动游戏-奥德赛.cmd:20-21` | `launch_local.py` | 待迁（同上） |
| `停止游戏环境.cmd:6-7` | `scripts/stop_environment.py` | **Go 侧已就绪**（`cmd/dfolauncher stop`，含 `--dry-run`），只差按 A/B 决定接入哪个启动器 |
| `配置环境.cmd:11-12` | `scripts/configure_env.py`（311 行） | 待迁（W2 第 5 步） |
| `build-publish.ps1:57-58` | `tools/python` 跑 `scripts/build_publish_zip.py` | 待迁（构建期依赖；可改为 PowerShell 自身打包或 Go 工具） |
| `gm-tool/GM管理台.cmd:13`、`gm-tool/Start-GMWeb.cmd:5-8`、`gm-tool/dashboard/start_gm_dashboard.ps1:7-8` | `gmweb.py`（并回退到 `gm-tool/python/python.exe`） | 业主此前已确认 GM 工具"不用，可以删"；这三处随 GM 目录一并处理，不属 W2 |

### B. 仍依赖 `tools/pg` 的入口（决定 `tools/pg` 能否删）

| 位置 | 内容 | 状态 |
| --- | --- | --- |
| `build-publish.ps1:48` | 打包时剔除 `tools\pg\pgsql\pgAdmin 4` | 已在"若不存在则跳过"的语义下无害 |
| `build-publish.ps1:71` | 最终校验要求 `initdb.exe` | **已改为按实际打包内容决定**（若 `tools/pg` 不存在则不再要求） |
| `scripts/configure_env.py:266` | 把 `tools/pg/pgsql/bin` 写进 `postgres_bin` | 待改：sqlite 模式下不应写该字段；随 `configure_env.py` 迁 Go 时一并处理 |
| `server/work/dfo-lan/runtime/storage/local.example.json:4` | 模板示例含 `"postgres_bin": "tools/pg/pgsql/bin"` | 模板可保留（postgres 档仍需要）；sqlite 档示例应另有 `driver/sqlite_path` |
| 文档若干（`使用教程.md`、`docs/*.md`） | 使用说明中的路径与备份命令 | 文档更新，不阻塞删除 |

### 结论

`tools/pg` 的**代码侧**依赖已清空（打包校验已分叉、服务端可跑 SQLite、启动器与停止脚本按 driver 分叉、
转换器可迁移存档），仅剩文档与 `configure_env.py` 一处；**唯二的阻塞是业主侧动作**：
① SQLite 实机跑一次；② 若采用 SQLite 档，`configure_env.py` 那处随之调整。
`tools/python` 则明确卡在 A 表前三行（`launch_local.py` 的三个入口）与 `build_publish_zip.py`。

## 决定性证据：Go 启动器**运行时已经不需要 Python**（2026-10-04 实测）

上一节记录了"相邻项目已有完整 Go 启动器、但仍依赖 Python"，本轮把**依赖的性质**查清了，
结论比先前乐观得多：

**它启动的进程里没有任何 Python。** 扫过 `115us-dfolauncher` 全部非测试 Go 文件的
`exec.Command` / `exec.CommandContext`，只有这些：
`explorer.exe`（在资源管理器中定位文件）、`rundll32`/`open`/`xdg-open`（打开链接）、
`pg_ctl`（`gm/cmd/gmtool/storage.go:84`，自己启动 PostgreSQL）、
`proc.Hide(exec.Command(exe, …))`（通用进程启动器，`exe` 是服务端/探针二进制）、
`internal/run/run.go:815`（启动**客户端** `DFO.exe` 并带上网关端口）、`internal/selfupdate`（自更新后重启自己）。
**没有一处调用 `tools/python/python.exe`，也没有一处调用任何 `.py`。**

而对 `launch_local.py` / `channel_probe.py` / `stop_environment.py` / `configure_env.py` 的引用，
逐条看下来**全是注释、镜像表与存在性检查**，不是调用：
- `internal/check/compat.go` 自述是「`channel_probe.py` 里"按 configs 文件存在性追加参数"的**镜像表**」——
  也就是说探针的参数拼接逻辑**已经在 Go 侧重写过一遍**；
- `internal/config/config.go` 复刻 `launch_local.py` 的 `ROOT`/相对路径基准（与我本轮在
  `internal/launcher` 里做的事相同）；
- `internal/run/run.go`（1310 行）是真正的编排主体。

### 因此 W2 的真实剩余量远小于先前的 1,900 行估算

`tools/python` 的删除条件收敛为**两件小事**，且都在相邻项目里：
1. `internal/check/check.go:114-116` 目前把 `tools\python\python.exe` 列为 **LevelError**
   （文案「游戏会话编排依赖便携版 Python（tools/python）」）。运行时既然不需要它，这条检查就要
   降级或删除，否则**删掉 `tools/python` 后启动器自己会拒绝启动**——这是唯一真正的阻塞。
2. `check.go:199-202` 把四个 `.py` 入口列为待检路径；它们是历史/参考物，应改为非必需
   （保留为"可选存在"以便诊断，不作为错误）。
   本仓库内的 `.cmd` 入口（`启动游戏.cmd` 等）仍调 `launch_local.py`，但**发布版走的是 Go 启动器**
   （`DFO-115US单机一键启动器.exe`），所以这些 `.cmd` 是开发/源码路径，不构成发布阻塞。

### 方向问题的答案（据证据，而非偏好）

先前提出的 A/B 之争，实测证据指向 **A 已经是既成事实**：发布用启动器是 Go、运行时零 Python 调用，
Python 侧只被**检查**而非**执行**。所以"把启动编排迁到 Go"这项工作在相邻项目里**基本已经完成**，
剩下的不是移植，而是**清理那两处陈旧的 Python 必需性检查**。
本仓库内我写的 `cmd/dfolauncher`（`stop`/`check`/profile 加载）因此**不该继续扩张**；
它的价值在于：① 证明这些语义可以在本仓库内独立成立；② 提供 `stop` 的 Go 实现，供 `.cmd` 在
需要时切换。是否保留它，取决于业主是否希望本仓库自带启动能力。

## 修正（2026-10-04，紧接上节）：发布版走 GUI，**CLI 编排仍无 Go 实现**

上节说"本仓库内我写的 `cmd/dfolauncher` 不该继续扩张"，**这个结论不完整**。本轮查了相邻启动器
`cmd/launcher/main.go` 的命令行面：它**只有 `--gm-window` 一个参数**（第 98 行），其余全部是
WebView2 的 JS 处理器（`app.Handle("check.run", …)`、`app.Handle("update.check", …)` 等），
即**它是一个 GUI 应用，没有 CLI 编排模式**。

于是两条路径必须分开看：

| 路径 | 入口 | 运行时是否需要 Python |
| --- | --- | --- |
| **发布 / 一键启动（GUI）** | `DFO-115US单机一键启动器.exe`（相邻项目） | **不需要**（上节已证实零 Python 调用）；仅被 `check.go:114-116` 的 LevelError 挡住 |
| **开发 / 源码构建（CLI）** | 根目录 `启动游戏.cmd --source-build`、`启动服务端.cmd`、`启动游戏-奥德赛.cmd` | **仍然需要**：这些入口调 `launch_local.py`，而**没有任何 Go CLI 能替代它** |

所以 W2 的剩余工作**确实包含把 `launch_local.py` 的 CLI 编排迁到 Go**，这一点不因 GUI 启动器的存在而消失。
落点有两个选项（仍属方向问题，但范围已明确）：
- **A'**：给相邻启动器加一个 CLI 模式（在 `cmd/launcher` 里按 `--source-build`/`--server-only` 分发）；
- **B'**：在本仓库 `cmd/dfolauncher` 内完成（已有的 `stop`/`check`/profile 加载 + 本轮的存储启动即为地基）。

两者都需要同一批能力：**存储启动 → 探针拉起 → PVF 准备 → 服务端拉起 → 等待网关 → 客户端拉起**。
本轮已在 `internal/launcher/storage.go` 落地第一段（`StartStoragePlan`/`StartStorage`，与 `stop` 同样的
"计划/执行"分离，因此可测试且不会误启服务）。

**修正后的 W2 剩余量**：`launch_local.py` 312 行中的 CLI 编排（存储启动已做；余下探针、PVF 准备、服务端
与客户端拉起、网关等待），以及 `build_publish_zip.py`；`configure_env.py` 511 行中的配置写入部分可参照
相邻项目已有的 `internal/config`/`internal/envfix` 决定是否复用。

## 第四轮修正：`launch_local.py` 自己**不启动服务端与客户端**，它启动的是探针

读完 `launch_local.py` 的收尾（第 244 行之后），编排的真实形状是：

```python
process = subprocess.Popen(
    [sys.executable, str(helper), tag, mode],   # helper = channel_probe.py，用 Python 运行
    cwd=ROOT, env=env, stdin=DEVNULL,
    stdout=<session>/helper.out, stderr=<session>/helper.err, creationflags=FLAGS)
# 然后等 <session>/run.json 出现（PVF 直读 210s，JSON 档 30s），超时或 helper 退出即报错
```

也就是说：

| 组件 | 真正做什么 |
| --- | --- |
| `launch_local.py`（312 行） | 只做**前导**：路径/配置/profile 解析、存储启动、内层 PVF 确保、7001 占用预检，然后**用 `sys.executable` 拉起 `channel_probe.py`**，并把 `DFO_CLIENT_DIR`/`DFO_SERVER_BINARY`/`DFO_CHANNEL_IDENTITY`/`DFO_ENABLE_OBSERVER` 塞进环境；随后只等 `run.json` |
| `channel_probe.py`（**716 行**） | **真正的编排者**：探测/引导连接、拉起服务端与客户端、维护 7001 频道服务、写 `run.json` |

**这把 A' / B' 的成本差拉得很开**：

- **A'（给相邻启动器加 CLI 模式）**：`115us-dfolauncher` 的 `internal/run/run.go`（**1310 行**）
  本来就是 `channel_probe.py` 编排逻辑的 Go 实现（另有 `run/probe.go` 与镜像探针参数逻辑的
  `check/compat.go`），`internal/storeboot` 又覆盖了存储启动。A' 主要是**给已有实现加一个命令行入口**，
  而不是写新逻辑。
- **B'（在本仓库重写）**：等于**再实现一遍 716 行探针编排**，而相邻项目里已经有一份等价的 Go 实现。

**结论（据证据，不含偏好）**：A' 明显更省，且避免同一套编排在仓库之间维护两份。
本仓库已完成的 CLI 前导（路径解析、依赖检查、profile 加载、存储启动、服务端环境构造）**在 A' 下仍有价值**：
它是这套语义的**可执行对照规范**（相邻项目若行为有别，可据此逐条比对），而不是待丢弃的重复品。
但**不应继续在 B' 上往下写探针编排**——那正好落进"同一逻辑维护两份"的坑。

## W2 启动编排：移植清单（2026-10-04 实测 `channel_probe.py`，716 行）

"python 也干掉"的**唯一**剩余阻塞就是这一份编排。它不能靠猜，以下是从源码量出来的形状，
可以直接当移植清单用：

| 组成 | 实测规模 | 说明 |
| --- | --- | --- |
| 环境变量决策 | **14 处普通 + 11 处条件**（共 25 处 `os.environ[...]`） | 形态是"**override 文件存在则用它，否则用默认路径**"：`DFO_ODYSSEY_COIN_RULES`、`DFO_ODYSSEY_WEAPON_BOX`、`DFO_ODYSSEY_GROWTH`、`DFO_ODYSSEY_CHAPTERS`、`DFO_ODYSSEY_CHAPTER_DROP`、`DFO_EQUIPMENT_WEAR_RULES` 各一组；另有 3 处无条件绝对路径：`configs/attunement-rewards.generated.json`、`configs/equipment-journal.generated.json`、`configs/equipment-create-cost.generated.json`。**必须是绝对路径**，因为网关的 cwd 是包根 |
| 服务端命令行 | **只有 1 处 `command.append/extend`** | 参数主要以列表字面量给出，不是逐条拼装；因此"有多少 flag"不能按 append 数估计 |
| **参数裁剪** | `prune_unsupported(command)` + `exe_flags(command[0])` | **版本感知**：先探测该二进制支持哪些参数，再丢掉它不支持的。移植时不能盲目把参数全传，否则旧二进制会拒启 |
| 进程 | **4 处 `subprocess.run/Popen`** | 探针 `probe.exe`（7001）、服务端、（持久化模式下的 `wireprobe-character.exe`）、客户端 |
| 就绪判据 | 会话目录里的 `ready.json`（启动流程）与 `run.json`（`launch_local.py` 侧等待） | 两者都要等，超时口径不同（PVF 直读更长） |
| 顺带事实 | `catalog_startup.py` 只校验**历史 JSON** 目录，PVF 原生域下直接跳过 | 移植时可整体省略，但要注意它存在时不应被误当成必需步骤 |

**已可复用的本仓库实现**：`internal/launcher` 的 `BuildServerEnv`（`launch_environment` 的四条规则）、
`Check`（路径与依赖）、`StartStorage`（存储启动）、`LoadProfile`（profile 与必需文件）、
以及已构建的 `bin/dfolauncher.exe`。

**相邻项目已有的等价 Go 实现**（`115us-dfolauncher`）：`internal/run/run.go`（1310 行，
含 `run/probe.go`）覆盖编排，`internal/check/compat.go` **自述就是 `channel_probe.py` 参数规则的镜像表**，
`internal/storeboot` 覆盖存储启动。**因此 A' 的移植量显著小于 B'**——B' 等于把上面这张表在
两个仓库各维护一份，而 A' 主要是给已有实现加命令行入口并把 `tools\python` 的 LevelError 降级。

## 第五轮实测：启动编排的**命令行装配**有多大，以及为什么 B' 会变成第三份实现

上一节给了移植清单；本轮把最核心的一块量清楚了——**服务端命令行是怎么拼出来的**。
结论直接影响 A'/B' 的取舍，所以单列一节。

`channel_probe.py` 用 **19 个 `command += [...]` 块**（第 227–452 行，约 230 行）逐步拼出服务端命令行，
随后才交给 `prune_unsupported`（这一块**我已移植并验证**）。这 19 块的内容性质是：

- **按文件存在性追加参数**（`if X.exists(): command += [...]`）——例如
  `-select-probe-config`、`-entry-basic-probe`、`-responses`、`-item-shop`、
  `-channel-refresh-config`、`-channel-identity`；
- **按模式分支**：`-game-listen 127.0.0.2:0` 在**两处**出现（不同模式各自设置）；
- **按持久化分支**追加 `-character-storage` / `-character-catalog` / `-character-rules`；
- 另有第二条命令（角色检查：`wireprobe[-character].exe -fixture … -output …`）。

**关键判断：这套"文件存在 → 追加参数"的规则在 Go 里已经有一份实现。**
相邻项目 `115us-dfolauncher/internal/check/compat.go` 自述就是
「`channel_probe.py` 里"按 configs 文件存在性追加参数"的**镜像表**」，并写明维护要求是
"上游 `channel_probe.py` 新增一处'文件存在就追加 -xxx'时，必须同步加到这里"。
也就是说：**B' 等于在 `dfo-lan` 里写出第三份同样的规则表**（Python 一份、相邻项目一份、本仓库一份），
而三者必须在每次上游改动时同步——这正是根 `AGENTS.md` §0 反复禁止的"平行规则表"。

因此本节的结论与建议：
- **已完成且可复用的本仓库件**（都是独立可验证、且对 A' 有对照价值）：
  存储启动、profile 加载与必需文件、依赖检查、**启动环境四条规则**（`BuildServerEnv`）、
  **探针的 9 条环境决策**（`ProbeEnvironment`，带"重新解析 Python 源码"的同步门禁）、
  **参数裁剪与 PVF 支持门禁**（`ExeFlags`/`PruneCommand`，对真实 `wireprobe-pvf.exe` 验证过）。
- **不建议继续 B' 的部分**：那 19 块命令行装配。它既大（~230 行）又与相邻项目已有的 Go 镜像表重复，
  且我无法在本会话内把它移植到"可交给业主实机验证"的程度——**半成品启动器比不启动更糟**，
  业主要验证的是"游戏能不能进"，一个参数缺失的启动器会让验证失败在与双引擎无关的原因上。
- **建议 A'**：给相邻项目加 CLI 入口（复用它的 `run.go` + `compat.go` + `storeboot`），
  并把 `internal/check/check.go:114-116` 的 `tools\python` 从 LevelError 降级。
  本仓库已完成的这些件在 A' 下是**对照规范**：相邻项目若有行为差异，可据此逐条比对。
