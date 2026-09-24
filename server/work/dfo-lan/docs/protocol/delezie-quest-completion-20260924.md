# 瘟疫之村任务 23108 通关与任务完成基线

状态：用户实机确认，2026-09-24。C2S attempt 1/3。

## 问题与依据

任务 23108 `Confront Apostle Delezie` 的 `[clear map]` 目标为 `100008696`；副本 7123、迷宫 87 的源 `[clear condition]` 也指定该地图。路线在标记 Boss 房 `76425` 之后继续经过 (5,0)、(6,0)，再进入 (6,0) 的 layered 地图 `100008696`。`76425` 只有非战斗展示 Boss，末段目标图则有可战斗 Rank-3 Boss。

旧行为在 `76425` 加载后提前发送 NOTI115 和 NOTI31，客户端进入结算，无法继续走到任务目标图，因此任务不能完成。

## 修复

服务端对这一源迷宫延后副本完成判定，直到进入 `100008696`。在该目标图允许源 Rank-3 Boss 的既有 CMD117 校验通过；死亡确认和正常结算流程保持原样。其余副本不受此迷宫条件影响。

## 实机验证

会话日志：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260924_202843_277130_next37/events.jsonl`。

- CMD45 房间切换依次进入 `76425`、`100017716`、`100017717` 和 `100008696`；`76425` 加载后没有提前发送结算。
- 在 `100008696` 收到 CMD117，随后记录 `boss_check_confirmed`（NOTI115）和 `dungeon_clear_enabled`（NOTI31）。
- 客户端请求 CMD46，服务端记录 `dungeon_play_result` 与 `dungeon_clear_reward`。
- 后续记录 `quest_finished`，任务 ID 为 23108。
- 用户确认实机通关不再黑屏，任务现可完成。

`go test ./...` 与 `go vet ./...` 通过。候选服务端 SHA-256：`35C74E9B78A34CFB29269F948302EC54DE00CE88145D5DEC8A481DCF8F6DDA7A`。
