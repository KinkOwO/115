# 图内换宠物：NOTI 103 CREATURE_STATE 证据（2026-09-26/27）

## 完整 NOTI 分发表已提取（2026-09-27，IDA Pro 9.4）

用 `E:\Program Files\IDA Professional 9.4` 的 idat 无头跑 IDAPython（`E:\115us\ida_probe1..4.py`，
输出在 `E:\115us\ida-work\probe1..4.out`），对工作副本 `ida-work\DFO.exe.i64` 提取出**全部四个注册函数**
（registrar = `sub_1459A3DD0(a1, id, handler, 0)`）：

- `0x1452BAD10`（134 条）、`0x1452F9420`（279 条）、`0x1453140E0`（180 条）、`0x14599D5D0`
- 合计 **593 个 NOTI id 的权威映射**（probe3.out 有全表）。

宠物家族真身（此前所有静态候选全部排除——46=DIE_PVP_CHARACTER、532=DIMENSION_SPACE_INFO、
1270=CUSTOM_ABILITY_PART_SET_OPTION、234=CONDITION_EVENT_CLEAR_INFO、551=LOGIN_TIME_EVENT、
1929=CONVERSION_SKILL，"宠物单例"实为 PVP/其它管理器）：

| NOTI | 处理器 | 反编译线格式与语义 |
| --- | --- | --- |
| 100 DIED_CREATURE | `sub_1452A9DB0` | `{u16 actor_id}`；actor-info+0x518(hidden)=1，隐藏两个跟随者对象；本地角色还弹 21/36 号提示 |
| 101 RENAME_CREATURE | `sub_14530ED90` | `{u16 actor_id, dstr(≤20)}`；写 +0x508 名字与跟随者 |
| 102 GAIN_EXP_CREATURE | `sub_1452D0DA0` | `{u8 level, u8 mode, u32 exp[, 2B]}`（已有基线）|
| **103 CREATURE_STATE** | `sub_1452A92F0` | **`{u32 key, u32 satiety}`：按宠物 key 设置饱食度**（sub_145E34260 按 key 查条目 + sub_145E34A70=105 读取器里的 set_satiety）。与召唤/外观完全无关 |
| 104 RESPONSE_CREATURE | `sub_14530FEC0` | `{u16 actor_id}`；本地角色宠物 vtable+2464(4) |
| 106 EVOLUTE_CREATURE | `sub_1452AF460` | `{u16 值, u16 actor_id}`；写 actor-info+0x504（mode0 写 item_id 的同一格）并刷新 |
| 107 REVIVAL_CREATURE | `sub_1452B34F0` | `{u16 actor_id}`；actor-info+0x518(hidden)=0，vtable+40(0)/+104 刷新两个跟随者对象 |

关键对应：mode0 宠物段（0x1456394b0）写 actor-info 的 +0x504(item_id) / +0x508(name) / +0x518(hidden)
与 100/106/107 操作的是**同一组字段**。所以 **NOTI 100（隐藏）+ 107（显示）就是服务端驱动的宠物显隐对**，
也是图内换宠物唯一不用整体重建的可见通道。

## attempt 2（2026-09-27，已部署）

- `protocol.ActorCreatureRef(actorID)`：`{u16 actor_id}` 两字节（LE），NOTI 100/107 共用。
- `cmd/wireprobe/equipment_flow.go` 的 `creaturePresenceRefresh(inDungeon, equipped, actorID)`：
  图内换上宠物 → NOTI 100（隐藏）+ NOTI 107（显示，客户端按当前穿戴重建跟随者）；图内卸下 → 只发
  NOTI 100；城镇不发（mode0 已重绑）。attempt 1 的 NOTI 103 两连包已移除——那是饱食度更新，
  `{0,0}+{1,0}` 还会把 1 号宠物的饱食度清零。
- 测试 `TestCreaturePresenceRefreshInsideDungeon`；`go test ./cmd/wireprobe ./internal/game/protocol
  ./internal/character` 通过、vet 通过。
- 候选 `bin/wireprobe-creature-presence-candidate.exe`
  （SHA-256 `CACB01ECBA4BBD55869803AEA20F227C94C33048FEC55530F9AA9AB2DF4AE610`），已部署到
  `bin/wireprobe-handoff-source.exe`，旧版备份 `.bak-presence-20260927-0230`。
- 待实机复测：① 图内换宠物跟随者是否变为新宠物；② 卸下后跟随者消失；③ 过门/回城正常；
  ④ 回城后外观正确。若显示侧仍用旧模型（107 只是重新显示缓存对象、不按 +0x504 重建），
  attempt 3 需要"让 +0x504 在隐藏期间更新"的通道（候选：106 EVOLUTE 写 +0x504 的路径做载体，
  或回到 mode0 在安全时机的实验）。

## 实机故障（attempt 1 期间的记录）

- 会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20260926_214727_641833_next37/events.jsonl`。
- 21:51:05 进图（NOTI2 mode0 携带 0x1456394b0 宠物段，跟随者正常创建）；21:51:08.819 副本内 CMD19
  `070000 8185dc1d 01 000000 03 1a00 8085dc1d …`（宠物仓 list7 槽 0 ↔ 穿戴栏槽 26 对换）。
- 服务端事务成功：NOTI19 应答 + NOTI13×2 + NOTI14(空间 3/7) + NOTI105 + NOTI102，无 NOTI2（9-23 门禁）。
- 玩家随后 21:51:17 过门（CMD38→CMD45 正常）、清怪、21:51:37 回城——副本流程不受影响，但可见宠物
  始终是旧宠物。对照同会话 21:51:03 城镇换宠物：同样的移动多发了 `equipment_appearance_refreshed`
  （NOTI2 mode0），宠物立即变化。

## 结论：客户端换宠物的可见绑定通道只有三条

1. **NOTI2 mode0 的 0x1456394b0 宠物段**（item_id=u32 + dstr 名字 + u8 present，present 的反码写入
   跟随者 hidden 位）。mode0 是整体重建包，副本内发送会卡切图（2026-09-23 两次实测），不能用作图内通道。
2. **NOTI105 宠物列表**（sub_1452CA7E0，559B）：逐条读
   `key:u32, satiety:u8, mode:u8, exp:u32, [mode==1: u8,u8], level:u8, dstr(≤0x1e), tail:u8`，
   尾字节以 `entry[0x30] = (tail==1)` 存储。整个函数只构建列表数据，**没有任何场景/跟随者调用**——
   105 是纯 UI 数据通道，改键值/改首条都不会让跟随者重绑。
3. **NOTI103 CREATURE_STATE**：客户端分发表（`analysis/dumps/opcode_table_detailed.json`，表槽
   0x14ef337e8）登记为 `ENUM_NOTIPACKET_CREATURE_STATE`。宠物子系统内形状匹配的读取器：
   - `0x1452cac00`：`u8 state, u8 mode` → 取全局单例 `[rip+0x93a1453]` 的 +0x130 宠物对象，
     虚表 `+0x230(state, mode)`。
   - `0x1452cacb0`：`u8 flag` → 单例 +0x118 对象的 `+0x3c8 = (flag != 0)`（布尔）。
   两者都不做 actor 级重建，与 102/105 同级风险（图内已实测安全）。

辅助佐证：客户端发送 `CMD 2018 CREATURE_SUMMON (0x7E2)` 时负载为空
（0x142b5f8d0 处构造空 payload，`mov r8d, 0x7e2; call 0x145450460`），召唤状态由服务端应答驱动。

## 实现（attempt 1/3）

- `protocol.CreatureState(state, mode)`：负载 `{state, mode}` 两字节。若真实读取器是单字节变体
  （0x1452cacb0），首字节仍是 state，第二字节留在同一 NOTI 帧内未读，无害。
- `cmd/wireprobe/equipment_flow.go` 新增 `creatureStateRefresh(inDungeon, creatureEquipped)`：
  - 副本内且换上宠物：`{0,0}` + `{1,0}`（先收回后召唤，保证 0→1 状态翻转触发按新穿戴对象重建跟随者）。
  - 副本内且卸下宠物：`{0,0}`。
  - 城镇：不发（mode0 已重绑，状态翻转是多余动作）。
- 顺序：跟随在 NOTI105/102 之后，与既有 13/14 行同批发出。

## 测试

- 新增 `cmd/wireprobe/equipment_creature_state_test.go::TestCreatureStateRefreshInsideDungeon`：
  断言图内换上=两包(0→1)、卸下=一包(0)、城镇不发。
- `go test ./cmd/wireprobe ./internal/game/protocol ./internal/character` 通过；
  模块级 `go test ./...` 中 `runtime/update-backup/...`、`runtime/manual_update`、
  `cmd/itemshopimport`、`internal/inventory` 的失败均为 22:13 更新包遗留（测试引用了源里不存在的
  `itemNeedMaterials`/`AddExtra`、备份树缺 fixture），与本次改动无关。
- `go vet ./cmd/wireprobe ./internal/game/protocol ./internal/character` 通过。

## 待实机复测

1. 图内换宠物：可见跟随者是否变为新宠物（核心验收）。
2. 图内卸下宠物：跟随者是否消失。
3. 换宠物后继续过门切图：不得复现 9-23 卡门。
4. 回城后宠物外观正确（回归 9-23/9-26 修复）。

若 attempt 1 无效（跟随者不变）：优先核对 103 实际注册的读取器（.i64 内确认表槽 0x14ef337e8 的
运行时写入者），再尝试单字节形态或调整 state/mode 语义；若触发卡门则回退本通道。
