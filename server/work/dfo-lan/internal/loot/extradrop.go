package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/extradrop"
	"strings"
)

// ExtraDropRewards 是活动 1093「额外掉落奖励」的掉落侧：
// **在声明了 `[dungeon type] = ENDKEEPER OF ORDER` 的副本里，按每件掉落装备的品质额外给
// `Mystical Energy`**。
//
// ## 口径来源（都是已证事实）
//
//   - 副本判据：脚本 `[target dungeon type] ENDKEEPER OF ORDER`
//     ↔ `catalog.DungeonType(d)`（读副本脚本自己的 `[dungeon type]`），大小写不敏感。
//   - 数量：`extradrop.Catalog.EnergyFor(组, 稀有度)` —— 稀有度取**掉落装备自己的**品质
//     （客户端面板原文："equipment that **drop** … are giving off Mystical Energy"）。
//   - 装备判据：`LootItem.Kind != "stackable"`（与 `PreparePickup` 判装备同一条）。
//
// ⚠️ **封顶只在本函数内生效**（`MaxAmount`），**不是**源要的"按天/按期" ——
// 那需要角色级持久化（CN 版是按天的 800/200）。本轮先打通获取，封顶口径待续（见 next200 §13）。
type ExtraDropRewards struct {
	Catalog *extradrop.Catalog
	// FixedPerRun > 0 时改成「**每局固定给一次**」模式（业主 2026-10-13 定案）：
	// 不打稀有度表，每个**局**（Session 生命周期）只给 `FixedPerRun` 个能量。
	//
	// ★ 触发点 = **深渊（调律/天平）的结算那一步** —— `Session.Death` 里
	// `attunementPending` 为真的那一帧（源领主死亡、调律/征兆/天平一起结算那处）。
	// 业主原话：「完全可以做成固定掉落，比如小深渊每次固定掉落 10 个，方便玩家刷取」；
	// 随后追加：「不能绑定在深渊的天平结算上吗，相当于在目前的结算外额外进行的独立处理」。
	// = 0 时回到稀有度表（官方口径：Rare1/Unique2/Legendary3/Epic4/Primeval5）。
	//
	// 缺省值**不写死**在装配处，而是走 `cmd/wireprobe` 的 `extraDropFixedPerRun()`
	// 读 `DFO_EXTRADROP_FIXED`（未设 ⇒ 10；`off`/`0` ⇒ 关掉固定模式；`N` ⇒ 每局 N 个）。
	//
	// 语义说明：1093 的目标副本只有深渊那一条（`[target dungeon type]` = ENDKEEPER OF ORDER），
	// 所以这个开关**只影响深渊**，别的副本本来就不在目标范围内。
	FixedPerRun uint32
	// group 决定查哪张表：自刷走 `drop item`，组队走 `party drop item`。
	// ⚠️ 两组的**分工边界**还没有正面证据（面板只支持 drop 这条），先按"是否组队"分流。
	PartyGroupEnabled bool
}

// Enabled 报告这一份能不能用（未装配 / 源读失败都是 false，整段跳过）。
func (e *ExtraDropRewards) Enabled() bool {
	return e != nil && e.Catalog != nil
}

// Matches 报告这个副本是不是脚本声明的目标副本。
func (e *ExtraDropRewards) Matches(d catalog.DungeonDefinition) bool {
	if !e.Enabled() {
		return false
	}
	// 优先按**源里的副本 id**（`etc/rewardboostinfo/**` 的 `[dungeon index]`）⇒ 决定性、不依赖
	// 副本脚本是否声明 `[dungeon type]`。找不到 id 时才退回类型字符串比较。
	for _, id := range e.Catalog.TargetDungeonIDs {
		if id == d.ID {
			return true
		}
	}
	if len(e.Catalog.TargetDungeonIDs) > 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(catalog.DungeonType(d)),
		strings.TrimSpace(e.Catalog.TargetDungeonType))
}

// AwardsFor 给这一批掉落算出**附加**的能量奖励（不改动入参）。
//
// party=true 时走组队组（其 `[rule] check only higher rarity` 的口径见
// `extradrop.EnergyForParty` 的注释 —— 组队表本身只从 legendary 起，低稀有度天然查不到）。
//
// 返回的 award 的 `Template` 是能量物品、`Amount` 是应给数量；`awarded` 是本函数给出的总量
// （调用方可用于记日志 / 与封顶对账）。
func (e *ExtraDropRewards) AwardsFor(items map[uint32]catalog.LootItem, equipRarity map[uint32]int32, tierByItem map[uint32]string, drops []Award, party bool) (out []Award, awarded uint32) {
	if !e.Enabled() {
		return nil, 0
	}
	// ★ 缺省走 `get item` 组 —— 官方帮助原文把它标成 "upon **Equipment Drop**"
	//（rare1 / unique2 / legendary3 / epic4 / primeval5），正是「掉了一件装备」这个场景；
	// 而 `drop item` 组对应 "upon **Observing Rarity Beam**"（unique5/legendary10/
	// epic15/primeval20）—— 那条要等「是否出现稀有度光柱」的判据落地再接。
	// ⚠️ 2026-10-12 前用的是 `drop item`，**那张表里没有 rare** ⇒ 低品质装备一件都不给。
	group := extradrop.GroupGetItem
	if party && e.PartyGroupEnabled {
		group = extradrop.GroupPartyDropItem
	}
	for _, d := range drops {
		if d.Template == 0 || d.Amount == 0 {
			continue
		}
		// ★ 稀有度统一按**名字**解析，三步兜底（顺序有讲究，见各自注释）：
		name := ""
		// ① 装备目录（数字）—— 普通副本的装备都在这条路上。
		if r, ok := equipRarity[d.Template]; ok {
			if n, ok2 := catalog.EquipmentAwakeningRarityName(int(r)); ok2 {
				name = n
			}
		}
		// ② **调律（深渊）表的档位** —— 那批装备（10416xxx）不在装备目录里，
		//    但源自己带 tier（rare/unique/legendary/epic/primeval），与能量表同源。
		if name == "" {
			if t, ok := tierByItem[d.Template]; ok {
				name = strings.ToLower(strings.TrimSpace(t))
			}
		}
		// ③ 掉落目录（只在**确实登记为非堆叠**时才算装备；它里面基本都是 stackable）。
		if name == "" {
			if item, ok := items[d.Template]; ok && item.Kind != "stackable" {
				if n, ok2 := catalog.EquipmentAwakeningRarityName(int(item.Rarity)); ok2 {
					name = n
				}
			}
		}
		if name == "" {
			continue
		}
		tier, ok := e.Catalog.EnergyForTier(group, name)
		if !ok || tier.Amount == 0 {
			continue // 该档位不在表里 ⇒ 不给（表只从 rare 起）
		}
		if e.Catalog.MaxAmount > 0 && awarded+tier.Amount > e.Catalog.MaxAmount {
			continue // 本轮封顶（见类型注释：不是源要的"按天"）
		}
		awarded += tier.Amount
		out = append(out, Award{Template: tier.Item, Amount: tier.Amount})
	}
	return out, awarded
}
