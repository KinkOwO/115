# next147 — 千海天深渊：直读模式下的玩法修复落地

> 2026-10-01。依据业主提供的取证文档《千海天深渊 · 核心玩法与掉落（实证定义）》
> （原文见当日对话附件），按「先核实现状、再对照落实」的方式在**PVF 直读模式**下重做。
> 承接 `next141`（内层自动生成）/`next142`（校验锁自动派生）/`next145`（发布默认程序）/`next146`（存档身份解耦）。

## 0. 结论速览

| # | 文档里的修复点 | 直读模式现状（核实结果） | 本轮处置 |
| --- | --- | --- | --- |
| 1 | **进货疲劳**：源 `[use fatigue only start dungeon]`（`only start` = 进本只收一次） | **整套丢失** —— `DungeonDefinition.EnterFatigue`、`FatigueService.EnterCostFor`/`EnterRoomForDungeon`、`storage.RunPaidFatigue` 全都不存在，记费仍走本地策略「每房间 1 点」 | ✅ 重新实现（源声明优先，策略兜底，见 §2） |
| 2 | **奖励表漏 3 张**（`100004766/767/768` 无奖励数据） | **仍缺** —— 直读已能扫全 `etc/rewardboostinfo`，但内容策略只列了 4 个副本号 | ✅ 策略补到 7 个（见 §3） |
| 3 | `[special setinfo reward]` 柱型选择（`PillarForRun`/`PillarForDrops`） | **全仓没有任何消费点**：`dungeon_special_reward.go` 不存在，`[special setinfo reward]` 仅出现在普通掉落组的解析测试里 | ⏸ **未实现**（见 §4：先确立消费点再谈） |
| 4 | 三种发放机制（fixed / additional / coupons / hidden） | 直读已实现（`internal/loot/attunement*.go`，模型 `rewardboostinfo-ctp-v1`） | 无需改动 |

## 1. 进货疲劳：核实与实现

### 1.1 核实

- `DungeonDefinition` 无 `EnterFatigue`；`FatigueService` 只有 `EnterRoom`（每房间收 `Rules.RoomCost`）；
- 三处**进本准入**（`dungeon_flow.go` 选图/直接移动、`card_flow.go` 卡片流）都以
  `w.fatigue.Rules.RoomCost > 0` 为判据 —— 而本地策略 `configs/fatigue-probe.json` 的 `room_cost` 是 **0**，
  意味着这套检查**整体被跳过**；
- ⇒ **记费口径与准入判断不一致**，正是文档 §九 描述的「客户端已进本、才在加载阶段被拒 ⇒ 卡在加载界面」的温床。

### 1.2 实现（源优先 + 策略兜底）

| 面 | 改动 |
| --- | --- |
| 目录 | `internal/catalog/dungeons.go`：`DungeonDefinition.EnterFatigue uint16`，用 `sectionCells(cells, "[use fatigue only start dungeon]")` 解析（该段紧随闭标签，恰好只收到一个数）；新增 `DungeonCatalog.DeclaredEnterFatigue()` 汇总声明值供启动日志核对 |
| 疲劳服务 | `internal/character/fatigue.go`：注入点 `EnterFatigueOf func(uint32) uint16`、`EnterCostFor(dungeonID)`（准入与记费**共用**口径）、`EnterRoomForDungeon(...)`（源声明的副本**只在第一次进本收官方值**，同 run 后续房间收 0；未声明的副本保持「每房间收本地值」原行为） |
| 存储 | `internal/storage/fatigue.go`：`RunPaidFatigue(ctx, id, run)`（该 run 是否已有 `cost>0` 的房间记录） |
| 接线 | `cmd/wireprobe/main.go` 在 `dungeonCatalog` 就绪后注入 `EnterFatigueOf`：**源声明优先**，源没有该段时读内容策略的 `dungeon_enter_fatigue` 兜底 |
| 准入 | `dungeon_flow.go`（选图/直接移动）、`card_flow.go`：判据改为 `w.fatigue.EnterCostFor(<dungeonID>) > 0` —— **与记费同口径** |
| 兜底 | `dungeon_flow.go` 的 `finishDungeonLoading`：记费失败**不再 `return nil, e`**，而是把加载应答发完、立刻 `leaveDungeon()` 送回城镇并清 `activeDungeon`（文档 §九 的原则：**限制必须在进入之前生效，绝不能发生在副本里面**） |
| 策略 | `configs/pvf-mine-policy.json` 新增 `dungeon_enter_fatigue`（8/8/8/8/30/6 等，取自源声明，等价于文档 §二的实测值） |

### 1.3 验证（2026-10-01 18:08 实机启动，日志原文）

```
PVF dungeons declaring [use fatigue only start dungeon]: … 100003295=6 100004306=30
  100004766=8 100004767=8 100004768=8 100005014=8 100005067=8 100005068=8 …
```

**与文档 §二的实测值逐条一致**（100005014/67/68/766=8、100004306=30、100003295=6）。

> 取证插曲：一开始用 `pvfinspect` 单独导出 `endkeeperoforder.dgn` 时**看不到**该段，一度以为源里没有；
> 实际是**目录全集**里才有（该段分布很广，`DungeonCatalog` 解析全量 3200 个副本时命中）。教训：
> **单文件抽查不能替代全量解析**，判断「源里有没有」要用解析器的视角。

## 2. 奖励表：从 4 张补到 7 张

- 直读路径 `internal/loot/attunement_source.go` 会**扫描整个 `etc/rewardboostinfo`**，按 CTP 里原生的
  `[dungeon index]` 绑定副本 —— 因此**不需要** JSON 模式下的 `-extra` 参数；
- 但绑定结果受**内容策略白名单**过滤（`configs/pvf-mine-policy.json` 的 `attunement_dungeons`），
  原先只有 `100005066/100005067/100005068/100005014` 四个；
- 补入 `100004766/100004767/100004768`（`FortifiedDiscipleOfDoom` 的 unique/legendary/epic）后，
  启动日志：

```
PVF attunement prepared: dungeons=[100005066 100005067 100005068 100005014 100004766 100004767 100004768] templates=144 coupon rows=5
loaded attunement rewards (7 dungeons [ … 7 个 … ], 144 reward templates)
```

⇒ **7 张表 / 144 模板 / 5 条征兆行**，与文档 §六 的验收目标一致。

## 3. 柱型（`[special setinfo reward]`）：为什么本轮不做

文档 §四/§五记录过一次「柱型读取 + `PillarForRun`/`PillarForDrops`」的实现。本轮核实：

- `internal/loot/dungeon_special_reward.go` **不存在**；
- 全仓 grep `special setinfo`：只有 `internal/catalog/droptable_test.go`（普通掉落组的解析测试）提到该标签；
- 协议侧只有**月殿堂**的「特殊奖励剩余次数」（`protocol.DungeonSpecialRewardInfo115`），与「柱型」不是一回事；
- ⇒ **当前没有任何消费点**（既不改存档、也不改协议、也不参与结算）。

按 AGENTS §0.2/§0.3（真源优先、禁止猜包）：**语义未确立时不实现**。要恢复它，需要先确定
「柱型交给谁用」（协议字段？掉落展示？保底归属？）—— 这是**下一步的取证任务**，不是代码问题。

## 4. 遗留

| 项 | 现状 |
| --- | --- |
| `hidden` 的 index→箱子触发条件 | 文档 §七 已列为未确立；本轮未动 |
| `[special setinfo reward]` 的 `prefix`（320/138/427-430）语义 | 未确立；`dungeon_special_reward.go` 缺失时无从谈起 |
| `100004306`（`DiscipleOfDoom`，非 Fortified）的奖励机制 | 其 5 条 `[special setinfo reward]` 不在 `rewardboostinfo.lst` 里；本轮**进货疲劳已覆盖**（30 点），奖励机制仍未确立 |
| 部分奖励模板不在 item catalog | 例如 `10401416/10401429`（启动日志会告警「name N box(es) this build cannot open」）—— pre-existing |
| 实机验收 | 进货疲劳 8 点（成长契约 −1 = 每本 +7）、第 23 次 clamp、第 24 次**在选图阶段**被拒 —— **待用户手动验证** |

## 5. 相关

- 业主取证原文：《千海天深渊 · 核心玩法与掉落（实证定义）》（2026-10-01，含 §一~§九）
- `next141-内层PVF自动生成与哈希门禁-设计.md`、`next142-PVF校验锁自动派生-根治.md`
- `next145-直读模式重编后发布PVF默认程序-根治.md`、`next146-存档身份与内层哈希解耦-结构性根治.md`
- 代码：`internal/catalog/dungeons.go`、`internal/character/fatigue.go`、`internal/storage/fatigue.go`、
  `internal/loot/attunement_source.go`、`cmd/wireprobe/{main,pvf_special,pvf_scenes,dungeon_flow,card_flow}.go`
