# SQLite 启动链与本机 mod 编译：确认基线

## 后续上游更新与停止入口收口

业主本轮确认：“确认能启动了，可以提交”。启动确认对应 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261011_233347_523876_next37/`，源码基线为 `dea70aff`，不是下文早期候选的身份。

- `232453_156659_next37/gateway.err` 的最终原因是礼包 mod 声明未注册，并非 SQLite 打开失败。更新后 mod 源码和清单已恢复，但原候选仍为空加载器。使用现有安装 overlay 重建候选；此前仅有 `.modkit-owner` 的残留目录跳过逻辑继续保留，带清单缺源码仍阻断。
- 成功会话 `ready.json`：PID23516，监听 `127.0.0.2:17726`，51个频道。`gateway.err:260` 打开同一 SQLite 存档；`:308` 礼包 mod 启动自检就绪；`:309` 已加载一个 mod、一个 boot 钩子、一个 console 钩子和一份奖励脚本。
- 停止入口改用 `storage-route.ps1 stop-environment`，复用 Unicode 安全、隐藏子窗口、原窗口跟随日志的提权方式。执行 Go 停止链后再次检查活动档租约，只有明确的进程不存在错误才允许清理；存活、查询失败、无效 PID 均保留并失败。Go 停止器非零时仍检查残留租约；只读预检不提权、不清理。
- 提交前复查 `runtime/storage/entry-stop-environment.log` 发现23:33:40的无参数调用被空字符串拒绝，已归一化空参数，并补实际 null 位置绑定和空字符串回归。该修正未要求用户重复启动游戏；尚无修正后的实机停止链成功日志，确认范围不扩展为该停止序列的实机验收。

验证入口：`pwsh -NoProfile -File scripts/build-game.ps1`。`runtime/build-game/20261011-232925-997/` 保留日志、overlay、程序和旧文件备份。全量 build/vet（已安装 mod overlay）、完整基础 test（干净 overlay）均退出0，失败集合为空；独立 `TestLocalBuildInstalledModDeclarations` 退出0，注册名单为 `giveaway.random-equipment`，不执行 boot、不访问玩家库。wireprobe 测试71.558秒，catalog70.853秒。

`powershell -NoProfile -ExecutionPolicy Bypass -File scripts/test-stop-environment.ps1` 退出0，覆盖无参数/null/空字符串、预检、已退出/存活/不可查询持有者、无效PID、停止器失败及存档夹具原文保持；包含真实 Windows 进程查询，但不停止真实进程。`scripts/test-build-game-mods.ps1` 退出0，覆盖残留标记、仅测试源码、清单缺源码、排序及原加载器保持。停止入口 `--dry-run`、候选 `check --root . --source-build`、编码/行尾及 `git diff --check` 均通过。

| 当前产物 | 字节数 | SHA256 |
| --- | ---: | --- |
| `bin/wireprobe-handoff-source.exe` | 31693824 | `11dbb61118d1e4ad7d98f86037e2509e43891c81121a5bbc744482cebfd036c8` |
| `bin/dfolauncher.exe`、`bin/dfolauncher-adventure-elite.exe` | 16979968 | `c331eb0ccb5020aee0628e478bbe03d4957114279dcc418b11e415f27594de01` |

默认服务端保持 `a6f269603887f18fa8bb60814a655641f12c5eeaf1a07f89d840e166a393be6c`。本次未改 DLL、PVF、schema 或玩家存档，无迁移需求；未验证礼包实际领取及全部玩法。提交前尝试 fetch origin/fork（未配置，失败），实际 fetch gud 成功，HEAD 与 gud/main 差异0/0，无需合并。只提交本任务脚本、回归和文档。

## 早期启动恢复确认

业主确认原话：“确认能启动，可以提交”。确认范围为上游更新后的 SQLite 游戏启动恢复，以及已安装 mod 的编译、注册和启动自检；不扩大为全部玩法、礼包实际领取或精锐战斗验收。

## 问题与修复

- SQLite 入口选择 `dfolauncher-adventure-elite.exe`，该程序仍为旧提交 `be026c9f` 的构建。当前源码已支持 `DFO_LOCAL_GM="1"`，默认启动器检查通过，旧精锐启动器却报告 `Unknown profile key or invalid value: DFO_LOCAL_GM`。编译脚本现在同步重建本机已有的精锐启动器，并在安装前核对模块身份、备份旧程序。
- 后续 `210005_914760_next37/gateway.err` 末尾明确报告 `giveaway.random-equipment` 的 `console.command`、`reward.script`、`server.boot` 声明未注册。仓库加载器为空，直接构建不会包含本机已安装 mod。编译脚本按 `.modkit-owner` 或 `mod.json` 发现安装目录，稳定排序生成 Go import 和 Register 调用，通过临时 overlay 编译候选；仓库加载器原文保持。
- 完整基础测试使用干净加载器，保留无 mod 的奖励基线；另一个测试进程使用与候选相同的加载器、实际启用名单及 `CheckDeclarations`，任何声明/注册差异都阻断安装。该测试不执行 boot 钩子、不访问玩家存档。生成物采用 `.go.build` 扩展名，避免被 `go ./...` 扫描成新包。

## 验证与程序身份

成功入口：`pwsh -NoProfile -File scripts/build-game.ps1`，Go1.26.0，`GOTOOLCHAIN=local`。日志、overlay、产物及旧程序备份位于 `runtime/build-game/20261011-210803-310/`。

- 全量 `go build ./...`、`go vet ./...`：使用已安装 mod overlay，退出0。
- 完整 `go test ./... -count=1`：使用干净加载器 overlay，退出0，失败集合空；wireprobe 84.660秒。
- `go test ./mods -run '^TestLocalBuildInstalledModDeclarations$' -count=1 -v`：使用已安装 mod 和临时注册验证测试 overlay，退出0，注册名单为 `giveaway.random-equipment`。
- 空安装、安装标记发现、稳定排序、普通目录排除和原加载器保持的离线夹具检查通过；精锐路由/提权传递及 PowerShell5.1 语法检查通过。
- 实际候选 `-pvf-catalogs world -pvf-check-catalogs` 检查退出0，日志先确认 mod 已启用，报告 `storage_accessed=false`、`runtime_started=false`；实际内层源为 `4d8c0c82…`。

过程中的失败如实区分：首次沙箱测试的 Windows 命名管道出现 Access is denied，沙箱外同套测试通过；首次夹具落在 runtime 下被 Go 扫描，已移动本任务夹具并改生成物扩展名；直接给完整基础测试加载礼包 mod 时，`TestRewardScriptsDiskOverridesBundled` 和 `TestProductionMODRegistrationKeepsExistingCharacterRewards` 因额外金币/邮件失败，最终完整基础测试与独立 mod 校验均通过，未跳过或改写这两个断言。

| 产物 | 字节数 | SHA256 |
| --- | ---: | --- |
| `bin/wireprobe-handoff-source.exe` | 38759424 | `764caa1c77cee71578caf6c10a1ec0a0cecde5cf54e40a9112c983cc18957efa` |
| `bin/dfolauncher.exe`、`bin/dfolauncher-adventure-elite.exe` | 17738240 | `005b6463fd8f5bbdb74d3ce7bd84e47f9628836ae4f9cb47cc28fa6a3ccdfa04` |

## 手动启动证据与边界

业主手动会话目录：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261011_211110_256315_next37/`。

- `ready.json`：PID16896，监听 `127.0.0.2:1925`，51个频道。
- `gateway.err:2`：已安装礼包 mod 启用；`:343`：启动自检通过；`:345`：已装载一个 mod、一个 boot 钩子、一个 console 钩子和一份奖励脚本；`:348`：奖励规则脚本身份已列明。
- `runtime/storage/entry-game-sqlite.log` 已越过网关就绪并进入客户端启动步骤，与业主确认一致。PVF 哈希差异是现有告警，不是该次网关退出原因；本修复未改来源校验策略或运行资源。

默认 `wireprobe-pvf.exe` 的 SHA256 保持 `a6f269603887f18fa8bb60814a655641f12c5eeaf1a07f89d840e166a393be6c`。未发布默认服务端，未修改 DLL、PVF、schema 或玩家存档，无迁移需求。程序、日志、overlay、SQLite 文件和其它工作区改动不提交。

提交前实际执行 `git fetch origin`、`git fetch fork`：因本机未配置这两个名字失败，未猜测或新增远端；`git fetch gud` 成功，HEAD 与 `gud/main` 的左右差异为0/0，无待合并更新。本轮沿该上游基线提交，并保留上述实际测试结果。
