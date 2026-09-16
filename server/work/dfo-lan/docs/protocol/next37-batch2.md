# 37 轮第二批（任务链 / 金币 / 装备诊断）

紧接 next37-status.md。二进制 `f47f5e08…`。

## 1. 掉落按角色过滤——已撤销

上一批加的"只掉本角色能穿的装备"是误解用户需求。全职业掉落是游戏特色，已删除
（`drop_pool_flow.go` 删除，`session.go` 的 Pool 字段移除，`Roll` 恢复用
`Equipment.DropPool()`）。保留了 `WearableBy` 重构（`wear.go` 的穿戴校验下沉到一处，
行为不变）。

## 2. "装备穿不上"根因：客户端本地拒绝

实机日志无任何入站 CMD19——客户端**本地**弹"This equipment can't be equipped"，服务端
根本没被问到。逐件查背包：

- `100261068`（鞋，21650 引导任务奖励）：源数据带 `[impossible contents]`，是**收藏品**，
  本就不可穿。`[usable job] [all]`、level 1 看着能穿，但被这个 flag 拦下
- `404030090`：gunner 武器，弓箭手不可穿（正确）
- `27603`：剑士系武器（正确）
- 已穿上的 20002/24002/22002：饰品，`[free]`，无 impossible-contents

已发一整套已知可穿的弓箭手基础装（bow 117000003 + 全身 [free]/[all|archer]/≤9级，
grant-id `2026-09-12-archer-starter-set-01`，装备栏 10~20）供实机验证穿戴本身没坏。

## 3. 任务链根因：复合 [type] 取错字段

`internal/catalog/quests.go` 把 `d.Kind` 设为 `[type]` 段的**最后**一个 type-6 词。
复合任务把主目标放**第一个**，后面追加子条件（`arrive in town`、`accept`）。于是
**34 个复合任务**（含主线 22 级的 21029 =「[look cinematic] + arrive in town」）被误判为
未实现的 "arrive in town"，主目标丢失。

各复合组的 `[int data]` 实测都吻合**第一个**词的解码器：

| 复合 | int-data | 归属 |
|---|---|---|
| [meet npc] + arrive in town (12) | 1 cell | SingleMeetNPC |
| [reach the range] + arrive in town (11) | 6 cells | ReachRange |
| [look cinematic] + arrive in town (9) | 1 cell | LookCinematic |
| [meet npc] + accept (3) | 1 cell | SingleMeetNPC |

改为取**第一个** type-6 词（`ImportQuests` 加 break，`LoadQuests` 对既有目录重新推导）。
4 个子形状不同的边缘任务（3-cell reach-range、7-cell meet-npc）仍正确保持未实现。

## 4. [look cinematic] 目标

过场动画无服务端可验证条件（客户端本地播放后才让提交）。`InitialProgress` 对
`[look cinematic]`（单 cell、值>0）返回 progress 0（客户端门控提交），只结算任务自身
配置的奖励，不凭空造。

## 5. 主线实测（改后）

从 3148 沿 [pre required quest] 前推 200 个任务：**8→~50 级 0 卡点**（改前卡在 22）。
51 级起遇到 `[seeking]`（收集）/`[hunt monster]`/`[hunt enemy]`/`[condition under clear]`
共 11 处未实现——下一批的目标。角色现 9 级，早期主线已全通。

低级未实现目标类型按解锁量排（LAST-cell 语义，即服务端真实所见）：
[clear a dungeon] 183、[seeking] 155、[condition under clear] 120、[hunt monster] 72、
[clear quest] 71（单 cell=前置任务 id，最易做）。

## 6. 任务金币

完成金币来自 `[gold reward table]`（每级一个值，200 项），非任务自身 cell。之前只实现
经验表。新增 `progression.QuestGold`：复用已验证的难度权重+等级惩罚，作用于金币表。
`Finish` 事务内以 award id 0（钱包）入账并记入收据。实测 **2290 个任务发金币**，553 个
无难度的不发（正确）。

**未完**：游戏内实时金币数字需要任务奖励包的原生布局（`GoldPickupConfirmed` 只适用于
副本内场景金币，town 交任务不适用）。当前金币 durable 落库，重选角色可见。

## 7. 待续

- 血量 0.0143 倍率的根因（唯一未定死的）
- 51 级+ 的 [seeking]/[hunt monster]/[hunt enemy]/[condition under clear]
- [clear quest]（59 个低级支线，单 cell 前置，最易）
- 任务金币/翻牌的实时显示包
- 装备分解 CMD26、商城 CMD1302、邮件 CMD63/2036
