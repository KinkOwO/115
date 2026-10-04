# scripts/ — 仓库级脚本

本目录存放**仓库级脚本**（管环境、启动、打包、停止、提交门禁），与各组件自带的脚本目录分开。
`2026-10-04` 从仓库根目录收拢到这里；同日晚些时候**根目录的运行/维护 `.cmd` 也全部移入本目录**
（根目录不再有任何 `.cmd`，见根 `AGENTS.md` §0.4.1）。

## 文件

| 文件 | 作用 | 谁调用 |
| --- | --- | --- |
| `check-commit-hygiene.ps1` | **提交前门禁**：检出「本地缓存/构建产物入库」与目录规范违规；退出码 2 = 需业主二次确认（根 `AGENTS.md` §0.3.1） | 任何提交前手动跑：`pwsh -NoProfile -File scripts/check-commit-hygiene.ps1` |
| `configure_env.py` | 配置本机环境（写 `server/work/dfo-lan/runtime/storage/local.json`、探测客户端目录） | `scripts\配置环境.cmd` |
| `stop_environment.py` | 安全停止环境（PG `pg_ctl stop -m fast` 做 checkpoint、清理进程与端口） | `scripts\停止游戏环境.cmd` |
| `build_publish_zip.py` | 打发布包 | 根 `build-publish.ps1:59` |
| `test_environment_storage.py` | 上面两个脚本的单元测试（用 tempfile + mock，不碰真实环境） | 手动：`python scripts/test_environment_storage.py` |

两个脚本都用 `__file__` 定位仓库根（`stop_environment.py` 取上一级，
`configure_env.py` 逐级向上找同时含 `server/` 与 `tools/` 的目录），因此**移动本目录层级时必须同步改这两处**。

## 子目录

- `local-fixes/` —— 仓库级的一次性修复/移植脚本（2026-10-03 入库，内含 README）。
- `incremental-package/` —— **另一条并行工作**在建的增量打包子目录，不属于本次整理范围，请勿改动。
## 不在这里的东西（有意为之）

- **JSON 没有收拢**。仓库里的 JSON 是三类完全不同的东西，混在一起会破坏既有契约：
  1. **内容基线**：`server/work/dfo-lan/configs/*.json` —— 根 `AGENTS.md` §0.2 规定 **PVF 是唯一内容真源**，
     这些只是历史对照/审计用途，**不是运行输入**；
  2. **运行配置（路径被写死）**：`server/work/dfo-lan/runtime/storage/local.json`、
     `server/launcher.local.json`、`server/work/dfo-lan/configs/pvf-default.json`
     —— 被 `launch_local.py`、`configure_env.py`、`channel_probe.py` 按固定路径读写；
  3. `gm-tool/configs/` 是 GM 工具自己的配置。
- **组件自带脚本目录**保持原位，它们各自被本组件的构建/启动链按路径引用：
  - `server/work/dfo-lan/scripts/`（服务端编排：`launch_local.py`、`bootstrap_local.py`、`Generate-SQL.ps1`、`test_postgres_storage.py` 等）
  - `server/work/dfo_probe_tools/`（启动链必需：`channel_probe.py` + `probe.exe`）
  - `gm-tool/scripts/`

- `incremental-package/` 是**另一条并行工作**的在建子目录，不属于本目录的整理范围，请勿改动。

## 运行入口（全部在本目录）

`启动游戏.cmd`、`启动服务端.cmd`、`启动游戏-奥德赛.cmd`、`停止游戏环境.cmd`、`检查环境.cmd`、
`配置环境.cmd`、`移除tools.cmd`/`还原tools.cmd`、`一键打包.cmd`、`打包增量更新.cmd`、`伊斯-*.cmd`，
以及提交门禁 `check-commit-hygiene.ps1`。

> 这些 `.cmd` 一律先 `cd /d "%~dp0.."` 回到仓库根，因此内部路径仍按仓库根书写（根 `AGENTS.md` §0.4.2）。
> 构建脚本 `build-publish.ps1` 仍在**仓库根**（被 `一键打包.cmd` 以 `%~dp0..\build-publish.ps1` 调用）。
