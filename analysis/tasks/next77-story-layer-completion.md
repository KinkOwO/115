# 剧情副本（城主宫殿 / quest 3191）通关不弹结算 — 修复记录

状态：代码完成、离线端到端（真实 catalog）通过；**待用户实机验证**。
对外部修复包 `115us-城主宫殿剧情修复-20260923` 的处理：**不照抄**，两项刻意不采纳（见 §5）。

## 1. 症状与真源

- 副本 15 迷宫 6（`dungeon/act2/palaceofload.dgn`，quest **3191**）能走完整条剧情，但**翻牌结算界面不出、Clear 按钮不亮**。
- 真源：`configs/dungeons.full.json`（本仓库**已入库**，308 MB）。实测该迷宫：

```
maze 6: quest=3191 start=(0,1) boss=(0,0) rooms=2 [(0,1)=57980, (0,0)=57981]
layer pos=(0,0) maps=[100008695, 100008694, 100008684, 100008683]   ← 末图 100008683
末图 100008683 的 fixedMonsters（7 只）：
  idx=4 template=63821  rank=3 team=100 nonCombat=true   ← [displayhuntdummy] 剧情替身
  其余 6 只均 team=0 nonCombat=true
```

## 2. 完成链与两个断点

```
客户端 CMD117 BOSS_CHECK ─► BossCheck() 记 completionTarget（唯一赋值来源）
                            └─► tryComplete() ─► Completed()
                                  └─►（CMD37 钩子）completeDungeon()
                                        ├─ CMD115 boss_check_confirmed ← BossCheckConfirmed(CompletionTarget())
                                        └─ CMD31  dungeon_clear_enabled ← 点亮 Clear
                                              └─ 玩家点 Clear → CMD46 ─► dungeonResult()（要求 Completed() && completionSent）→ 结算
```

- **断点 1（判定）**：末图唯一的 rank-3 是 `[displayhuntdummy]` 替身，随 `NonCombat=true` 生成 ⇒ 客户端**不为它发 CMD117** ⇒ `completionTarget` 恒 0 ⇒ 旧 `tryComplete()` 在 `completionTarget == 0` 时直接 `return`（只放行副本 26 专用分支）⇒ `completed` 永远 false。
  - ⚠️ 判据必须看 `NonCombat`，**不能看 `Rank`**（它的 `Rank` 确实是 3）。
- **断点 2（签发）**：即使判定通过，旧 `CompletionTarget()` 在 `completionTarget == 0` 时只回落到副本 26 专用查询、返回 0；而 `protocol.BossCheckConfirmed(0)` 会报 `invalid boss identity` ⇒ `completeDungeon()` 返回 error ⇒ **CMD115 与 CMD31 整批被丢弃**，日志里只剩一条错误事件（`dungeon_completion_error`，随本轮之前的改动已有），表现是"什么都没发生"。

## 3. 改法（唯一文件：`internal/dungeon/completion.go`）

新增语义化判据并在 `completionTarget == 0` 分支加通用剧情层收口：

| 方法 | 含义 |
|---|---|
| `hasKillableBoss()` | 房内是否有客户端会为其发 CMD117 的 rank-3（`Rank==3 && !NonCombat`） |
| `atLayerFinalMap()` | 当前房是否位于**所属** layers 序列的末图 |
| `hasLayerEntry()` | 当前房是否属于某个 layers 序列（原先内联的同类循环改为调用它） |
| `roomSettled()` | 已加载且房内**可击杀**敌人全灭 |

```go
if s.completionTarget == 0 {
    // 副本 26 迷宫 3 末图：相反形状，显式排除（见 §4）
    if s.Definition.ID == 26 && s.Maze.Index == 3 && s.Room.Map == 100008697 { ...; return }
    if s.hasKillableBoss() || !s.atLayerFinalMap() { return }   // 负向：真 Boss 房 / layers 非末图
    if !s.roomSettled() { return }                              // 负向：有活怪
    s.completed = true
    return
}
```

`CompletionTarget()` 改为回落到 `storyDisplayTarget()`：取房内**可编身份**（`Entity != 0 && != 65535`）的 rank-3 实体，**team100 优先**，team0 剧情 Boss 仅作兜底。

## 4. 不回归 lotus（副本 26 迷宫 3）

副本 26 迷宫 3 的末图是**相反形状**：进入时带活体战斗目标（`0x102d`），由结尾 `[CHANGE MAP]` 过场回到缓存末图才收口 —— 清场瞬间若按通用判据收口，会在过场播放前就下发通关。因此该末图**显式排除**在通用判据之外（`tryComplete()` 里先判、直接 return），原 lotus 判据与上报身份（team100 的 `0x1030`）语义**逐字不变**。

- team100 优先这一条不能丢：末图 NOTI29 里 team0 的剧情 Boss 排在更前面，若按"第一个 rank-3"取，上报身份就会错 —— `TestLotusClosingCinematicReusesFinalLayer` 钉住了这一点。
- `reportableLotusTarget()`（原来自己又判一遍副本/迷宫/地图）与原 `CompletionTarget()` 分支收敛为 `storyDisplayTarget()`，判据只留在一处。

## 5. 与外部修复包的刻意分歧（两项不采纳）

| 包内改动 | 结论 | 理由 |
|---|---|---|
| `session.go: RoomCleared()` 新增 `if !s.hasLiveEnemy() { return true }` | **不采纳** | 实测迷宫 6 四个层图 `warpKeyRoom()=false` 且全部怪 `NonCombat`，**既有循环已经返回 true**（`(!m.NonCombat \|\| m.Rank==3 && keyRoom)` 对该数据为假）。该早退只会在 `keyRoom` 房间引入**无证据**的行为变化。 |
| `pendingCompletionCheck` 待检标记 + `dungeon_flow.go`（`FlagCompletionCheck`）/`finishDungeonLoading` 改造 | **不采纳** | 本仓库 `main.go` CMD37 钩子（`Loaded=true` → `TryComplete()` → `completeDungeon()`）已经在**会话交接之后**的新会话上判定（`activeDungeon = pending` 发生在同一个 CMD45 帧的发送之后），标记机制冗余，并且会引入第二个判定入口。判定集中在 `tryComplete()` 内，**未在切图点硬编码地图号**。 |

包内 `main.go` 的"不吞错"（`dungeon_completion_error`）本仓库**此前已有**，无需再改。

## 6. 验证

- 修复前逐图重放（真实 catalog + 真实地图脚本）：`100008695/100008694/100008684/100008683` 四图全部 `completed=false target=0` → 症状复现。
- 修复后：前三图仍 `false`，末图 `completed=true`、`target=4100`（= 展示替身实体）、`BossCheckConfirmed(target)` 编码成功（4 字节）。
- `100008683` 的 `RoomCleared()=true`、`roomSettled()=true`（全 `NonCombat`），无需改动 `RoomCleared`。
- `go vet ./internal/... ./cmd/...` 0 项；`go test -count=1 ./internal/... ./cmd/...` **全绿**（含既有 maze4 用例 `TestStandardDungeonCastellanChamberLayerTransitions`、lotus 用例与 Odyssey 场景）。
- 新增用例：
  - `internal/dungeon/story_layer_completion_test.go` 8 例 —— 正向 1（无 CMD117 也能完成且身份可编）、负向 6（真 Boss 房 / 普通房间 / layers 非末图 / 有活怪 / 有活怪门不放行）、lotus 例外 1（清场不得完成、只有过场后完成）。
  - `internal/dungeon/story_completion_target_test.go` 1 例 —— 读**真实 catalog**（`../../configs/dungeons.full.json`，缺失则 Skip），逐图断言"只有末图可完成"、断言末图确实只带非战斗 rank-3 替身、断言上报身份可被 `BossCheckConfirmed` 编码。
- 候选程序：`bin/wireprobe-handoff-source.exe`，SHA256 `D31F9FDCE38E3874FBE6DDE5A17EAE1920797097DE84F350011AD9419733051D`（改前备份 `D:\115us\.workbuddy\tmp\wireprobe-before-storyfix.exe`）。

## 7. 实机验收判据（待用户操作）

跑一次城主宫殿剧情副本后，在最新 `server/work/dfo-lan/runtime/roles_persist_*/events.jsonl` 中核对：

| 事件 | 期望 |
|---|---|
| `boss_check_confirmed`（CMD115） | ≥ 1，`plain_hex` 为可解身份 |
| `dungeon_clear_enabled`（CMD31） | ≥ 1 |
| `dungeon_clear_reward`（CMD46） | ≥ 1（玩家点 Clear 后） |
| `dungeon_completion_error` | 0 |

表现层面：Clear 按钮点亮、点击后弹出翻牌结算即通过。

## 8. 已知未决（与本次修复无关，不猜）

- **CMD623**（剧情结束 / 完成任务按钮）在本仓库 `cmd/wireprobe/request_scope.go` 的 `dungeonRequest` 与 `main.go` 的 handler switch **都没有分支** ⇒ 被静默丢弃。若通关后任务栏仍有异常，嫌疑在此；需要新的取证（IDA + 实机帧）后才动，按纪律不猜包。
- lotus（副本 26）**结尾黑屏**仍是未解决问题（任务可完成、画面黑屏），见 `analysis/tasks/lotus-terminal-layer-20260923.md`；本轮不触碰。
