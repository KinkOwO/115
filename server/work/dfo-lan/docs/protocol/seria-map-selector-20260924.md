# 赛丽亚房间右侧选图回原点：候选修复

状态：**用户实机确认通过**。C2S 同功能变更计数：**attempt 2/3**。

## 证据

- attempt 1/3：`e21cde9` 补齐 CMD 1418 和 Return 戳返程；用户确认西海岸下方出口可返回。之后右侧快速传送出现“任意选图都回进入赛丽亚前的地点”的回归。
- 实机会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260924_154433_391213_next37/events.jsonl`：第 225–228 行下方出口发送空体 CMD 1418，返回保存的 40/0；第 290–293 行右侧选图发送 CMD 36，明文 `28000000000000007f01bd00052600000001000005000000`，解码为目标 40/0 `(383,189)`、`Flag=5`、`TailFlags=[5,0]`、来源 38/1。服务端却保存了 Return 戳 39/0 `(3494,314)`。
- 当前 `configs/world-probe.json` 设置 `require_portal_proximity=false`，所以仅靠距离校验不能区分这两条路径。
- 复验会话 `roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260924_155305_912949_next37/events.jsonl`：第 177–180 行 CMD 1418 从赛丽亚返回 39/0；第 211–214 行右侧地图选择 CMD 36 目标 40/0 `(374,191)`，服务端保留所选落点并切至 `westcoast.map`。用户随后确认两条路径都已恢复正常。

## 候选改动与回滚

- CMD 36 带地图传送标志时走现有 `teleportTransition`，按所选地图和落点完成等级、地图及坐标校验；Return 戳不再覆盖它。普通赛丽亚出口和 CMD 1418 仍走 Return 戳。
- `world.Service.transition` 同步排除地图传送标志，防止此层直接调用时再次改写目的地。
- 以实机 CMD 36 明文加入回归测试，断言从带有 39/0 Return 戳的房间选 40/0 时，落点为 40/0 `(383,189)`；既有下方出口测试继续覆盖 Return 戳。
- 回滚 attempt 2/3：将 `odyssey_teleport.go` 和 `internal/world/service.go` 两处地图传送排除条件恢复至当前 HEAD，并移除对应回归用例；此举会恢复右侧选图回原点的缺陷，仅用于隔离回退。

## 验证与确认基线

`go test ./...` 与 `go vet ./...` 通过；确认程序 `bin/wireprobe-handoff-source.exe` SHA-256 `63FFEF2584461D65E0ECCE7D8D7E8EF7A870286F3CDDEC94D19F441704926ED4`。用户手动确认右侧选图和下方出口均正常。
