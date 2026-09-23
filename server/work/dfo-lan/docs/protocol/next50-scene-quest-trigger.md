# next50 — 剧情场景 [clear map] 任务的 SET_QUEST_TRIGGER 闭环（坠机场景卡死修复）

## 现象与证据

2026-09-22 实机（`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260922_204626_569689_next37/events.jsonl`）：
主线任务 3191「What Falls from the Sky」（skycastle_21.qst，`[clear map]`，目标地图 100008683，副本 15 maze 6）
进入副本 15 后按序走完 boss 房 (0,0) 的层叠场景 57981 → 100008695 → 100008694 → 100008684 → 100008683
（每次 `dungeon_next_map_sent` + `cinematic_skip_saved`，12:54:59 ~ 12:55:27，共 6 张图）。
在最终层（坠机现场）两只脚本怪死亡（实体 0x1013/0x1014）并播完过场（cinematic 0x3a32）后，
客户端于 12:55:29.910 发出 CMD33 `21 00 77 0c 00…`（SET_QUEST_TRIGGER，quest 3191），
服务端回 `quest_interaction_refused: NPC interaction requires an owned town character`，之后客户端只剩心跳（id=122），卡死。

对照成功路径（同日志 quest 3190，12:53:13）：boss 检查（CMD117）→ boss 死亡 →
`map_clear_quest_triggers`(NOTI291) + `boss_check_confirmed`(NOTI115) + `dungeon_clear_enabled`(NOTI31) →
客户端自发 play_result（CMD46）。而本场景房（地图 57981 及其层）怪物表只有 5 只 `[fixed]` `[cinematic]`
剧情 NPC 怪（模板 109015589/109015581/109015474/109015667/109015585），**没有 rank-3 boss**，
客户端因此从不发 CMD117，服务端 `completionTarget` 永远为 0，通关序列无从触发——场景结束后
客户端改走 SET_QUEST_TRIGGER 直接置任务触发器。

## 协议定性

- CMD33 = `ENUM_CMDPACKET_SET_QUEST_TRIGGER`（`analysis/dumps/opcodes.tsv`，字符串 VA 0x14b03f3d0）。
- 90dof 参考实现（`quest/request.go`）佐证布局：前 2 字节 wire echo(0x21)，随后 u16 questID、u8 triggerType、u8 isIncrement；
  本客户端实机正文 `21 00 77 0c` + 12 字节零尾一致（triggerType=0）。
- 服务端既有城镇路径：CMD33 对 `[meet npc]` 任务置进度 0 并回 NOTI291（QuestTriggers）。
  本次把同一语义扩展到副本内的 `[clear map]` 场景目标。

## 服务端判定（不信任客户端触发值）

`internal/quest/scene_trigger.go` `SceneClearObjective`：
- 任务必须是单目标 `[clear map]` 且无 pending；否则**静默忽略**（与城镇路径对非 meet-npc 任务一致）。
-  run 必须存在且已 Loaded；`run.Room.Map` 必须等于任务目标地图（最终层），否则拒绝（记 `quest_interaction_refused`）。
  层叠最终层只能经有序场景切换到达，且此前每个房间都已清场——"站在目标图上"本身就是服务端持有的证据。

`SceneTrigger` 落账复用 `Store.RecordQuestMapClear`（runID 键控幂等，重复触发/重放不会重复结算；
该 UPDATE 只命中 `status='accepted' AND progress=1` 的本角色任务），随后回 NOTI291（与城镇 meet-npc 路径同一响应形状）。

## 改动文件

- `internal/quest/scene_trigger.go`（新增）：`SceneClearObjective` 纯校验 + `Service.SceneTrigger`。
- `cmd/wireprobe/quest_flow.go`：`questInteraction` 前置校验不变；`activeDungeon != nil` 时走场景路径，
  城镇 `[meet npc]` 路径逐字保留。
- `internal/quest/scene_trigger_test.go`（新增）：最终层命中、四种非场景任务静默、无 run/未 Loaded/错误层图三拒绝。

## 验证

- `go test ./...` 与 `go vet ./...` 全绿（含既有 quest/dungeon 回归）。
- `bin/wireprobe-handoff-source.exe` 已重建（sha256 30392313c9ab40343b3469e65f1517583ffbf835db2358eb4948f9cb61dbaf5e），未覆盖 39 版归档基线。

## 实机验证（已通过，用户操作）

用户实机测试确认：重登回城后用源码候选版重打该剧情副本，坠机过场播完后任务 3191 正常收到 NOTI291（进度置 0）变为可提交，并顺利完成任务推进后续剧情。服务端事件为正常 `quest_scene_trigger`。

## 未闭环

- 场景通关后客户端是否还会要 `dungeon_clear_enabled`/`boss_check_confirmed` 或自发 play_result/离图，
  待本次实机观察；如需补，以实机证据为准另起 attempt。
- 其它 `[clear map]` 场景任务（目标图为层叠最终层、无 boss）同路径受益，但未逐一实机回归。

## 回滚

`git checkout -- server/work/dfo-lan/cmd/wireprobe/quest_flow.go` 并删除本轮新增的
`internal/quest/scene_trigger.go`、`scene_trigger_test.go` 与本文档，重编候选版即可。
