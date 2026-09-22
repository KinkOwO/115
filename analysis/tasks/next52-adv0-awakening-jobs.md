# next52 — 无转职分支职业的觉醒缺口：`demonicswordman` / `creator mage` 的 36 个觉醒技能不可学

> 日期：2026-09-22 · 状态：**取证完成；§5 的第 1、2 层已落地并验证，第 3 层（协议）待实机证据**
> 触发来源：用户提供的第三方补丁包 `50级以上技能修复以及源码`（`learning_catalog.go` + README）
> 相关：`internal/character/learning_catalog.go`（本次已落地的双价 `[purchase cost]` 修复）

## 0. 一句话结论

**`creator mage`(job 10) 与 `demonic swordman`(job 9) 没有转职分支**（`.chr` 里 `[max grow count] 1`，
只有 `[growtype 1]` 一段），它们的角色 `Advancement` 恒为 **0**，而**觉醒（stage 1/2/3）就挂在
growtype 0 上**（觉醒段写在 `[growtype 1]` 内，技能觉醒矩阵的非零值也落在 growtype 列 0）。
服务端有 4 处硬编码假设"觉醒只发生在 adv>=1"，导致这两个职业**完全无法觉醒**，
其 **36 个觉醒技能**（demonic swordman 14 + creator mage 22）永远不可学。

第三方补丁对同一现象的处理是**把 `[growtype maximum level] == 0` 解释成"不限制"**，
能顺带放开这 36 个技能，但代价是全局语义反转（详见 §4），本仓库**不采用**该路线。

## 1. 真源证据（客户端 PVF，非推测）

源自 `server/work/client-build/Script.inner.pvf`（已解包内层 PVF），用
`go run ./cmd/pvfinspect -source ..\client-build\Script.inner.pvf -file <entry>` 读出原文。

### 1.1 三个 `.chr` 的 growtype 结构对比

| 文件 | growtype 段 | `[max grow count]` | 觉醒段位置 | 转职分支（配置层 `advancement_growth`） |
| --- | --- | --- | --- | --- |
| `character/swordman/swordman.chr` | `[growtype 1]`…`[growtype 6]` | `5` | 在各 `[growtype 2..6]` 内 | `[1,2,3,4,5]` |
| `character/swordman/dsswordman.chr` | **仅 `[growtype 1]`** | `1` | **在 `[growtype 1]` 内** | **无** |
| `character/mage/creatormage.chr` | **仅 `[growtype 1]`** | `1` | **在 `[growtype 1]` 内** | **无** |

即：普通职业的"第 N 个 growtype 段承载该 growtype 的觉醒"，而无转职分支职业的
**唯一 growtype（slot 0）自己就承载觉醒 1/2/3**。

### 1.2 觉醒授权确实存在（不是空数据）

`creatormage.chr` 的 `[growtype 1]` 段内：

```
[awakening 1]
[awakening skill]
273 1 268 1
[/awakening skill]

[awakening 2]
[awakening skill]
274 1 261 1 262 1
[/awakening skill]

[awakening 3]
[awakening skill]
407 1
[/awakening skill]
```

`dsswordman.chr` 同构（`[awakening 1..3]` 均在 `[growtype 1]` 内）。另有
`[selectable awakening skill index] 0 284`（creatormage）/ `0 255`（dsswordman），
对照 swordman 的 `[growtype 2]` 段里是 `86 245` —— **该字段是两个技能 id，首个为 0 表示空位，
不能当作 growtype 索引使用**（此点仅作记录，未用于下结论）。

### 1.3 技能侧交叉印证

`skills.next27.json` 里这两个职业的觉醒技能全部是
`[growtype maximum level] = [0 0 0 0 0 0]`（基础 growtype 全 0）+ 觉醒矩阵非零值只落在
**growtype 列 0**。例：

| 技能 | 矩阵（3 stage × 6 growtype） | 非零列 |
| --- | --- | --- |
| `creatormage/iceage` | `40 0 0 0 0 0 / 40 0 0 0 0 0 / 40 0 0 0 0 0` | 0, 6, 12 → stage1/2/3 的 growtype 0 |
| `creatormage/theendoftime`(271 同类) | `0…0 / 0…0 / 40 0 0 0 0 0` | 12 → stage3 的 growtype 0 |
| `demonicswordman/timestop` | `40 0 0 0 0 0 ×3` | 0, 6, 12 |

## 2. 服务端缺口（4 处，全部是"adv>=1"假设）

| # | 位置 | 现状 | 后果 |
| --- | --- | --- | --- |
| 1 | `internal/catalog/awakening.go` `AwakeningSkillGrants` | `grow < 2` 直接 `continue` | **导出配置里这两个职业的 `awakening_skills` 为空**（与 `characters.*.json` 实测一致），`ApplyAwakening` 会报 `source awakening skills missing` |
| 2 | `internal/character/awakening.go` `ForAwakening` | `adv < 1` → `false` | 学习层永不认 adv=0 的觉醒矩阵 |
| 3 | 同上 `State.WireAdvancement` | `Awakening != 0 && Advancement == 0` → 报错 | 拒绝"growtype 0 + stage>0"这一合法组合 |
| 4 | 同上 `Service.ApplyAwakening` | `state.Advancement == 0` → 报错 | 这两个职业走不到觉醒落库 |

> 注：`AdvancementGrowth` 只从 `advancement = 1..15` 收（`internal/catalog/characters.go` 的
> `ProfessionGrowth` 循环），所以"adv=0 无分支"在配置层也是一致的 —— 缺口只在觉醒链路上。

## 3. 影响面（全目录扫描，可信度高）

对 `configs/skills.next27.json` 全量（16 个职业）扫描：

- 带觉醒矩阵的技能：**1024** 个；其中现状可达 **969** 个。
- 矩阵在 **growtype 列 0** 有非零值的技能：`thief` 1 个、**`demonic swordman` 14 个**、**`creator mage` 22 个**。
  - `thief/shakedown` 虽是列 0 非零，但它同时有 `[growtype maximum level] = [0 10 0 0 0 0]`（growtype 1 上限 10），
    属普通技能数据，**放宽后不会变可达**，不受影响。
- 把 `ForAwakening` 的 `adv >= 1` 放宽成 `adv >= 0` 后**新增可达**的：
  **`demonic swordman` 14 个 + `creator mage` 22 个 = 36 个，其余 14 个职业新增 0 个。**

即修复面天然收敛在这两个职业，因为其它职业的觉醒矩阵 growtype 列 0 全为 0。

## 4. 与第三方补丁的关系（为什么本仓库不照搬）

第三方补丁（`learning_catalog.go`）走的是另一条路：**去掉 `ForAdvancement` 里
`cap[adv] <= 0` 的拒绝 + 把 `caps[growIdx] == 0` 当作"不限制"**，等于把
`[growtype maximum level] == 0` 从"该 growtype 学不了"改读成"该 growtype 无上限"。

在本仓库基线上照搬实测 8 个测试失败（`go test ./internal/character/`）：

```
TestSwordmasterLearningUsesSourceRules/base_job, /other_advancement
TestArcherBaseSkillsRespectBothFitnessAndGrowCap        (grow0 zero-cap skill leaked)
TestSkillLearningRespectsLevelAndGrowtype               (unadvanced Swordman learned a growtype 1 skill)
TestKnightPreviewSkillsEligibilityAndLearning           (Elven Knight learned a Chaos skill)
TestAwakenedSkillLearningUsesOwnJobAndStage             (unawakened learned awakening skill)
TestAwakeningOnlySkillsLearnableAfterAwakening
TestAutoSetSourceLearningAndVariation                   (cross-job variation accepted)
```

原因是 `cap = 0` 在本仓库里是"该 growtype 不可学"的**结构性判据**（`[awakening maximum level]`
矩阵由 `forState` 精确写回后，cap 才是唯一权威上限）。放宽它会让跨职业、跨 growtype、
未觉醒角色全部泄漏。因此本仓库保留 `cap = 0` 语义，改为按 §2 精确放开"无转职职业的 adv=0 觉醒"。

## 5. 修复顺序（第 1、2 层已落地，第 3 层待定）

1. **（已落地，纯数据层）** `catalog.AwakeningSkillGrants`：`grow < 2` → `grow < 1`
   （允许 `[growtype 1]` → `adv = 0`）。已核对：普通职业的 `[growtype 1]` 段内**没有**
   `[awakening skill]`（swordman 的首个 `[awakening 1]` 在第 132 行，属 `[growtype 2]`），
   故对现有职业零影响；段落之前出现的 `[awakening skill]`（`grow` 仍为 0）依旧拒绝。
2. **（已落地，学习层）** `ForAwakening`：`adv < 1` → `adv < 0`。影响面＝§3 的 36 个技能，已扫描确认。
3. **（未落地，协议层，需实机证据）** `WireAdvancement` / `ApplyAwakening` 允许 adv=0 + stage>0。
   这两处需要"职业无转职分支"的判定入口（`len(prof.AdvancementGrowth) == 0`），
   且 **wire 字节是否就是 `stage << 4 | 0`、客户端觉醒流程是否照此收发，必须实机验证**
   （按根 `AGENTS.md` 的"禁止猜包"，未闭环前不叠包）。

### 5.1 第 1 层的落地验证（2026-09-22）

用当前源码临时重导配置（输出到临时目录，未覆盖仓库配置）：

```
cd server\work\dfo-lan
go run ./cmd/catalogimport -source ..\client-build\Script.inner.pvf -output <临时文件>
```

结果与 `.chr` 原文**逐值一致**：

| 职业 | `advancement_growth` | `awakening_skills` |
| --- | --- | --- |
| job 0 `swordman` | `1,2,3,4,5` | `1..5`（**无 `0` 键**，零影响） |
| job 9 `demonic swordman` | 无 | `0: {1:[263,1], 2:[256,1,255,1], 3:[269,1]}` |
| job 10 `creator mage` | 无 | `0: {1:[273,1,268,1], 2:[274,1,261,1,262,1], 3:[407,1]}` |

测试：`internal/catalog/awakening_test.go` 的 `TestAwakeningGrantKeepsUnadvancedGrowtype`（`[growtype 1]` 授权保留 +
段前授权仍拒）；`internal/character/learning_adv0_awakening_test.go`（growtype 0 列可学、
stage 0 与负 advancement 仍拒、影响面收敛在 job 6/9/10）。

### 5.2 尚未生效的部分（重要）

- **仓库内 `configs/characters.*.json` 尚未重新导出**：运行时的 `awakening_skills` 表仍是旧的空值，
  所以第 1 层要等下次按各自参数重新生成这些配置后才在服务端生效。
- **实机行为不变**：第 3 层未放开前，角色状态不可能出现 `adv=0 + stage>0`，
  因此这两个职业仍无法觉醒，36 个技能仍不可学。第 1、2 层是第 3 层的前置。

## 6. 待验证 / 未闭环

- 客户端在这两个职业觉醒时实际发出的 `CMD 1881 change grow-type` / 觉醒包的 growtype 字段值。
- `[selectable awakening skill index]` 首值 0 的确切语义（本报告未依赖它下结论）。
- 这两个职业在服务端创建/进入流程是否已完整（`alljobs-pilot` 配置含 job 9/10，
  `TestAllSourceAdvancementsEntryAndGrowth` 只覆盖 `AdvancementGrowth` 里存在的分支，故未覆盖 adv=0 觉醒）。
