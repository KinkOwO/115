# 剧情副本（城主宫殿 / quest 3191）通关不弹结算 — 修复记录

状态：**用户实机确认通过**（2026-09-23 晚，任务正常通过、结算正常弹出）。
对外部修复包 `115us-城主宫殿剧情修复-20260923` 的处理：**不照抄**，两项刻意不采纳（见 §5）。
合并上游 `origin/main`（`fc94907`）时发现上游 `b06532b` 已独立修了**同一类**问题，两套判据互斥互补 —— 见 §6。

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
- **断点 2（签发）**：即使判定通过，旧 `CompletionTarget()` 在 `completionTarget == 0` 时只回落到副本 26 专用查询、返回 0；而 `protocol.BossCheckConfirmed(0)` 会报 `invalid boss identity` ⇒ `completeDungeon()` 返回 error ⇒ **CMD115 与 CMD31 整批被丢弃**，日志里只剩一条错误事件（`dungeon_completion_error`），表现是“什么都没发生”。

## 3. 改法（唯一文件：`internal/dungeon/completion.go`）

| 方法 | 归属 | 含义 |
|---|---|---|
| `atLayerFinalMap()` | 本仓库新增 | 当前房是否位于**所属** layers 序列的末图（任一匹配 layer 不是末图即 false；不属于任何 layer 也是 false） |
| `hasLayerEntry()` | 本仓库新增 | 当前房是否属于某个 layers 序列（原 `tryComplete()` 尾部的内联循环收敛为调用它） |
| `hasFightableBoss()` | 上游 `b06532b` | 房内是否有可战斗真 Boss（`!NonCombat && Team!=0 && (Rank==3 \|\| APC&&rank5..8)`） |
| `roomEnemiesDead()` | 上游 `b06532b` | 房内**敌对**敌人全灭（`Team!=0 && !NonCombat && !Dead`） |
| `reportableDisplayBoss()` | 上游 `b06532b` | 房内可上报的 rank-3 展示 Boss（`Rank==3 && Team!=0`，身份可编） |
| `atSourceBossMap()` | 上游 `b06532b` | Boss 坐标上的 Boss 房，且该坐标**不属于**任何 layer |

```go
if s.completionTarget == 0 {
    // 副本 26 迷宫 3 末图：相反形状，先于所有通用判据短路（见 §4）
    if s.Definition.ID == 26 && s.Maze.Index == 3 && s.Room.Map == 100008697 { ...; return }

    // 上游判据：Boss 坐标上的 Boss 房（非 layer 场景，副本 22/25/27）
    if s.Loaded && s.atSourceBossMap() && !s.hasFightableBoss() && s.roomEnemiesDead() && s.reportableDisplayBoss() != 0 {
        s.completed = true
        return
    }
    // 本仓库判据：layer 序列末图（副本 15 迷宫 6 城主宫殿）
    if s.Loaded && s.atLayerFinalMap() && !s.hasFightableBoss() && s.roomEnemiesDead() && s.reportableDisplayBoss() != 0 {
        s.completed = true
        return
    }
    return
}
```

`CompletionTarget()` 沿用上游实现：`completionTarget == 0` 时回落 `reportableDisplayBoss()`（非 lotus 收口时），再回落 `reportableLotusTarget()`。判据集中在 `tryComplete()` 内，**未在切图点硬编码地图号**。

## 4. 不回归 lotus（副本 26 迷宫 3）

副本 26 迷宫 3 的末图是**相反形状**：进入时带活体战斗目标（`0x102d`），由结尾 `[CHANGE MAP]` 过场回到缓存末图才收口 —— 清场瞬间若按通用判据收口，会在过场播放前就下发通关。因此该末图在 `completionTarget == 0` 分支**开头先于所有通用判据短路 return**（本仓库新增的保护），lotus 自身的判定与上报身份仍走 `reportableLotusTarget()`（team100 的 `0x1030`），语义**逐字不变**。

- team100 优先这一条不能丢：末图 NOTI29 里 team0 的剧情 Boss 排在更前面，若按“第一个 rank-3”取，上报身份就会错 —— `TestLotusClosingCinematicReusesFinalLayer` 钉住了这一点。
- 上游原先把 lotus 分支放在 `atSourceBossMap` 分支**之后**；本仓库把等价的 lotus 判定提到开头，避免 lotus 场景被通用判据劫持。

## 5. 与外部修复包的刻意分歧（两项不采纳）

| 包内改动 | 结论 | 理由 |
|---|---|---|
| `session.go: RoomCleared()` 新增 `if !s.hasLiveEnemy() { return true }` | **不采纳** | 实测迷宫 6 四个层图 `warpKeyRoom()=false` 且全部怪 `NonCombat`，**既有循环已经返回 true**（`(!m.NonCombat \|\| m.Rank==3 && keyRoom)` 对该数据为假）。该早退只会在 `keyRoom` 房间引入**无证据**的行为变化。 |
| `pendingCompletionCheck` 待检标记 + `dungeon_flow.go`（`FlagCompletionCheck`）/`finishDungeonLoading` 改造 | **不采纳** | 本仓库 `main.go` CMD37 钩子（`Loaded=true` → `TryComplete()` → `completeDungeon()`）已经在**会话交接之后**的新会话上判定（`activeDungeon = pending` 发生在同一个 CMD45 帧的发送之后），标记机制冗余，并且会引入第二个判定入口。 |

包内 `main.go` 的“不吞错”（`dungeon_completion_error`）本仓库**此前已有**，无需再改。

## 6. 与上游同日同类修复的关系（关键，勿误删）

上游 `b06532b`（“宣战概要面板已读状态”那一批）在同一文件 `completion.go` 独立修了**「无 Boss 级实体的 Boss 房」**这一类：

- 上游判据 `atSourceBossMap()` 要求 `Room.Boss && position == Maze.Boss`，且**该 position 不属于任何 layer**（其 `display_boss_completion_test.go` 明确断言 `len(maze.Layers)==0`），典型案例副本 25 迷宫 2 / 地图 53500；文档侧对应 `docs/dungeon/TODO-当前服务端补齐.md` 的 01/02/03（副本 22/25/27）。
- 本仓库判据要求**位于某个 layer 的末图**。
- 城主宫殿迷宫 6 只有 **1 个 layer 条目**（4 张图共用 position `(0,0)`），末图落在 layer 坐标上 ⇒ 上游 `atSourceBossMap()` 恒为 false，**上游修复不覆盖本症状**；反之副本 25 那条无 layer，`atLayerFinalMap()` 为 false。
- 结论：两条**互斥且互补**，都必须留。合并时以**上游代码为基线**，本仓库判据作为额外分支插入；上游函数、注释与判定顺序逐字保留（唯一改写是把上游 lotus 分支提到分支开头短路，条件组合不变）。
- 上游 TODO 里「无 Boss 级实体的 Boss 房」仍写着“建立逐类证据清单、不得仅凭目录统计批量放行”；本轮实测补上了 **layer 末图**这一形状。

## 7. 验证

- 修复前逐图重放（真实 catalog + 真实地图脚本）：`100008695/100008694/100008684/100008683` 四图全部 `completed=false target=0` → 症状复现。
- 修复后：前三图仍 `false`，末图 `completed=true`、`target=4100`（= 展示替身实体）、`BossCheckConfirmed(target)` 编码成功（4 字节）。
- `100008683` 的 `RoomCleared()=true`、`roomEnemiesDead()=true`（全 `NonCombat`），无需改动 `RoomCleared`。
- 合并上游后：`go build ./...` 通过、`go vet ./internal/... ./cmd/...` 0 项、`go test -count=1 ./internal/... ./cmd/...` **全绿**（含上游新增 `display_boss_completion_test.go` / `apc_boss_completion_test.go`、既有 maze4 用例 `TestStandardDungeonCastellanChamberLayerTransitions`、lotus 用例与 Odyssey 场景）。
- 新增用例：
  - `internal/dungeon/story_layer_completion_test.go` 8 例 —— 正向 1（无 CMD117 也能完成且身份可编）、负向 6（真 Boss 房 / 普通房间 / layers 非末图 / 有活怪 / 有活怪门不放行）、lotus 例外 1（清场不得完成、只有过场后完成）。
  - `internal/dungeon/story_completion_target_test.go` 1 例 —— 读**真实 catalog**（`../../configs/dungeons.full.json`，缺失则 Skip），逐图断言“只有末图可完成”、断言末图确实只带非战斗 rank-3 替身、断言上报身份可被 `BossCheckConfirmed` 编码。

## 8. 实机确认（用户已确认）

用户在候选程序上实机跑完城主宫殿剧情副本：**任务正常通过、翻牌结算正常弹出**。回归判据（最新 `server/work/dfo-lan/runtime/roles_persist_*/events.jsonl`）：

| 事件 | 期望 | 状态 |
|---|---|---|
| `boss_check_confirmed`（CMD115） | ≥ 1，`plain_hex` 为可解身份 | 通过 |
| `dungeon_clear_enabled`（CMD31） | ≥ 1 | 通过 |
| `dungeon_clear_reward`（CMD46） | ≥ 1（玩家点 Clear 后） | 通过 |
| `dungeon_completion_error` | 0 | 通过 |

## 9. 已知未决（与本次修复无关，不猜）

- **CMD623**（剧情结束 / 完成任务按钮）在本仓库 `cmd/wireprobe/request_scope.go` 的 `dungeonRequest` 与 `main.go` 的 handler switch **都没有分支** ⇒ 被静默丢弃。若通关后任务栏仍有异常，嫌疑在此；需要新的取证（IDA + 实机帧）后才动，按纪律不猜包。
- lotus（副本 26）**结尾黑屏**仍是未解决问题（任务可完成、画面黑屏），见 `analysis/tasks/lotus-terminal-layer-20260923.md`；本轮不触碰。
