# A Song of Destruction — Resting Place 追逐切房

2026-09-28，attempt 1/3。服务端修复已由用户手动实机确认切换场景进入下一个房间，确认为本路线基线。

## 实机证据

会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260928_132502_509875_next37`：

- 05:36:06Z：进入副本 100002476，maze 0，任务 12415。
- 05:36:38.9256088Z：进入追逐地图 100003288，坐标 (5,0)，一个原生地图怪物。
- 05:36:59.3128486Z：原生 CMD45 请求目标 (6,0)。随后每约三秒重发，总计六次；18 字节 transition record 完全一致。
- 服务端六次拒绝，原因为 `current room not loaded or still has live enemies`；未下发下一地图。

原生 CMD45 正文已逐字节保存在 `internal/dungeon/forced_script_warp_test.go`；这六次客户端重试来自同一次运行，不是六次服务端修改尝试。

## 当前客户端源与 IDA 链

源 PVF 为 `server/work/client-build/Script.inner.pvf`，checksum `7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80`。只读导出位于被忽略的 `runtime/grave-*` 目录，不提交原始导出或运行日志。

1. Dungeon `100002476_graveofrest.dgn` 的 maze 对应任务 12415，声明 `[warp map condition]` 从 (5,0) 到 (6,0)。这里 `[move map even enemy]` 是 0，因此没有把全图普通走门改为允许带活怪通过。
2. 地图 100003288：原生怪物 109013068，带 `[cinematic]`，`[basic action] Action/Auto_Dash.act`，普通 pathgate 禁用。
3. `list/monster.lst` 将 109013068 指向 `contents/2021/ozma_raid/monster/raid/phase1/boss/berias_chariot/berias_chariot_q.mob`；该 mob 的 etc action 包含 `Action/Last_0.act`。
4. `Last_0.act` 在 frame 8 开黑屏并启用定时 trigger；1500ms trigger 执行 `[KICK OUT MAP CHARACTER]`，目标网格 (6,0)，落点 (151,330,0)，随机范围 (30,20)，包含死亡玩家。并不要求先击杀该追逐角色。源 action SHA-256 `ef821999c36196a4da7d43ef6a4c733043ede09527840f002a701a84fbbacdc3`。
5. 当前权威 IDB 的注册函数 `140cca4f0` 将 KICK OUT MAP CHARACTER 注册至 `140ced7f0`。执行函数先校验当前角色/房间、节流及包含死亡玩家分支，再调用 `145ea1ca0`；后者创建 type=1 的脚本跳转记录，默认方向 5/5，委托 `144d349e0`。
6. NOTI29 reader `1452b7100` 在 `1452b74b5` 读取 18 字节，通过 `145ea2f90` 按字段消费 type、flag、两个方向及六个有符号 u16 值。`145ea3770` 把解码后的记录写入地图过渡状态。既有 StartMap 新房间模式复用该 reader，未改变包布局、codec 或 loading 顺序。

六次原生 transition record 均为：`01 00 00 00 05 05 97 00 4a 01 00 00 1e 00 14 00 00 00`，与上述 action 的落点和范围一致。确认的函数命名已写入当前 IDA 会话及 `analysis/dumps/idb_funcs.tsv`。

## 服务端修改与边界

复用既有 `MoveScript`，移除其仅允许 Odyssey 的总门禁，以已取证的具体路线为准；原 Odyssey 白名单保留。新的 `forced_script_warp_routes.json` 仅添加本条路径，绑定 PVF checksum、副本/maze、原地图/目标地图、坐标、DGN/map SHA-256 和完整 transition record。

只在已加载的所属副本、匹配源路线和原生记录时允许带活怪进入下一房间。普通 `Move`、其他地图和未取证强制路线不放宽；不伪造 CMD39、怪物死亡、经验、掉落或任务通关。目标 Boss 仍走原有战斗和任务完成逻辑。

无客户端、DLL、数据库结构或玩家存档修改。回滚本次 `script_warp.go` 差异并删除新增强制路线文件，即恢复原运行路径。

## 验证与候选版

针对性测试覆盖：真实源地图活怪、原生 CMD45、普通走门拒绝、正确强制跳转、NOTI29 原样落点、新房间模式、无伪造战斗进度、错误副本/目标/record/layer、未加载和源校验拒绝。

`go test ./...`、`go vet ./...` 均通过；候选编译通过。候选 `bin/wireprobe-handoff-source.exe` SHA-256：`559B93604E6354289EF8484FEBF4B54FDED6B1B6BEAB217B75D9E507B7BCC1A5`。

## Confirmed baseline

用户手动以候选版实机确认：追逐最后黑屏后成功切换场景并进入下一个房间，修复的强制切房路径成立。当前确认止于进入下一房间；Boss 战斗和任务提交仍需另行覆盖，不影响本次卡场修复的基线确认。候选 SHA-256：`559B93604E6354289EF8484FEBF4B54FDED6B1B6BEAB217B75D9E507B7BCC1A5`。
