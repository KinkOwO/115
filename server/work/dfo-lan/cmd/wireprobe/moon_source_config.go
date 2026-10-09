package main

import (
	"fmt"
	"sort"
	"strings"

	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
)

// 本文件把月湖单人的翻牌策略**从源声明装配**出来（D1：只换产出模型，不动领取事务）。
//
// 选块口径 D6：单人走 `[contents] = normal` 那一块（它的 `[reward multiple info]` 是
// 装备 7 / 誓约·星蕴石 3，正是玩家看到的 700% / 300%）；源里若没有带 Contents 的块
// （老副本）就退回落单块 —— 只退到"源里确实存在的那一块"，不合成。
func moonSourceBlock(decls []catalog.DungeonRewardBlock) (catalog.DungeonRewardBlock, bool) {
	for _, b := range decls {
		if strings.EqualFold(strings.TrimSpace(b.Contents), "normal") {
			return b, true
		}
	}
	for _, b := range decls {
		if len(b.Special) > 0 || len(b.Multiple) > 0 {
			return b, true
		}
	}
	return catalog.DungeonRewardBlock{}, false
}

// conquestObservedFixedCounts 是**玩家实测**的固定产物数量，**按结算副本分组**。
//
// 源里没有"这次给几张"这种表，所以这些数只能沿用实测值，其余成员按源声明各 1 件 ——
// 见 loot.MoonSourceFixedChoices。两族的固定组恰好同为 21291（门票 Doom Oracle /
// 迷雾工商协会银币 / 通宝袖珍罐），所以数量沿用同一份实测；哪个副本换了组就补一行。
var conquestObservedFixedCounts = map[uint32]map[uint32]uint32{
	100004137: {10362429: 60, 10362432: 15}, // 沉月湖第二层（2026-10-02 实机截图）
	// 蔚蓝号：**L0 取值** —— 官服抓包 `F16-s2c.txt` 第 682 帧（id=35, body=616）的头两条行就是 `10362432 x160`、`10362429 x100`。
	// 原来挂的 100004134 是猜的：蔚蓝号真正进的是频道 [guide dungeon index] = **100004131**（见 azure_flip.go）。
	100004131: {10362429: 100, 10362432: 160},
}

// conquestFamilyDungeons 是「征讨地下城」这一族的副本号。
//
// 装备的**品级组**（21276/21277/21278）只由这一族里**某一层**声明（实测是沉月湖一层
// 100004136），蔚蓝号两层都只声明 `[normal group index] 2 21251 3 1 21291`。所以装配时
// 取**整族的并集**，再按结算层选块与效率（[reward multiple info]）。
var conquestFamilyDungeons = []uint32{100004136, 100004137, 100004131, 100004134}

// conquestFamily 从已加载的副本目录里挑出征讨族的成员（缺的不算 —— 不新开导入，
// 目录里没有就把那一层当不存在，族并集会少声明而已）。
func conquestFamily(dungeons *catalog.DungeonCatalog, ids []uint32) []catalog.DungeonDefinition {
	if dungeons == nil {
		return nil
	}
	out := make([]catalog.DungeonDefinition, 0, len(ids))
	for _, id := range ids {
		if d, ok := dungeons.Dungeons[id]; ok {
			out = append(out, d)
		}
	}
	return out
}

// conquestFlipPolicy 装配**征讨地下城**的翻牌策略（通用）。
//
// 分工：**装配在 cmd 层**（要知道源目录与装备目录），**掷骰与结算判据在 internal/loot**
// （与发奖同源）。返回的第二值是给启动日志用的一句话摘要。
//
// 参数把「族」与「结算层」分开：
//
//	settlement —— 通关结算落在哪一层：它的 [custom group info]（[contents]=normal）给块、
//	              效率（[reward multiple info]）与基础产物声明；
//	family     —— 整族（多层）。装备**品级组**只由族里某一层声明（沉月湖一层），
//	              所以组号取整族并集。
//
// 实测（2026-10-09）沉月湖与蔚蓝号除「效率」「卡片池」外完全同构：五条声明同名同组、
// 誓约池 100 件、套装池 396 件都相同 —— 差异全部来自源，不在这里写死。
func conquestFlipPolicy(svc *loot.Service, family []catalog.DungeonDefinition, settlement catalog.DungeonDefinition, booster *catalog.BoosterCatalog) (loot.MoonSourcePolicy, string, error) {
	var policy loot.MoonSourcePolicy
	if svc == nil {
		return policy, "", fmt.Errorf("月湖源驱动翻牌需要掉落服务")
	}
	block, ok := moonSourceBlock(settlement.RewardDecls)
	if !ok {
		return policy, "", fmt.Errorf("结算层 %d 的源里没有翻牌声明（[difficulty dropitem group list]）", settlement.ID)
	}
	// 组号清单：**这一族（一层 + 二层）所有块声明过的组并集**。
	//
	// 为什么不是只看二层：源把四个品级各声明成一个组，而这些组是**一层**（100004136）声明的
	// （实测 `[4, 21251, 21276, 21277, 21278, 1, 21291]`），二层只声明 21251 与 100 级大池（组 3）。
	// 只读二层会永远只出"稀有"（业主 2026-10-09 实机就是这个现象）。
	groups, e := moonFamilyGroups(family)
	if e != nil {
		return policy, "", e
	}
	if len(groups) == 0 {
		return policy, "", fmt.Errorf("结算层 %d 这一族没有声明任何 [normal group index]", settlement.ID)
	}
	// 产出池 = 源里"全是装备模板"的那些组；固定产物 = 含堆叠物的那些组（判据不依赖装备目录）。
	poolGroups, fixedGroups := svc.SplitMoonSourceGroups(groups)
	fixedMembers := svc.MoonSourceGroupMembers(fixedGroups)

	rarityPool, rarityNote := moonRarityPool(svc, poolGroups, settlement.MinimumLevel)
	equipTemplates := svc.MoonSourceEquipmentTemplates(block, svc.MoonSourceGroupMembers(poolGroups))
	equipPool, poolNote := moonEquipmentPool(svc, equipTemplates, settlement.MinimumLevel)

	// 誓约/结晶装备池：源里 [special setinfo reward] 指向的组装的是**显示件**
	// （21468 的 12 个「X Oath/Crystal Set」盒、21470 的「Dim Oath Crystal」，都是 [etc]
	// 且没有产出段、玩家打不开）。真正的誓约/结晶是 equipment/character/common/{oath,primer}
	// 下的**装备**（业主口径「誓约和星蕴石本质也是装备，也有品质 4 档」），
	// 池子按结构从掉落表里收：整组都是 [oath]/[primer] 装备的组。
	oathPool, oathNote := svc.MoonOathEquipmentPool()
	// 套装装备池：`SetEquipmentReward` 的 12 个盒也是**占位件**（24 格、没有产出段、
	// 名字是套装名不是物品名）。真装备在掉落表里——「12 块 × 11 件」的全装备组，
	// 块序与声明的 12 个盒逐一对位。见 loot.MoonEquipmentSetPool。
	setPool, setNote := svc.MoonEquipmentSetPool(block)
	// 真·客户端可开的罐子（booster 且声明了 `[oath item booster]`/`[lottery ani info]`）。
	// 判据与深渊**同一条**（cmd/wireprobe/booster_flow.go 的 boosterBoxSource.serverUnwrap）：
	// 命中的原样落地让玩家自己点开，其余一律展开 —— 业主 2026-10-09 明确的口径。
	keepAsIs := moonKeepAsIsBoxes(booster)

	policy = loot.MoonSourcePolicy{
		Source:            svc.Catalog.Source.SaveIdentity(),
		SettlementDungeon: settlement.ID,
		Block:             block,
		Fixed:         loot.MoonSourceFixedChoices(fixedMembers, conquestObservedFixedCounts[settlement.ID]),
		// 品级→模板（优先走这条）；源模板建不出品级摊平时才退回 EquipmentPool 的 Roll 路径。
		RarityPool:       rarityPool,
		EquipmentPool:    equipPool,
		OathPool:         oathPool,
		SetEquipmentPool: setPool,
		KeepAsIs:         keepAsIs,
		Level:         byte(settlement.MinimumLevel),
		Rank:          moonSoloEquipmentRank,
		// ⚠️ 2026-10-09 纠正：`ConquestClearReward115` 里的 `[8]` 是**座位数**（多人组队每人一组），
		// 每个座位的产物是**变长列表**，真实上限是 `len(...) > 126` 才拒绝 —— 而每件产物在客户端
		// 就是**独立一格**（业主 + 玩家实证）。所以这里不是 8 个格子，而是 126 件的帧上限。
		MaxRows: 126,
	}
	note := fmt.Sprintf("块=%s 倍数=%v 组[产出池=%v 固定=%v] 固定条目=%d 品级池=%d 装备池=%d 誓约装备池=%d 套装装备池=%d 原样落地罐子=%d 条目=%d；%s；%s；%s；%s",
		block.Contents, block.Multiple, poolGroups, fixedGroups, len(policy.Fixed),
		len(rarityPool), len(policy.EquipmentPool), len(policy.OathPool), len(policy.SetEquipmentPool),
		len(policy.KeepAsIs), len(block.Special),
		rarityNote, poolNote, oathNote, setNote)
	return policy, note, nil
}

// moonKeepAsIsBoxes 收集「真·客户端可开的罐子」的模板：booster 目录里声明了
// `[oath item booster]` / `[lottery ani info]`（`ClientOpenPath`）的那些。
//
// 与深渊**同一条判据**（cmd/wireprobe/booster_flow.go 的 boosterBoxSource.serverUnwrap）：
// 只有这些才原样落地让玩家自己在消耗品里点开；其余（含两个标记都没有的一般礼盒）
// 一律服务端展开 —— 客户端没有入口的盒子落到玩家脚下就是死件（10419728 那次实机反馈）。
func moonKeepAsIsBoxes(b *catalog.BoosterCatalog) map[uint32]bool {
	if b == nil {
		return nil
	}
	out := map[uint32]bool{}
	for id, def := range b.Definitions {
		if def.ClientOpenPath {
			out[id] = true
		}
	}
	return out
}

// moonPoolCandidate 是一个平台装备池的候选（用于翻牌掷骰）。
type moonPoolCandidate struct {
	name string
	pool []inventory.EquipmentDrop
}

// moonSourceEquipmentDrops 用**源声明的装备模板**直读完整装备目录建掷骰池。
//
// 为什么不能复用平台的三个池（2026-10-09 实机启动日志实测）：
//
//	DropPool=2794 / OrdinaryPool=6905 / HellPartyPool=5701，**最高 grade 全部只有 107、命中源模板 0 个**
//
// 根因在 `internal/inventory/equipment.go` 的 `Basic()`：它是平台池的入口，明确拒绝
// `[rarity] > 2`（"special equipment reward requires additional source state"）
// ⇒ 池里只有 rarity ≤ 2 的老货，115 级的稀有~太初永远进不去 —— 业主实机看到的
// 「level 100/105、rarity 2 的魔法封印稀有装备」就是这么来的。
//
// 翻牌这条路走的是 `moonEquipmentDestination`（只要求"定义可查 + 装备源给得出耐久"），
// 所以可以直接用源模板从**完整目录**（`Equipment.Full`，实测 424216 条）建池。
//
// 等级过滤按副本 `[minimum required level]`（115）：源里同一张产出池混着 100 级大池，
// 不过滤会被低级货淹没（这条口径与本仓旧实现一致）。
func moonSourceEquipmentDrops(svc *loot.Service, want []uint32, minLevel uint32) ([]inventory.EquipmentDrop, string) {
	if svc == nil || svc.Equipment == nil {
		return nil, "装备目录不可用（Equipment=nil）"
	}
	// 两个目录类型都有 Definition(id)：*FullEquipmentCatalog（完整）与 *EquipmentCatalog（选择集）。
	var src interface {
		Definition(uint32) (inventory.EquipmentDefinition, error)
	} = svc.Equipment.Full
	via := "Full"
	if src == nil {
		src, via = svc.Equipment, "Selection"
	}
	var out []inventory.EquipmentDrop
	var missing, lowLevel, badShape int
	for _, tpl := range want {
		if tpl == 0 {
			continue
		}
		def, e := src.Definition(tpl)
		if e != nil {
			missing++
			continue
		}
		if minLevel > 0 {
			if lv := def.Fields["[minimum level]"]; len(lv) == 1 && lv[0].Type == 0 && uint32(lv[0].Value) < minLevel {
				lowLevel++
				continue
			}
		}
		grade, rarity := def.Fields["[grade]"], def.Fields["[rarity]"]
		if len(grade) != 1 || grade[0].Type != 0 || grade[0].Value <= 0 || len(rarity) != 1 || rarity[0].Type != 0 || rarity[0].Value < 0 {
			badShape++
			continue
		}
		durability, e := svc.Equipment.Reward(tpl)
		if e != nil {
			missing++
			continue
		}
		out = append(out, inventory.EquipmentDrop{ID: tpl, Grade: grade[0].Value, Rarity: rarity[0].Value, Durability: durability})
	}
	note := fmt.Sprintf("源模板%d个 ⇒ 建池%d个（目录=%s 缺定义%d 低于%d级%d 形状不符%d）",
		len(want), len(out), via, missing, minLevel, lowLevel, badShape)
	return out, note
}

// moonEquipmentPool 选翻牌装备掷骰用的池。
//
// **优先**用源模板直读建的池（见 moonSourceEquipmentDrops）；源模板建不出池时，
// 退回"与平台三池求交"的老办法并记日志；再退不了就用 legacy 池 —— 每一步都写清原因，
// 宁可日志刺眼也不要静默发低档货。
func moonEquipmentPool(svc *loot.Service, want []uint32, minLevel uint32) ([]inventory.EquipmentDrop, string) {
	if svc == nil || svc.Equipment == nil {
		return nil, "装备目录不可用（Equipment=nil）"
	}
	direct, note := moonSourceEquipmentDrops(svc, want, minLevel)
	if len(direct) > 0 {
		return direct, note
	}
	candidates := []moonPoolCandidate{
		{"DropPool", svc.Equipment.DropPool()},
		{"OrdinaryPool", svc.Equipment.OrdinaryPool},
		{"HellPartyPool", svc.Equipment.HellPartyPool},
	}
	best := 0
	bestHits := -1
	var parts []string
	for i, c := range candidates {
		hits := len(loot.MoonSourceEquipmentPool(c.pool, want))
		maxGrade := uint32(0)
		for _, d := range c.pool {
			if g := uint32(d.Grade); g > maxGrade {
				maxGrade = g
			}
		}
		parts = append(parts, fmt.Sprintf("%s=%d条/命中%d/最高grade%d", c.name, len(c.pool), hits, maxGrade))
		if hits > bestHits {
			bestHits, best = hits, i
		}
	}
	note = note + "；池候选[" + strings.Join(parts, " ") + "]"
	if bestHits <= 0 {
		return candidates[0].pool, note + " ⇒ 都没命中，退回 DropPool（注意：可能出低档装备）"
	}
	return candidates[best].pool, fmt.Sprintf("%s ⇒ 回退选中 %s（命中源模板 %d 个）", note, candidates[best].name, bestHits)
}

// moonFamilyGroups 汇总这一族（一层 + 二层）所有块声明过的组号（去重、保序）。
func moonFamilyGroups(dungeons []catalog.DungeonDefinition) ([]uint32, error) {
	seen := map[uint32]bool{}
	var out []uint32
	for _, d := range dungeons {
		for _, b := range d.RewardDecls {
			gs, e := loot.ExtractGroupIndices(b.Groups)
			if e != nil {
				return nil, fmt.Errorf("副本 %d 的 [normal group index]: %w", d.ID, e)
			}
			for _, g := range gs {
				if !seen[g] {
					seen[g] = true
					out = append(out, g)
				}
			}
		}
	}
	return out, nil
}

// moonRarityPool 把产出池组的成员按**品级**摊平（品级→模板）。
//
// 源里这一族把四个品级各声明成一个组（实测 21251=rarity2 / 21276=3 / 21277=4 / 21278=6，
// 每组 11 件 115 级装备）⇒ "品级"这个维度在源里就是**组**，不需要另找表。
// 品级数值取自完整装备目录的 `[rarity]`；低于副本 `[minimum required level]` 的成员按旧口径过滤
// （组 3 的 792 件 ≤100 级就是这么滤掉的）。任何成员读不到定义都只跳过它，不整批失败。
func moonRarityPool(svc *loot.Service, groups []uint32, minLevel uint32) (map[int32][]uint32, string) {
	out := map[int32][]uint32{}
	if svc == nil || svc.Equipment == nil || len(groups) == 0 {
		return nil, "品级池：装备目录不可用或没有产出组"
	}
	var src interface {
		Definition(uint32) (inventory.EquipmentDefinition, error)
	} = svc.Equipment.Full
	via := "Full"
	if src == nil {
		src, via = svc.Equipment, "Selection"
	}
	var missing, low, bad int
	for _, gid := range groups {
		for _, tpl := range svc.MoonSourceGroupMembers([]uint32{gid}) {
			def, e := src.Definition(tpl)
			if e != nil {
				missing++
				continue
			}
			if minLevel > 0 {
				if lv := def.Fields["[minimum level]"]; len(lv) == 1 && lv[0].Type == 0 && uint32(lv[0].Value) < minLevel {
					low++
					continue
				}
			}
			r := def.Fields["[rarity]"]
			if len(r) != 1 || r[0].Type != 0 || r[0].Value < 0 {
				bad++
				continue
			}
			out[r[0].Value] = append(out[r[0].Value], tpl)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Sprintf("品级池：0 件（目录=%s 缺定义%d 低于%d级%d 形状不符%d）", via, missing, minLevel, low, bad)
	}
	// 掷骰权重（业主 2026-10-09 口径）：源 rarity 表前两档占 96.5%，池里只有 2/3/4/6 ⇒
	// 把无模板那几档的比例按权重摊给有模板的几档（= 在可用档上重归一化）。这里只打印结果，
	// 口径本体在 loot.MoonRarityWeights。
	weightNote := ""
	if tiers, weights := loot.MoonRarityWeights(svc.Tables.Rarity, out); len(tiers) > 0 {
		wp := make([]string, 0, len(tiers))
		for i, t := range tiers {
			wp = append(wp, fmt.Sprintf("rarity%d=%.1f%%", t, weights[i]*100))
		}
		weightNote = "；掷骰权重[" + strings.Join(wp, " ") + "]（重归一化到池里有的档）"
	}
	keys := make([]int, 0, len(out))
	for k := range out {
		keys = append(keys, int(k))
	}
	sort.Ints(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("rarity%d:%d件", k, len(out[int32(k)])))
	}
	return out, fmt.Sprintf("品级池[%s]（目录=%s 缺定义%d 低于%d级%d 形状不符%d）%s",
		strings.Join(parts, " "), via, missing, minLevel, low, bad, weightNote)
}
