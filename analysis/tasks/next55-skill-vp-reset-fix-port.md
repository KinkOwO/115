# next55 · 技能系统修复包（第三方接续版）对照与消化

> 日期：2026-09-22 · 状态：**已落地可采纳部分（待实机验证）**
> 来源包：`E:\迅雷下载\技能系统修复_20260922.zip`（含 `README.md` + 8 个 `.go`）
> 相关：`next54-slot-expansion-fix-port.md`（同批外部说明的对照方法）
> 分支：`fix/skill-vp-reset-absorb`（本地，未推送；开槽修复在 `fix/equip-slot-unlock-chain`）

---

## 0. 一句话结论

对方的 **VP 保存链 + CMD483 Reset 链**是真缺口，值得消化；但整包**不能直接覆盖**——
它的基线停在我们的 `29c8046`（`fix/skill-zero-growtype-row` 那一支），比当前 `main`
少了 **2062 直达进图**、**网关端口撞车重试**、以及**觉醒 growtype 0 的修复**，
直接覆盖会把这些一并回退掉。

---

## 1. 对方基线判定

`main.go` 里同时出现三处"我们没有差别、他们有"的**旧代码**，可反推出对方基线：

| 差异 | 我们（main） | 对方包 | 归属 |
| --- | --- | --- | --- |
| 游戏端口监听 | `openGameListeners(*gameListen, channelCount)` | `net.Listen("tcp4", ...)` 裸调用 | 我们的 `1ddb6ad` 端口撞车重试 |
| CMD 2062 | `case 2062: directMoveDungeon` | **无此 case** | 我们的 `dfa47bb` 清关直达 |
| `ForAwakening` | `adv < 0`（带 growtype 0 说明注释） | `adv < 1` + 注释被删 | 我们的 `adv0` 修复（分支 `6be6375`） |
| 宠物跟随段 | `service.go` 未投影 `CreatureItemID` | 已投影（`wornCreature`） | 分支 `00b45bb`，**不在 main** |
| 教程标记 | 无 `TutorialFlag = 30` | 有（标注为 Experiment） | 分支 `a17e73f`，**不在 main** |

对照 `git merge-base`：对方基线与 `fix/skill-zero-growtype-row` 同源（`29c8046`），
且含有该分支独有而 `main` 没有的宠物/教程/继承设置提交。**结论：他们是在我们那条
MR 分支上继续做的**，所以"差异"里混着大量**基线漂移**而非他们的新改动。

---

## 2. 逐文件对照

| 文件 | 他们的改动 | 判定 |
| --- | --- | --- |
| `internal/character/skill_variation.go` | VP 解锁只看三觉、补满槽位、缺 `[variation point]` 不拒、`VariationRestore` 补块、新增 `ResetAutoSet` | ✅ **采纳**（退款口径已修正，见 §3.2） |
| `internal/character/service.go` | `SkillVariations` 去掉 `omitempty` | ✅ **采纳**（是 Reset 能落库的前提，见 §3.3） |
| `cmd/wireprobe/skill_flow.go` | 新增 `case 483`；`id==29` 后不再无条件补 id19 | ✅ **采纳**（id19 抑制收窄为"仅带 VP 的请求"，见 §3.4） |
| `cmd/wireprobe/main.go` | 路由加 `frame.ID == 483` | ✅ **采纳** |
| `cmd/wireprobe/awakening_flow.go` | README 声称"2177 后追加 id29" | ⚠️ **包里文件与我们逐字节相同**，该条未实现（见 §4.2） |
| `internal/character/awakening.go` | 把 `adv < 0` 改回 `adv < 1`、删注释 | ❌ **拒绝**（回退我们的 growtype 0 修复） |
| `cmd/wireprobe/main.go` | 端口监听、CMD2062、教程/设置恢复 | ❌ **拒绝**（基线漂移；前两项是回退） |
| `internal/catalog/characters.go` | `SlotSkills`→`PresetSkills`、去掉 `defaultShortcutEligible`、`nextSlot` 从 0、**删掉 `InitialSkillSlots` 的填充** | ❌ **拒绝**（见 §4.1） |
| `internal/character/learning.go` | 有命令的技能优先占 0..13；`placeNewShortcuts` 处理 65535 | ❌ **暂缓**（见 §4.1） |

---

## 3. 已落地的实现

### 3.1 VP 解锁与补块

- `applyVariations`：解锁条件从 `Level < 115 || Awakening != 3` 收窄为 `Awakening != 3`。
  **这是真 bug**：`ApplyAwakening` 的三觉门槛是 **100 级**（`[4]byte{0,50,75,100}`），
  100~114 级的三觉角色原来会被 `variation unlock progression not satisfied` 拒掉。
- `fillVariationSlots`：补齐 3 个 Enhance + 5 个 Evolve 固定宽度槽。
- `[variation point]` 块**完全缺失**→ 放行；**存在但缺该 tag**→ 仍然拒。
  当前源里确实存在这类技能（测试选中 `job10 skill185 skill/creatormage/armormasteryleather.skl`）。
- `VariationRestore`：三觉补满块后下发；未三觉返回 `nil`（客户端没有 VP 面板，不该收到空块）。

### 3.2 `ResetAutoSet`（CMD483）

落在 `internal/character/skill_variation.go`，拆成 `ResetAutoSet`（事务壳）
+ `resetAutoState`（纯状态变换，可脱库单测）+ `skillFloor`（源授予下限）。

**与对方实现的关键差异 —— 退款口径**：对方只把"觉醒授予"当下限，于是
**初始技能与 `[growtype N]` 自动授予的等级也会被退成 SP**（凭空发点）。
我们改为与 `Learn` 完全一致的 floor：

```
floor = initialSkills(存档) ∪ automaticSkills([growtype N] 自动授予) ∪ 觉醒授予(stage ≤ Awakening)
```

并要求 `known[id] >= rank`（与 `Learn` 同一条门槛），只退 `floor+1 .. rank` 这一段。
测试钉住：`skill46 floor=1 target=3 refund=30（全额 45）`。

其余语义：`LearnedSkills` 只保留"原本就在、且 floor > 0"的技能（不做无条件补授予，
避免把等级门尚未满足的觉醒技提前塞进去）；`SkillSlots` 清空（不清会被 `skillRows`
优先读取，新推荐技能被挤到 14+）；`mask&2` 清 Enhance、`mask&4` 清 Evolve、补满宽度。

### 3.3 去掉 `omitempty` 是 Reset 能生效的前提

`mergeSkillState` 是按字段合并：`skill_variations` 带 `omitempty` 时，清空后的
`[{},{}]` 会被**整个键省略**，旧值原封不动合并回去，Reset 的 VP 清除**根本不落库**。
这条必须一起改，否则"Reset 后 VP 还在"。

### 3.4 id19 抑制收窄（与对方不同）

对方的做法是 `if id == 29 || !applied` → `if !applied`，即**所有**生效的 CMD29
都不再补 id19。但那会把**普通加点**（已经实机验收通过）的补包也一起去掉。
我们收窄为只对"**带 variation 槽的 CMD29**"抑制，落成可测的纯函数：

```go
func skillTreeRefreshRequired(id uint16, applied, varied bool) bool
```

被拒/幂等请求仍然补 id19（把客户端拉回存档状态），CMD28 移动槽位不受影响。

### 3.5 附带

- `cmd/wireprobe/request_scope.go`：把 483 加进 `observedGameRequest`（已实现命令
  应通过该证据门禁，否则日志仍标 `unimplemented_sample`）。

---

## 4. 未采纳与原因

### 4.1 §6「推荐技能槽位」两条：**暂缓**（对方自己也标"搁置"）

对方 README §6 承认最终仍与客户端 Auto Set 不一致。我们另外查了真源：

**`[preset skill]` 与 `[skill]` 的真实形状**（`docs/evidence/skycastle-auto-skills-20260917/details/00-atswordman.chr.tokens.json`）：

```
[growtype 1]
  [preset skill]  14 21 99 15 27 9 12 5 54 18 13 26 2 8 52 153 33 57 17 16 96 …   ← 一维 id 列表，有序
  [skill]         16 1 1 | 55 1 1 | 56 1 1 | 57 1 1 …                              ← 三元组 (id, level, enable)
[顶层]
  [skill]         179 7 1 | 96 1 1 | 174 1 1 | 169 1 1 | 8 1 1 | 190 1 1 | 511 1 1 | 452 1 1
```

对方**关于格式的判断是对的**（`[preset skill]` 确实是一维有序列表，`[skill]` 是三元组）。
但他们的落地方案做了两件有代价的事：

1. **删掉顶层 `[skill]` 对 `InitialSkillSlots` 的填充** —— 现行配置
   `configs/characters.generated.json` 里 `initial_skill_slots` 是**有值**的
   （例：job0 = `{169:0, 452:1}`），正是这段代码生成并落库的。删掉后下次**重生成目录**
   会让初始技能的默认快捷栏槽全部消失，与 `58f89e7`（初始技能带 command 与槽位）冲突。
2. `nextSlot` 从 0 起 + 去掉 `defaultShortcutEligible`，是**用一份猜出来的顺序替换另一份**，
   而 §6 自认结果仍与客户端不一致 —— 换不到正确性，只换来回归面。

**我们保留现状**，但记下一条明确的后续方向（见 §6）：`[preset skill]` 目前**完全没被使用**，
它才是"每个分支的推荐摆放顺序"的源声明；要做就应该"**新增**用它派生的槽位映射"，
而不是删掉 `InitialSkillSlots` 再顶上去，并且必须以实机 Auto Set 比对验收。

### 4.2 README 与包不一致的一条

README §1 称 `cmd/wireprobe/awakening_flow.go`"三觉完成 id2177 后追加 id29"，
但包内该文件与我们**逐字节相同**（`diff` 为 0），且其中没有 id29。
即：**三觉当次不会立刻推 VP**，要等进城/换角色走 `appearanceRestore` 或
`entryPayloads`（`plan.SkillVariations`）才下发。今晚实机时请特别确认这一条：
三觉后**当场**打开 VP 面板是否有内容；若无、而换角色后正常，就说明还缺这一包，
我们补一行即可（改动很小，但**未经验证不先加**）。

### 4.3 明确拒绝的回退

`awakening.go`（`adv < 1`）、`main.go`（端口监听、CMD2062）三项见 §1，属基线漂移，
不采纳。

---

## 5. 验证

- 新增 `internal/character/skill_reset_test.go`（7 个用例）与
  `cmd/wireprobe/skill_flow_test.go`（1 个用例）：
  - 源授予段不退款（`62:1` → SP 不变、技能保留、槽位清空）
  - 只退 `floor+1..rank`（`skill46 floor=1 target=3 refund=30`，全额 45）
  - Reset 幂等（连做两次 state 逐字节相同，含 VP 块）
  - `mask&2/&4` 清 Enhance/Evolve 并补齐 3+5 宽度；`mask=1` 不动 VP
  - 解锁跟随三觉而非 115 级（`3/100 → 放行`，`2/115 → 拒`）
  - 缺 `[variation point]` 的技能放行（选中 `job10 skill185`）
  - `VariationRestore` 三觉补满块且字节与期望一致；二觉返回 nil
  - `skillTreeRefreshRequired`：VP Apply 不补 id19、普通加点补 id19、被拒补 id19
- `go test -count=1 ./internal/... ./cmd/...` 全绿；`go vet ./internal/... ./cmd/...` 零输出。
- 实证方式：对方 `learning.go`/`characters.go` **换上后跑过全量测试（也是全绿）**，
  说明现有测试**钉不住**槽位语义 —— 所以 §4.1 的结论来自真源取证（chr tokens），
  不是来自"测试挂了"。跑完已还原，`git diff` 为 0。

## 6. 后续（未做，留痕）

1. **实机验证清单**见 §7；三觉当场 VP（§4.2）是第一优先确认项。
2. **`[preset skill]` 派生槽位**：作为独立任务，新增映射 + 实机比对，不动 `InitialSkillSlots`。
3. **CMD483 请求体格式**：现在按"body 不可按 (tree,mask) 解码"硬编码 `tree=0, mask=7`。
   对方的证据与我们一致（解出来是 111/40）。若要支持"只清 Enhance/只清 Evolve"的
   单选按钮，需要一份真实 body 的取证；483 已加入证据门禁，日志里会留 `plain_hex`。
4. **三觉授予的等级门**：`ApplyAwakening` 对 `[required level]` 未达的技能直接跳过，
   之后再升级也不会补授予（`knownSkills` 不查 `AwakeningSkills`）。属既有缺口，
   与本次无关，但值得单开一条。

## 7. 实机验证清单（今晚）

- [ ] 三觉角色（100~114 亦可）打开 VP → 选择 Enhance/Evolve → **Apply 后不重置**，
      重登仍在（这条是本包的主症状）。
- [ ] 三觉完成 id2177 **当场**打开 VP 面板是否有内容（§4.2）。
- [ ] Reset 按钮：技能清空、**SP 返还**、快捷栏清空后由客户端重新摆放。
- [ ] Reset 后 VP 的 Enhance/Evolve 也清空，且重登仍为空。
- [ ] 回归：普通加点（非 VP）仍即时刷新技能树；CMD28 拖槽仍生效。
- [ ] 若 Reset 后客户端**卡住等确认**，说明 483 需要应答包 —— 这是一条待补的假设。
