# 宠物通关经验：服务端确认基线（attempt 2/3）

## 故障与证据

- 原实现的 `CreatureListPayload` 对每只宠物固定发送经验 0、等级 1；登录、进图、穿脱和回城的 NOTI 102 也固定发送 `01 00 00 00 00 00`。通关结算只保存角色经验，没有宠物经验。
- 当前 115 客户端 `DFO.exe.i64`：通知 105 注册到 `sub_1452CA7E0`，读取 `key:u32, satiety:u8, mode:u8, cumulative_exp:u32, [mode==1: 2 bytes], level:u8, name, tail:u8`。
- 同一客户端：通知 102 注册到 `sub_1452D0DA0`，读取 `level:u8, mode:u8, cumulative_exp:u32, [mode==1: 2 bytes]`，随后调用 `sub_145E349E0`。后者用客户端升级阈值表按累计经验计算当前等级的进度。
- 当前客户端导出的 `Script.inner.pvf` 中 `creature/exptable.tbl` 有 55 个阈值，首项为 2；本修复使用这份资源的数值，等级上限为 55。
- 截图中的 “Creature EXP: 0%” 是玩家可见的百分比，提示文案说明它随通关消耗的疲劳值增长。百分比怎样由累计经验换算，仍需实机对照；服务端以已核实的累计经验与等级字段同步。

## 候选实现

- 通关结果走 `clear:<run_id>` 角色事件事务。结算从 `character_fatigue_rooms` 读取该 run 的房间收据及实际扣除疲劳值；事务中只给 slot 26 穿戴宠物增加累计经验。当前不扣疲劳的普通副本按首次加载房间数奖励，明确免疲劳副本不奖励。
- 在原有 inventory JSON 中增设可选 `creature_experience` 映射，以宠物实例键保存经验。旧存档没有此字段时读取为 0，不迁移或重建玩家物品。
- 105 与 102 从持久化经验构造；结算有实际经验增加时，在原结算包之后发送更新。事件键去重，重复结算不重复发奖。

## 验证与确认

- `go test ./...` 通过；经验上限收紧后重跑 `go test ./internal/inventory ./internal/character ./cmd/wireprobe` 通过；`go vet ./...` 通过。测试使用项目运行目录内的独立 Go 缓存，以避开本机默认缓存目录的权限问题。
- 已编译隔离候选版 `bin/wireprobe-handoff-source.exe`，未覆盖 39 版归档基准。
- 用户手动操作候选版完成实机确认：通关后宠物经验增加并升级，更换角色后经验与等级仍保留。
- 候选程序 `bin/wireprobe-handoff-source.exe` SHA-256：`5D975BB82D8033A000B844111AF635089124E0437BF16701E89F44590C2A66CC`。
- 本次不涉及客户端补丁或数据库结构变更；旧存档缺少经验映射时按 0 读取。

## attempt 1/3 实机反馈与根因

- 用户在 2026-09-24 手动通关、重登后，宠物经验仍为 0，同时观察到疲劳没有消耗。
- 会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260924_191116_212304_next37/events.jsonl` 显示副本 50、51、52 的所有 NOTI 36 疲劳值均为 0，通关后没有宠物经验更新通知。
- `configs/fatigue-probe.json` 的 `room_cost=0`；提交 `d81470f` 明确把进图疲劳改为不消耗。疲劳服务仍为每个首次加载的房间写 `character_fatigue_rooms` 收据，但 `cost=0`。首轮代码只求和 `cost`，所以经验必为 0。

## attempt 2/3 修正

- 用户确认保留不扣疲劳。普通副本通关时，若本 run 有实际疲劳扣除，则按扣除值给宠物经验；若实际扣除为 0，则按房间收据数给宠物经验。收据按 `character_id,run_id,map_id` 唯一，重复加载不重复计数。
- `NoFatigue` 的训练场等副本仍然不发宠物经验。疲劳规则及 UI 维持原值 0；宠物奖励与角色通关奖励在同一个幂等角色事件内保存。
- `go test ./...` 与 `go vet ./...` 已通过；用户手动关闭旧游戏环境后，重新编译 `bin/wireprobe-handoff-source.exe` 并手动实测通过，现标记为确认基线。
