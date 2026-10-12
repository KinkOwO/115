package extradrop

import (
	"fmt"
	"strings"

	"dfolan/internal/catalog"
)

// 组名常量：脚本 `[group] [type]` 的三种取值（本机实测）。
//
// 它们的角色靠**面板原文**定的（见 next200 §11）：
//
//	客户端面板说 "equipment that **drop** in Endkeeper of Order are giving off Mystical Energy"
//	⇒ 三个组都是「**掉落的那件装备**按它自己的稀有度给能量」，只是触发场景不同：
//
//	GroupDropItem      自刷掉落
//	GroupPartyDropItem 组队掉落（多一条 [rule] check only higher rarity）
//	GroupGetItem       "获得"（拾取/结算一类）
//
// ⚠️ 三个组的**分工边界**还没有正面证据（只能从 `[max amount]` 限定在 drop/party 两组
// 反推这两组是累计路径）。所以这里**只提供查表**，由调用方决定用哪一组 ——
// 别在包里替调用方"猜"一个。
const (
	GroupGetItem       = "get item"
	GroupDropItem      = "drop item"
	GroupPartyDropItem = "party drop item"
)

// EnergyFor 查「某稀有的装备掉落时应给多少 Mystical Energy」。
//
// rarity 是**装备源 `[rarity]` 的数值**（0..8）—— 与 `catalog.EquipmentAwakeningRarityNames`
// 同一套顺序（`common/uncommon/rare/unique/epic/chronicle/legendary/mythology/primeval`），
// 而 ctp 的 `[reward table]` 第一列正是这套名字 ⇒ 两边**逐名对齐**，不需要自己编映射表
// （自己编一份迟早与别人的定义漂移）。
//
// 表中没有该稀有度（例如本机 `drop item` 组没有 rare）⇒ ok=false，调用方按"不给"处理。
func (c *Catalog) EnergyFor(groupType string, rarity int32) (RewardTier, bool) {
	if c == nil {
		return RewardTier{}, false
	}
	name, ok := catalog.EquipmentAwakeningRarityName(int(rarity))
	if !ok {
		return RewardTier{}, false
	}
	g, ok := c.GroupByType(groupType)
	if !ok {
		return RewardTier{}, false
	}
	for _, t := range g.Tiers {
		if t.Rarity == name {
			return t, true
		}
	}
	return RewardTier{}, false
}

// EnergyForTier 与 `EnergyFor` 同款，但**直接按稀有度名**查表。
//
// 名字口径与 ctp 的 `[reward table]` 第一列完全一致（rare / unique / legendary / epic /
// primeval），所以调律表的档位名可以直接拿来用，不必再转一层数字
// （2026-10-13：深渊那批装备不在装备目录里，只能靠源自己的档位）。
func (c *Catalog) EnergyForTier(groupType, tierName string) (RewardTier, bool) {
	if c == nil || tierName == "" {
		return RewardTier{}, false
	}
	g, ok := c.GroupByType(groupType)
	if !ok {
		return RewardTier{}, false
	}
	for _, t := range g.Tiers {
		if strings.EqualFold(t.Rarity, tierName) {
			return t, true
		}
	}
	return RewardTier{}, false
}

// EnergyForParty 与 `EnergyFor` 同款，但走组队的 `[rule] check only higher rarity`。
//
// 该规则的字面意思是"**只检查更高的稀有度**"——本机的实现口径取：**同样是查表**，
// 但组队的表里没有低档（legendary 起）⇒ 低稀有度天然查不到、也就是"不算"。
// 若将来证明该 rule 另有含义（例如"只按队伍里最高稀有度算一次"），改这里。
func (c *Catalog) EnergyForParty(rarity int32) (RewardTier, bool) {
	return c.EnergyFor(GroupPartyDropItem, rarity)
}

// Describe 返回一行摘要（日志/工具用）。
func (c *Catalog) Describe() string {
	if c == nil {
		return "<nil>"
	}
	return fmt.Sprintf("extradrop1093 path=%s dungeon=%q groups=%d maxAmount=%d",
		c.Path, c.TargetDungeonType, len(c.Groups), c.MaxAmount)
}
// EnergyItem 返回神秘能量的物品号（从源里第一张有序档位里取，**不写死**）。
//
// 本机三张组表里每档的第二列都是同一个物品号（590015808 = `Mystical Energy`），
// 这里就取"第一张能找到的非零 Item"，源换了季节/版本也自动跟着变。
func (c *Catalog) EnergyItem() uint32 {
	if c == nil {
		return 0
	}
	for gi := range c.Groups {
		for _, t := range c.Groups[gi].Tiers {
			if t.Item != 0 {
				return t.Item
			}
		}
	}
	return 0
}
