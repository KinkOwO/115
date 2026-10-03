# 黄龙/银龙大会首场提前结算修复（2026-10-03，已确认）

用户反馈两个大会只赢第一场就触发最终胜利，没有进入下一场。

## 当前客户端证据与根因

手动会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_183234_180122_next37/events.jsonl`，角色10：

- 18:55:04（北京时间）进入黄龙副本100003298、地图100008699；NOTI372携带四轮路径实体4096～4099，最后一只为Rank3。
- 18:55:15.177，CMD39仅上报第一轮实体4096死亡；服务端发出死亡确认，回合推进到2。
- 18:55:15.194，客户端立即发CMD33，明文 `2100d835000000000000000000000000`，任务13784。
- 18:55:15.195～.196，服务端发出 `quest_scene_trigger`、`map_clear_quest_triggers`、`boss_check_confirmed`、`dungeon_clear_enabled`、`tournament_clear_reward`；随后CMD46触发普通结算。

实际触发链为 `questTrigger → SceneTrigger → SceneClearObjective → MarkSceneCompleted → completeDungeon`。大会四轮共用同一个任务目标地图，旧SceneClearObjective只验证已加载和地图匹配，把首场CMD33当成剧情收尾；MarkSceneCompleted绕过四轮战斗完成条件。此处不是CMD117实体错位兼容分支导致，本次没有修改那个分支。

当前PVF原生导入确认13784/13785均为单地图 `[clear map]`，分别对应100008699/100008700；当前DGN/MAP原生大会定义及既有NOTI372四轮reader证据见 `yellow-dragon-entry-evidence-gap-20260925.md`。没有新增包或改变reader/字段布局，沿用已确认四轮战斗及最终BossCheck流程。

## 实机确认

用户选择角色10 gene，在独立候选入口手动验证。会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_190800_657927_next37/events.jsonl`：19:09:09进入黄龙100003298；四名对手实体4096～4099于19:09:17、19:09:27、19:09:38、19:09:46依次确认死亡。每轮均有客户端CMD33，但前三轮均未发送任务结算或副本完成包；第四轮后才依次发送最终Boss确认、NOTI31通关开启与NOTI374。19:09:58两次CMD450选择均有ACK。

19:10:15进入银龙100003299；四名对手依次于19:10:24、19:10:33、19:10:46、19:11:07确认死亡，前三轮CMD33后继续下一场，第四轮后才出现Boss确认、NOTI31与NOTI374。19:11:17两次CMD450选择均有ACK。角色10的银龙任务13785在此前尚未接取，用户按正常任务流程接取；测试后任务状态符合流程。两大会各四轮、最终结算及奖励选择均由同一场手动会话日志确认。

本轮confirmed baseline为下述独立候选哈希；默认PVF二进制未替换。

## 服务端修复边界

- 未完成大会的CMD33不适用剧情通关路径：在存储写入之前静默返回，避免提前推进任务，也避免提前发送最终奖励。
- `Session.MarkSceneCompleted`不允许覆盖大会完成状态；大会继续由已有死亡顺序与最终BossCheck决定完成。
- 最终完成后原有MapClear任务推进、NOTI374和CMD449/450奖励流程保持。
- 没有修改客户端、PVF、schema、codec或奖励规则。

这是已确认大会功能的证据驱动纠错，沿用历史 `attempt 3/3`：没有提出第四个试包假设。回滚边界为 `internal/quest/scene_trigger.go` 的大会守卫和 `internal/dungeon/completion.go` 的MarkSceneCompleted守卫，以及对应回归测试；不要回滚用户的其他改动。

## 验证与候选

- HEAD overlay运行新增原生回归，在黄龙第一场准确失败：`CMD33 bypassed completion: applies=true`。
- 修复后，当前内层PVF的黄龙/银龙两项原生回归通过：逐场死亡后CMD33不能完成、回合推进至下一场，第四场后最终BossCheck正常完成；两大会用户手动实测通过。
- 无Store的专项证明首场触发在任何数据库写入之前返回；既有剧情通关3939、真实Boss确认、大会入场/选择回归通过。
- Go1.26.0全量 `go test ./...`、`go vet ./...` 与独立候选构建通过；当前PVF全54域原生准备通过。
- `go run ./cmd/charactercheck` 在隔离临时schema内失败：缺少 `account_unified_options` 表；HEAD overlay复核同样失败。这是旧检查工具的既有问题；本轮未改schema。

confirmed baseline：独立候选 `server/work/dfo-lan/.tmp/tournament-rounds-20261003/wireprobe-tournament-rounds.exe`，SHA256 `0d956338390f5f1332ed1f9bb0eacafe07510640461761cda01f53f19681a070`。启动入口为同目录 `启动验证.cmd`；profile继承当前 `configs/pvf-default.json`，只改变候选程序路径。保留默认程序与既有39版基准。

## 授权任务回放

用户明确授权为实机验证把大会任务的达成状态改回未完成。只读查库发现角色10（gene）的13784为accepted/progress0，13785尚无行；角色11的两项均completed/progress0。用户选择gene（ID10）：仅回退黄龙，打完交任务后正常接银龙。

实测前将角色10/任务13784从accepted/progress0改为accepted/progress1，保持config_version、progress_model、accepted_at及completed_at原值；银龙在黄龙交付后正常接取。事务写入既有 `character_quest_repairs`，reason=`tournament-rounds-role10-replay-20261003`。实测后按根 `.tmp/tournament-rounds-20261003/character10-quests-before.json` 精确恢复黄龙任务原行，逐字段相等验证通过；不回滚银龙任务或奖励、角色物品及其他进度。

独立profile的 `launch_local.py --check` 通过，解析至本轮候选，PostgreSQL可连接；没有启动客户端。任务备份、操作脚本与测试日志留在本轮根 `.tmp/` 私有目录，不提交存档数据。
