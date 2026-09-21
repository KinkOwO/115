# next49 — 清关后的「下一个剧情关卡」门：CMD 2062 DUNGEON_DIRECT_MOVE

## 现象

奥德赛（Arad Odyssey）副本清关后，场上出现两道传送门：**「返回城镇」**与**「下一个剧情关卡」**
（截图里后者是「卡勒特歼灭战」）。点「返回城镇」正常；点「下一个剧情关卡」只触发传送黑屏，
不会真的进入下一关。回城后从剧情 UI 点「移动」再进同一关则正常。

## 证据链

来源1 — 会话日志（`runtime/roles_..._20260921_225338_199523_next37/events.jsonl`，角色 `test-jh`）。
清关序列完整（`boss_check_confirmed` → `dungeon_clear_enabled` → `dungeon_play_result` →
`dungeon_clear_experience` → `dungeon_clear_reward` → `card_*` → `odyssey_clear_target_level` →
`odyssey_journal_updated`），此后点门时只有：

```json
{"id":2062,"kind":"client_frame","plain_hex":"4e01000000000000ffffffffff53f4f505020000000000000001000000a0000000c8000000140000000a000000000000","unimplemented_sample":true}
```

即客户端发出 **CMD 2062**，服务端只把它当"未实现请求采样"（`cmd/wireprobe/main.go:882`），
没有任何响应，客户端一直停在黑屏。

来源2 — 客户端 opcode 表（`analysis/dumps/opcodes.tsv`）：
`cmd 2062 = 0x080E = ENUM_CMDPACKET_DUNGEON_DIRECT_MOVE`，语义就是"副本直达"。

来源3 — 两次真实请求的核心字段（同一会话内两次点门，48 字节）：

| 时间 | 目标副本（+13，小端） | 难度（+17） | 门矩形（+29 起四个 u32） |
| --- | --- | --- | --- |
| 15:00:01 | `78f4f505` = **100004984** | 2 | 103, 205, 20, 10 |
| 15:02:21 | `53f4f505` = **100004947** | 2 | 160, 200, 20, 10 |

两次的矩形尺寸都是 20×10，只有位置不同——是客户端走进去的那道门。
`100004947` = 当前副本 `100004946` 的下一关，且在源数据里正是
`contents/2026/aradodyssey/dungeon/13_cartel/cartel.dgn`（**卡勒特**，与截图门名一致）。

来源4 — 目标副本的准入字段（`configs/dungeons.full.json`）：
`100004947` → `MinimumLevel 52`、`Odyssey true`、`DesignatedDifficulty 2`、`NoFatigue false`；
清关日志里 `odyssey_clear_target_level` 的首字节是 `0x34` = **52**，即本关清关后角色正好升到该关卡
要求的等级。上一关 `100004946` 是 `MinimumLevel 50`。

来源5 — 本客户端"进入一个副本"的既有帧序列（同一会话里从城镇进 `100004946` 的原始记录）：

```
dungeon_gate_ack (1,15,[01])                ← 客户端先点副本门
dungeon_selection_sent (0,27)               ← 服务端给可选副本表
[客户端发 CMD 16 选择]
dungeon_select_ack (1,16,[01])
dungeon_actor_appearance_sent (0,2)
dungeon_actor_addition_sent (0,2)
dungeon_worn_visuals_sent (0,14)
solo_party_initialized (0,9)
dungeon_info_sent (0,28)                    ← NOTI 28，携带副本 ID/难度
dungeon_start_map_sent (0,29)               ← NOTI 29，携带首房地图/怪物
[客户端发 CMD 37 加载]
dungeon_loading_ack (1,37,[01]) → actor_state (0,3) → loading_complete (0,30) → experience/fatigue/worn
```

## 实现

1. `internal/game/protocol/dungeon.go`：`DungeonDirectMove` + `DecodeDungeonDirectMove`
   （48 字节；消费 +13 的目标副本 ID 与 +17 的难度，+29 起的门矩形作为证据保留，其余字段与原始 48 字节
   一并留档，不臆断语义）。
2. `cmd/wireprobe/dungeon_flow.go`
   - 把"进副本帧序列"抽成 `dungeonEntryPlan(ackName, ackID, sel, session)`，CMD 16 与 CMD 2062 共用；
   - 新增 `directMoveDungeon`：校验存在进行中的副本、目标副本在源数据中存在、奥德赛副本需要奥德赛角色、
     疲劳（与 CMD 16 同规则），然后用 `dungeon.Select` 建新会话（等级/难度/maze 门禁与城镇选图一致），
     最后复用同一帧序列，**ack 换成 `(1, 2062, [01])`**；
   - 顺带把 CMD 16 里的"accepted 任务集合"读取抽成 `acceptedQuestIDs`，两处共用同一份判定。
3. `cmd/wireprobe/main.go`：CMD 2062 分发到 `directMoveDungeon`，返回值走 `pending` 分支
   （替换 `activeDungeon`、离开共享城镇场景、清掉 `drops`/翻牌状态），失败时不回通用 Refusal
   （与 2015 同样的处理）。
4. `cmd/wireprobe/request_scope.go`：2062 加入 `dungeonRequest` 白名单，请求体不再走未实现采样。

## 测试

- `internal/game/protocol/dungeon_direct_move_test.go`：两条真实 48 字节样本逐字段解码（副本 ID、难度、
  门矩形）+ 原始记录保留 + 非 48 字节拒绝。
- `cmd/wireprobe/odyssey_direct_move_test.go`：无进行中副本 / 目标不在源数据 / 坏长度三条拒绝路径；
  `dungeonEntryPlan` 的首包是 `dungeon_direct_move_ack(2062)`、末两包是 NOTI 28（携带目标副本）与 NOTI 29。
- `go test -count=1 ./internal/... ./cmd/...` 与 `go vet ./internal/... ./cmd/...` 全绿。

## 实机验证步骤（用户操作）

1. 用 `DFO-115US单机一键启动器.exe` 重启（启动器会重编源码候选版），进奥德赛副本清关。
2. 走「下一个剧情关卡」门（如打完「根特防御战」走「卡勒特歼灭战」）。
3. 预期：**直接加载下一关**（黑屏后进入 cartel 副本），不再卡住；「返回城镇」照旧可用。
4. 若仍黑屏：取 `runtime/roles_*/events.jsonl`，重点看
   - 有 `dungeon_request_refused id=2062 reason=...` → 服务端拒绝了（原因会写明：等级/难度/任务/疲劳）；
   - 没有该事件、且 `dungeon_direct_move_ack`/`dungeon_info_sent`/`dungeon_start_map_sent` 已发出
     → 客户端消费的入口不对，需要按 `analysis/AGENTS.md` 的门禁回到 IDB/frida 取证（本机无 IDA/Ghidra）。

## 未闭环点

- **响应侧未做静态验证**：本实现依据的是"同 id ack + 本客户端进副本既有序列"这一在本客户端已验证过的惯例
  （CMD 15/16/37/45/2015 都是这样），不是从 IDB 读出的 2062 消费路径。首次实机若不生效，下一步是
  hook 客户端 2062 的响应处理函数（frida，见 `analysis/tools/arrow_probe.py` 的既有做法）。
- 请求里 +0（207/334）、+4、+8..+12、+25 的字段语义未定，未参与判定。
