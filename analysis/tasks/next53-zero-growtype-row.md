# next53 — `[growtype maximum level]` 的两种零：整行 0 vs 部分 0（弹药等 138 个技能不可加点）

> 日期：2026-09-22 · 状态：**已落地，待实机验证**
> 触发来源：玩家反馈"弹药职业觉醒后无法学习觉醒技能，换第三方补丁包就可以"
> 相关：`analysis/tasks/next52-adv0-awakening-jobs.md`（同属技能可学性，但根因不同）

## 0. 一句话结论

源里 `[growtype maximum level]` 的零有**两种**含义，此前被当成一种：

| 形态 | 例子 | 含义 |
| --- | --- | --- |
| **整行全 0** | `atgunner/quartermaster` = `0 0 0 0 0 0` | 该技能**不使用**基础 growtype 上限表；归属由 `[skill fitness growtype]` / `[skill fitness second growtype]` 决定，等级上限取 `[maximum level]` |
| **部分为 0** | `archer/latentability` = `0 1 1 1 1 1` | 那个 growtype **真的学不了** |

旧逻辑把两者都读成"上限 0 → 不可学"，于是**整行 0** 的技能一律无法加点（弹药 `quartermaster`、
`g96thermobaricgranade`、`pistolcarbine`、`extruder`、`lockonsupport` … 共 138 个，覆盖全部 17 个职业）。
第三方补丁把两者都读成"不限"，所以"补丁可以、我们不行"成立 —— 但它同时放开了**部分 0** 的行，
这正是补丁造成 8 个既有测试失败（未觉醒学觉醒技能、跨 growtype、未转职泄漏）的原因。

## 1. 真源证据（`.skl` 原文）

用 `pvfinspect` 从 `server/work/client-build/Script.inner.pvf` 读取：

```
# skill/atgunner/quartermaster.skl
[required level] 75         [maximum level] 20
[growtype maximum level] 0 0 0 0 0 0
[awakening maximum level] 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0
[skill fitness growtype]                ← 空
[skill fitness second growtype] 2       ← 归属声明在这里
[purchase cost] 80

# skill/atgunner/g96thermobaricgranade.skl
[pre required skill] 56 1   [required level] 80   [maximum level] 50
[growtype maximum level] 0 0 0 0 0 0
[awakening maximum level] 0 ×18
[skill fitness growtype]    ← 空（无 second growtype）
[purchase cost] 90
```

关键：`[awakening maximum level]` 存在但**整行 0**，所以它不是"觉醒矩阵驱动"的技能（与 next52 的
job 9/10 不同 —— 那些矩阵**有值**、只是落在 growtype 0 列）。

对比"部分 0"的样本 `skill/archer/latentability.skl`：`[0 1 1 1 1 1]` + `[skill fitness growtype] 0 1 2 3 4 5`
—— 源明确为每个 growtype 给了上限，0 就是"未转职不可学"。

## 2. 量化（全目录 3224 个技能）

| 量 | 值 |
| --- | --- |
| 第三方补丁比本仓库多放开的技能 | **138**，覆盖 **17/17 个职业**（每个 3~12 个，非专为弹药） |
| 其中 caps 为"部分 0"却被补丁放开的 | 0（补丁的额外泄漏在**状态级**：未觉醒/未转职/跨分支） |
| 本方案落地后"补丁能学、我们不能学" | **0** |
| 本仓库比补丁多放开（觉醒矩阵精确能力） | 29（保留） |
| 源里无任何归属声明的技能（本方案唯一变宽点） | 19 |

19 个无声明技能里，绝大多数是各职业通用 `conversion`（`cost=0`、`lvl=1/20`、`max=1`）与
`priest/hpmaxuppersonal`；其余（`mage/summonspirit*ex`、`thief/moonshine`、`mage/disactivestatus`）
`[maximum level] = 0`，本就不可能有 rank。故实际变宽面很小。

## 3. 实现

`internal/character/learning_catalog.go`：

- 新增 `allZeroCaps([]int) bool`：区分"整行 0"与"部分 0"。
- 新增 `fitnessAllows(adv)`：归属判定 —— `[skill fitness growtype]` → `[skill fitness second growtype]`
  → 两者都空则不设 growtype 门禁。
- `ForAdvancement`：
  1. 行非全 0 → 沿用 `cap[adv] > 0`（部分 0 的零仍然拒）；
  2. 行全 0 且带**有效**觉醒矩阵 → 一律 false（觉醒专属，只能由 `forState` 写回后通过）；
  3. 其余全 0/缺失 → `fitnessAllows(adv)`。
- `Cost` 的 `limit`：整行 0 时不压 `[maximum level]`。

`atgunner/quartermaster`（`sec=[2]`）落地后**只在 adv=2 可学**，`atgunner/pistolcarbine`（`sec=[1]`）
只在 adv=1，`archer/latentability` / `knight prologue 61` / `lightswordmastery` / `crazymount` 的零列
仍被拒 —— 精度与泄漏面都由既有测试钉住。

## 4. 测试

新增 `internal/character/learning_zero_cap_row_test.go`：钉住两种零的分界、整行 0 的
`[maximum level]` 上限、跨分支拒绝、无声明技能仍受等级与前置约束、觉醒专属行不被自身放行。

`go test -count=1 ./internal/... ./cmd/...` 与 `go vet ./internal/... ./cmd/...` 全绿。

## 5. 待实机验证（用户操作）

1. 弹药（女枪手 / `atgunner`）觉醒后能加点 `quartermaster`、`g96thermobaricgranade`、`pistolcarbine`。
2. 同一角色的**其它分支**技能树里不出现这些技能（确认无跨分支显示）。
3. 未转职/未觉醒状态下不能加点这些技能。
