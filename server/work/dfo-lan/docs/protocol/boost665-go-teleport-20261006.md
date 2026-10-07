# 665 挑战面板 Go 按钮传送落点修复（2026-10-06，实机收口）

> 业主口径链：「传送的位置不对」→（修好并验落点后）「实机OK」。
> 现役确认基线：`server/work/dfo-lan/bin/wireprobe-pvf.exe` = `d28c08014123082bca9d611aad168ce6ff31cbe481f57aa11cd164467a654f9e`
> （30,218,752 B）。旧基线 `164780f1c5c3f8c8…`（662 直升主线清除那次）备份在
> `server/work/dfo-lan/.tmp/boost665-goteleport-20261006/baseline-164780f1.exe`，回退只需把它覆盖回去。

## 1. 症状与取证

- 面板：**Sky of a Thousand Seas BOOST UP**（2026-08-04 → 2026-11-17），`Clear Endkeeper of Order 0/10` 行的 **Go** 按钮，
  确认框「Move to Endkeeper of Order dungeon?」点击后玩家被丢到 **38/0 new_elvengard**，不是最终调律者入口镇。
- 修复前的命中帧（同一原帧，多次复现）：

  | 会话 | 时刻 | 入站 CMD36（plain_hex） | 我方存档结果 |
  | --- | --- | --- | --- |
  | `…_20261006_195540_767216_next37` | 12:15:57 / 12:16:17 | `f1000000010000008f00ad00052600000001000000000000` | 38/0 @1677,222（Seria 离场存档点） |
  | 同上 | 12:16:13（人在 38/0、无 Return 存档） | 同帧，prevArea=0 | `area_refused: no authorized source portal to destination` |
  | `…_20261006_212930_180742_next37` | 13:36:52 / 13:36:59 | 同帧 | 38/0 @1677,222；客户端 CMD35 回报 `8d06de0005730000`＝1677,222（服从了我方） |

- 帧解码（`protocol.AreaChangeRequest`）：town `0x000000f1`=**241**、area **1**、x `0x008f`=**143**、y `0x00ad`=**173**、
  flag **5**、prevTown 38、prevArea 1、tailFlags `00 00`。

## 2. 源真值（唯一判据）

`live/event/kor/2026/0326_boostup/boostupspecupchallenge.evt` 的 `[challenge info]` 每个 `[info]` 行自带：

| 行 `[no]` | `[challenge type]` | `[go contents town area]` | `[go contents channel type]`（未消费） | `[party create npc index]`（未消费） | `[go contents dungeon index]` |
| --- | --- | --- | --- | --- | --- |
| 0 | `clear endkeeper of order` | **241 1 143 173** | 22 | 100002681 | 100005014 |
| 1 | `clear higher or legion` | 214 1 500 255 | 103 | 100002636 | 100004132 |
| 2 | `clear raid` | 228 1 143 173 | 120 | 100002894 | 100004604 |
| 3 | `clear higher or legion` | 220 1 1663 238 | 108 | 100002769 | 100004520 |

- 行 0 的源值与实机 CMD36 逐字节相同 ⇒ Go 按钮的落点是**源定义**，不是客户端任意坐标。
- 241/1 = `commonmap/town/260326_skyofathousandseas/finalsanctuary/finalsanctuary_1.map`，kind `[dungeon gate]`、
  `[need level] 115`、`[virtual movable area]` = `[[34 211 1050 220] [157 153 300 80] [452 182 560 50]]`
  （143,173 由 `WalkableTolerance` 128 收进首行），出边 `241/1 → 241/0`，pending `dynamic portal destination`。
- 取源命令（只读）：
  `go run ./cmd/dfo-tool pvfinspect -source ../client-build/Script.inner.pvf -find nomatch -files live/event/kor/2026/0326_boostup/boostupspecupchallenge.evt -tokens -output runtime/pvf665`

## 3. 根因

`internal/world/service.go` 的 Seria 房离场判据：

```go
mapTeleport := r.Flag == 5 && (r.TailFlags[0] == 5 || r.TailFlags[1] == 5)
seriaLeave := old.Return != nil && src.SeriaReturnWarp && !mapTeleport && !(同区)
```

事件 Go 按钮的帧是 **flag 5 + tail 0 0**，与「走到 Seria 门口出镇」的帧形状一致 ⇒ `seriaLeave` 把目的地与坐标
整体改写成存档 Return。**结论：flag/tail 不能区分「门走」与「事件/地图选择器传送」**，不能靠补标志位判据；
必须按源行授权目的地。没有 Return 存档时又落到普通门控（38/1 非 permissive）被拒。

## 4. 接线（源 → reader → 领域规则 → 执行）

- reader：`boostup.ParseChallenges` 新增 `ChallengeDefinition.GoTarget [4]uint32` + `HasGoTarget`
  （四个数全为 type-0 且 ≥0，x/y ≤65535 才成立；缺行或形状不合＝无该目标，不影响其余字段解析）。
- 领域/执行：`cmd/wireprobe/boostup_challenge.go:boostChallengeGoTeleport(r, specialWarp)`
  ＝ `specialWarpPending` 未置位 + `ownedTownTeleport`（城镇角色、非副本、非选关、prev==当前、flag 5 tail 0、角色归属）
  + 挑战 `Enrolled` + 该行 `Unlocked` + 请求 town/area **等于源 GoTarget**；命中后**落点取源 x/y**（客户端改坐标不能选目的地），
  再复用既有 `teleportTransition`（区域存在、`RequiredLevel`、`ValidatePosition` 可行走、目的为 Seria 房才打 Return 存档）。
  存档读取失败视为「不属这一支」，交回原路径，避免把 NPC 移动/章节回城一起拒掉。
- 调用点：`cmd/wireprobe/odyssey_teleport.go:areaTransition`，排在 `boostAreaTransition` 与
  `npcMoveTeleport/episodeTownReturn` 之后、specialWarp/地图传送分支之前。未命中＝逐字节保持原行为。
- 重复规则检查（§0.2 第 2 条）：本轮之前 Go 目标在 Go/JSON policy 里**没有任何平行定义**，属净新增源读取；
  `[go contents channel type]`、`[party create npc index]` 仍未消费（见 §6）。

## 5. 验证

- `go build ./...` 退出 0；`go vet ./...` 无输出；`go test ./... -count=1` 全部包 `ok`，失败集合与空基线逐名一致。
- 新增/扩展测试：
  - `cmd/wireprobe/boostup_challenge_teleport_test.go`(新，5 例，用实机原帧)：源落点、忽略客户端 x/y、
    未解锁行仍走旧改写、源未登记的 town/area 不走此支、副本内角色拒绝。
  - `internal/boostup/challenge_test.go`：真实导出令牌断言四行 `GoTarget` 与源表逐值相同
    （`US115_TEST_BOOST_CHALLENGE=runtime/pvf665 go test ./internal/boostup -run TestBoostChallengeSourceAndLifecycle`）。
- 实机（业主操作）2026-10-06 14:06:28，会话 `runtime/roles_…_20261006_220240_888999_next37`：
  CMD36 原帧 → `world_position_saved` **241/1@143,173** → `area_change_sent` `finalsanctuary_1.map`
  → CMD35 连续回报 158,173 / 230,221 / 223,259（玩家在入口镇内走动），本场无 `area_refused`。

## 6. 未闭环

1. 胶囊城镇 222 内点 Go：`boostAreaTransition` 的毕业分支只认存档 Origin，仍会拒（实况路径是 38/1、38/0 → 241/1，已覆盖）。
2. 毕业离开胶囊城镇时该分支**丢弃客户端 x/y**，用 Origin 落地（13:32:44：请求 38/1@544,311 → 存成 557,210）；
   两点都在 Seria 房可行区内，先记缺口，需单独取证再动。
3. `[go contents channel type]`（22/103/120/108）与 `[party create npc index]` 是否要求切频道/建队，未取证。
4. 665 行 1/2/3 的通关计数仍缺「副本→内容号」真源（与本日早些条目同源）。
5. 根工作区无 `.git`（`git rev-parse` 报 `not a git repository`）⇒ 本轮无法提交；改动只在工作区与本文档。
   桌面交付包 `662与665活动今日修改-20261006.zip`（另一会话，22:0x 打包）**不含**本轮 4 个文件＋本说明，
   现役 exe 已含本轮改动 ⇒ 需要转交源码时按 §7 清单补打。

## 7. 本轮改动文件清单

- `server/work/dfo-lan/internal/boostup/challenge.go`
- `server/work/dfo-lan/internal/boostup/challenge_test.go`
- `server/work/dfo-lan/cmd/wireprobe/boostup_challenge.go`
- `server/work/dfo-lan/cmd/wireprobe/odyssey_teleport.go`
- `server/work/dfo-lan/cmd/wireprobe/boostup_challenge_teleport_test.go`（新）
- `CHANGELOG`（2026-10-06（晚）小节，实机收口）
- `server/work/dfo-lan/bin/wireprobe-pvf.exe`（`d28c0801…`，入库与否按 §0.3.3 由业主决定）
