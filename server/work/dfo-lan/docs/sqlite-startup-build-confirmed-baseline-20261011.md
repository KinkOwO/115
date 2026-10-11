# SQLite 启动链与本机 mod 编译：确认基线

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
