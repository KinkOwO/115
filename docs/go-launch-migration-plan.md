# 启动链去 portable 依赖（去 Python / 去 Go 工具链 / 去 PostgreSQL 便携运行时）

> 业主决策（2026-10-05）：「单纯靠一个启动器就能把服务端下载下来启动」→ 选**方案 1**（Go 编排接管），
> 并追加「**tools 也不要**」。即：启动游戏不再需要 `tools/python`、`tools/go`、`tools/pg` 任何一个。

## 1. 目标（可验收的定义）

在**没有任何 `tools/` 目录**、**没有任何系统 Python**、**没有便携 PostgreSQL** 的机器上，
只用「启动器 + 整合包（含预编译 `bin/wireprobe-pvf.exe` + 客户端）」就能：

1. 选客户端目录 → 生成/复用内层 PVF → 起存储（SQLite）→ 起网关 → 拉起客户端 → 能进游戏；
2. 停止时能干净收尾（服务端 / 客户端 / 存储）；
3. 全流程**不出现** `python.exe`、`go.exe`、`pg_ctl.exe`。

## 2. 现状：启动链上的 portable 依赖

| 环节 | 现在由谁做 | 规模 | 替代方案 |
|---|---|---|---|
| 配置解析 + 存储启动 + 环境拼装 + 起 helper | `scripts/launch_local.py` | 312 行 | `dfolauncher launch`（Go），存储复用已有 `start-storage` |
| 会话编排本体：夹具构造 / 网关参数 / 客户端拉起 / run.json | `dfo_probe_tools/channel_probe.py` | **721 行** | `internal/launcher` 内新增 Go 实现（协议夹具需按原样复刻） |
| 内层 PVF 四态门禁 | `scripts/ensure_inner_pvf.py` | 195 行 | 启动器 `internal/pvfprep` 已有同语义 Go 实现 |
| 内层 PVF 真身（解密 + 重打包 + 清单） | `scripts/prepare_inner_pvf.py` | 93 行 | **服务端 `internal/catalog/pvf` 已支持内层格式**（`FormatDFO20260901` + `rewrite.go`/`crypto.go`/`string_pool.go`）→ Go 侧直接生成 |
| 启动目录 | `dfo_probe_tools/catalog_startup.py` | 29 行 | 同上，Go 侧生成 |
| 首次初始化 | `scripts/bootstrap_local.py` | 45 行 | Go（SQLite 建库 + 写 local.json） |
| 停止 | `scripts/stop_environment.py` | 132 行 | `dfolauncher stop` **已存在** |
| 配置向导（非启动必需） | `scripts/configure_env.py` | 340 行 | 可留 Python（不挡启动）或后续 Go 化 |
| WFP 网络隔离 | `dfo_probe_tools/probe.exe`（C++，`probe.cpp` 22 KB） | 0.6 MB | **本身不是 Python**：先由 Go 直接调用；后续可选 Go 化（fwpuclnt） |
| 服务端程序来源 | 本机用 `tools/go` 就地编译 | — | 改用**预编译产物**：包内 `bin/` 或清单包（下载） |
| 存储 | PostgreSQL（`tools/pg`）或 SQLite | — | **SQLite**（`driver: sqlite` 已是本机现状；PG 仅作为可选） |

## 3. 已有可复用的 Go 资产（不用从零写）

- 服务端 `internal/catalog/pvf`：PVF 解析/重打包/字符串池/加密/内层归档格式（≈5000 行）。
- 服务端 `internal/launcher`：`cmd/dfolauncher` 的现有实现（`stop` / `check` / `start-storage`）。
- 服务端 `internal/database`：SQLite 双引擎已落地。
- 启动器 `internal/pvfprep`：内层 PVF 四态门禁（Go）+ 清单结构。
- 启动器 `internal/run`：会话状态机、进程管理、超时与停止兜底。
- 启动器 `internal/toolpath`：运行环境目录解析（已让 tools 可缺席）。

## 4. 分期实施（每期都必须：跑 `scripts/check-commit-hygiene.ps1` → `go build/vet/test` → 提交）

### Stage 1 · `dfolauncher launch --check` / `--dry-run`（Go 骨架）
把 `launch_local.py` 的**配置解析 + profile 校验 + 依赖校验 + 计划输出**用 Go 重写：
- 读 `server/launcher.local.json`（`client_dir` / `channel_identity` / `server_binary`）与
  `runtime/storage/local.json`（`driver` / `sqlite_path` / PG 三件套）；
- 移植 `repair_profile.load_profile`（PATH_KEYS / FLAGS / `DFO_PVF_CATALOGS` 白名单 55 域 /
  `DFO_PVF_SHA256` / `DFO_ISPINS_MODE` / 诊断键等），默认档 `configs/pvf-default.json`；
- 校验 required 文件（网关程序、客户端三件套、inner PVF、policies），`--check` 逐项打印；
  `--dry-run` 额外打印"存储 → 内层 PVF → 网关 → 客户端"的命令计划。
- **验收**：与 `python scripts/launch_local.py --check` 的输出逐项一致（Storage / Binary / Data mode / Client），
  且**不启动任何进程**。

### Stage 2 · `dfolauncher launch --server-only`（真起服务端）
- Go 版协议夹具（复刻 `channel_probe.py` L35–L100：varint/字符串/自定义异或位移/CRC/fold/header）；
- 起网关（参数表来自 profile + configs，含 `exe -h` 能力过滤 `prune_unsupported`）；
- 写 `run.json`（`server_pid`、日志路径）并与启动器现有的解析格式保持一致。
- **验收**：`--server-only` 起服务端，7 001 可连，`run.json` 与 Python 版字段一致。

### Stage 3 · 客户端拉起 + WFP 隔离
- Go 拉客户端（参数与 Python 版逐项对齐）；WFP 先调用现有 `probe.exe`，再评估 Go 化。
- **验收**：能进游戏；断网环境下客户端只走 127.0.0.1。

### Stage 4 · 内层 PVF 改 Go 生成
- 服务端侧新增 `dfolauncher prepare-inner-pvf`（用 `internal/catalog/pvf`），替代
  `ensure_inner_pvf.py` + `prepare_inner_pvf.py`；清单结构与启动器 `internal/pvfprep` 对齐。
- **验收**：`client-build/Script.inner.pvf` 与 Python 版产物**逐字节一致**（或哈希一致）。
- 完成后启动器 `internal/pvfprep` 改调它，删掉对 `tools/python` 的引用。

### Stage 5 · 启动器接线（默认走 Go）
- `internal/run`：优先 `dfolauncher launch`，仅在缺少该子命令时回退 Python 路径；
- 回退路径保留一个版本周期，确认稳定后删除 Python 调用与相关文案。

### Stage 6 · 收尾
- 清单/文档清理：`tools/python`、`tools/pg`（SQLite 档）、`tools/go` 不再列为必需；
- 服务端程序走预编译包（新清单项 `server-bin`），`server-src` 仅作开发者可选；
- `移除tools.cmd` 的既有语义（"移走 tools 仍能启动游戏"）达成。

## 5. 风险与回退

- **协议夹具必须逐字节一致**：Stage 2 起，Go 版夹具与 Python 版产物做字节对比；不一致就不切换。
- **并发**：本仓库同时可能有其它会话在改（历史上出现过 amend/rewrite）。每期只暂存本期文件。
- **回退**：Stage 5 之前，Python 路径始终保留；任何一期失败都可以直接用 `.cmd` 走回老路。
- **WFP**：`probe.exe` 无管理员权限时会优雅降级 —— 保留该行为，不在 Go 版里改变语义。
