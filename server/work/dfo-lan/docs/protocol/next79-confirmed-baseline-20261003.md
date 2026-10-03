# next79 已确认基线（2026-10-03）

用户明确确认：变更作战、Boss 结算、奖励发放生效。确认基线 SHA256：
`b3fc95409ac600108030221ea69fdb0d5c83bf7180559394b703be4cb1aa82c9`，
源码候选与默认 wireprobe-pvf.exe 一致。

实机日志：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261003_142311_870596_next37/events.jsonl`。

- 14:24:54 CMD2047 action4 后客户端重新确认并进入 stage0；stage1 也再次变更作战并确认。
- 14:26:02 stage0 CMD1654 进入结算，14:26:14 CMD2046 结束领奖。
- 14:26:20 stage0 CMD72 `010201`，ACK `010102`，其后成功进入 stage1。
- 14:28:10 stage1 CMD1654，14:28:22 CMD2046；与用户截图和奖励界面确认一致。
- stage1 的回城仍有新缺陷：14:28:23 CMD72 focus `020201` 被错误命名为
  settlement_exit_ack，主循环清掉 activeDungeon；14:28:24 正式 `010201` 被拒
  `cards before owned settlement`。此项未纳入确认，另行修复。

本轮确认范围：原始7772/2405B奖励体、结算本地原生角色资料、变更作战返回选择流程，
前两阶段到奖励界面，第一阶段离场。奖励持久化/重登保持没有本轮验证，不据界面
成功扩大为入库确认；既有回放奖励入库限制仍在。四阶段全通、后续难度数值映射、
第二阶段及其后的回城未确认。

专项测试和 go vet ./... 通过。全量4项既有失败已在修复前overlay复现，见
analysis/tasks/next79-legion-weekly-open.md §29/30。没有改schema、玩家存档或客户端资源。
