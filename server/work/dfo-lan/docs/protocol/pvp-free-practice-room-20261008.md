# 决斗场（PKC）自由练习场：频道接入与服务端房间协议（2026-10-08）

> 证据来源：115 客户端实机会话 `runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261008_{154719,155929,160334}_*_next37`
> 的 `events.jsonl` / `%LOCALAPPDATA%\DNF\DFO.trc`；opcode 名取自 `analysis/dumps/opcodes.tsv`；
> 参考实现为 90 级官方 Go 服务端 `pvp` 包（**只作线索，不作为事实标准**，见 `AGENTS.md` §0 铁律 2）。

## 1. 结论

- 决斗场「自由练习场」是**客户端内置**的频道类型：`CHANNEL_INTEGRATED_FREEPVP`，
  **channelType = 13**（115 客户端自己在 `DFO.trc` 里打出来的：`set Channel Type : [ CHANNEL_INTEGRATED_FREEPVP ]`）。
- 它**不在**内层 PVF 的 `etc/clientchannelinfo.etc` 里（该表只有 41 个团本/特殊类型），
  因此服务端要"发布"它，只能走 `internal/channelrefresh` 的**本地 Type 覆盖**分支
  （`channelrefresh.Config.Resolve`：ID 在 PVF 里没有 `[channelType]` 时，配置写了 Type 即放行）。
- 城镇是 `Town/Fair_PVP.twn`（**town 10**），随客户端发布，`list/town.lst` + `map/fair_pvp/*.map` 都在。
  但 `ChannelTowns` 只从 `clientchannelinfo.etc` 的 `[seriaRoomTown]` 构建，**补不到 type 13**，
  所以需要在服务端本地补一条落点（见 §3）。

## 2. 频道类型与 opcode

```
C→S（cmd 表）                      S→C（noti 表）
  50 MAKE_PVP_ROOM                   41 PVP_ROOM_INFO     45 START_PVP
  51 ENTER_PVP_ROOM                  42 PVP_ROOM_STATE    46 DIE_PVP_CHARACTER
  52 SET_PVP_SEAT_STATE              43 PVP_SEAT_STATE    47 END_PVP
  53 SET_PVP_READY_STATE             44 PVP_READY_STATE   48 PVP_RECORD
  54 SET_PVP_TEAM_MODE                                  49 REQ_PVP_RANK
  55 DIE_PVP_CHARACTER   110 PVP_HEART_BEAT   112 PVP_REQUEST_FIGHT
  56 PVP_TIME_OUT        195 PVP_CHANNEL_INFO  298 COMPLETE_LOAD_PVP
  57 END_PVP_RESULT      299 CONNECT_P2P_PVP   1285 JOUST_INFO
  58 RES_PVP_RANK   59 SET_PVP_MAP_INDEX
```

**cmd 表与 noti 表是两套独立命名空间，数字会撞车**（例：`cmd 43 = GET_ITEM`，
`noti 43 = PVP_SEAT_STATE`）。实测约定：

- **应答走同号（cmd 表）**：`output.send(kind=1, 同号, body)`，首字节 `1` = 成功。
  实机证据：`c2s 44 USE_STACKABLE` → `s2c 44`（前 12 字节回显请求）。
- **广播/主动推送走 noti 表**：`output.send(kind=0, noti号, body)`。
- **错误应答**：`0 + u16 错误码`（与既有 `protocol.Refusal` 同形）。
  实机确认客户端能解析：`[RECV] ENUM_CMDPACKET_MAKE_PVP_ROOM (Size : 8) Result : Error ErrCode : 19`。

## 3. 本批实机采样到的真实请求体

⚠️ **115 的 PvP 命令普遍"定长 + 尾部填充"，且字段宽度与 90 级参考实现不同。**
按 90 级的 `len(b) != N` 严格校验会**误拒客户端自己的合法请求**（本批踩到 3 次）。
实现一律采用「**只读需要的字段、不卡长度**」。

| cmd | 实测明文 | 解析 |
|---|---|---|
| 50 | `08 00 00 00 00 00 00 00` | `NameType=8`(预设名序号) `Map=0` `Pwd=0` **`SpecialMode=0`** `Flag=0` + 2B 尾 |
| 50 | `00 \| 11 00 00 00 \| "In Arena Training" \| 00 00 \| 00 \| 01 \| 00 \| 00×5` | `NameType=0` + 自定义名(17B) … **`SpecialMode=1`** + 5B 尾 |
| 52 | `00 03 \| 00 00` | `seat=0 state=3` + 2B 尾 |
| 52 | `01 fe \| 00 00` / `06 fe \| 00 00` | `seat=1/6 state=254`(ClosedSeat) |
| 52 | `00 02 \| 00 00` | `seat=0 state=2` |
| 54 | `01 00 00 00 \| 00×12` / `03 00 00 00 \| 00×12` | **`mode` 是 u32**（实测 1 和 3）+ 12B |

**`SpecialMode` 取值**（本批实测 + 90 级线索交叉验证）：

- `0` = 普通房间（90 级 `Manager.Create` 吃的就是这支）——**客户端点「创建房间」默认发这个**
- `1` = 练习房间（90 级 `CreatePractice`；房主占 0 号位，1..7 号位置 `ClosedSeat=254`）
- `3` = 街机（单人打 APC；客户端本地跑 AI 与伤害，服务端只记流程）——**本批未实现，显式拒绝**

**座位状态实测出现过 2 和 3**（90 级只定义了 `Wait=1 / Fight=2 / Empty=255 / Closed=254`），
说明 115 还有别的状态值；当前实现只对"有人坐的座位"限 `state <= 4`。

**`SpecialMode` 之外的两处宽度差异**：`cmd54` 的 mode 是 **u32**（90 级是 u8，且只允许 1/2，
而 115 实发 3），已放宽到 1..4 并显式排除服务端内部的 `PracticeMode(6)`/`ArcadeMode(10)`。

## 4. 服务端实现落位

| 文件 | 职责 |
|---|---|
| `internal/pvp/wire.go` | `ParseMake` / `ParseEnter` + `RoomList`(41) / `RoomState`(42) / `Seats`(43) / `EnterSuccess`(51应答) / `UserState`(3) / `RefusalBody` |
| `internal/pvp/room.go` | `Manager`：房号自增、8 座位、房主转移、空房删除、`Create` / `CreatePractice` / `Join` / `Leave` / `SetSeat` / `SetMode` / `SetMap` / `Ready` |
| `cmd/wireprobe/pvp_flow.go` | `dispatchPvp`：cmd 50/51/52/53/54/59/298 |
| `cmd/wireprobe/client_dispatch.go` | 把 `dispatchPvp` 注册进 `commandDispatch`（排在 `dispatchDungeon` 之前） |
| `internal/gamedata/catalogs_channel.go` | 本地补 type 8/13 → town 10 的落点 |
| `configs/channel.local35.json` | 新增 `{ID:21, Name:"Free PvP", Type:13}`（频道数 50 → 51） |

**建房（cmd50）成功后的下发序列**（照 90 级 `pvp_rooms.go`，本批只实机验到广播被客户端接受）：

1. 广播 `noti41` 房间列表
2. 广播 `noti3` 用户状态
3. **cmd50 本身不发单独应答**；只有 **cmd51（进房）**才回 `EnterSuccess`（`1` + 8 个准备标志）

## 5. 验证

- `go build` / `go vet` 通过；`go test ./internal/pvp/` **11 个用例全过**，
  其中 3 个直接跑在本批实机采样字节上（`TestParseMakeRealClientSample` /
  `TestParseMakeOrdinaryRoomSample` / `TestSetSeatAndModeRealSampleShapes`）。
- 实机（业主操作）：
  - 频道 21 / type 13 进入后角色落在 **town 10**（`events.jsonl`：`channel_town_spawn channel=21 channel_type=13 town=10`），
    客户端 `change module → MODULE_TYPE_TOWN` 且 `focus closer popup window : POPUP_WINDOW_TYPE_PVP_TOWN`。
  - 点「创建房间」→ 服务端 `pvp_room_created room=1 mode=2`（业主确认"进去了"）。
  - 应答链路：客户端 `[RECV] ENUM_CMDPACKET_MAKE_PVP_ROOM Result : Error ErrCode : 19`（该次是修正前的误拒）。

## 6. 未闭环 / 风险

1. **战斗期未实现**：cmd 55/56/57/58/110/112/195/299/1285 只记录 `${kind: pvp_command_unimplemented}`，
   不响应也不伪造。开打流程（`noti45 START_PVP` → 加载 → P2P）没有实测样本。
2. **街机（SpecialMode=3）未实现**，当前显式拒绝。
3. **多客户端广播未接**：现在的广播只发给发起者自己（单机够用）。
   真人对战要接 `gatewayRuntime` 的连接表 + UDP 定向中继（115 目前**没有任何** UDP 中继设施）。
4. **`Open Fail IRDPopupWindow Type : 13`**：进决斗场城镇后客户端每 0.5 秒失败一次（普通频道时该
   type 可正常 Open）。属客户端自身 UI 窗口，**与本批包无关**（客户端全程未发 cmd195）。
   原因未查明，待后续取证。
5. 客户端全程未发 cmd 51 / 53 / 298 / 299 —— 单人房主场景下可能是正常的，**未证实**。
