package loot

import "dfolan/internal/catalog"

// dungeonDropExclusions 读副本脚本声明的两个「不落」标记。
//
// 三个标记（源里确实存在，见 etc/dungeondroptablebygroup.etc 与各副本 .dgn）：
//
//	[exclude gold drop]                金币不落
//	[exclude monster random drop]      怪物**随机**掉落不落
//	[exclude monster card area drop]   怪物**卡片区域**掉落不落
//
// 前两个在这里解析；[exclude monster card area drop] **刻意不解析**：服务端没有
// 「卡片区域掉落」这条独立路径 —— 怪物卡是通过宝珠/附魔的 `[monster card id]` 关联的
// （见 internal/inventory/enchant.go），不是掉落池的一类。把某个 Award 当成「卡片区域」
// 来切没有依据，做了就是照猜。该项只记录不启用。
//
// [MERGE-20260928-EXCLUDE-DROP] 曾只实现 gold，理由是「服务端从不消费
// [normal group index]」。**该理由已失效** —— session.go 现在两条路径都在消费：
//
//  1. etc/dungeondropinfo.cos  → RollDungeonGroups（带率，281 个副本）
//  2. 副本脚本 [normal group index] → DungeonGroupIndices + RollDeclaredGroups
//     （3200 个副本，不带率；件数由全局表先定的 budget 约束）
//
// 所以 [exclude monster random drop] 可以安全启用：装备来源走的是副本自选组，
// 不依赖这里切掉的「随机掉落」。
func dungeonDropExclusions(d catalog.DungeonDefinition) (excludeGold, excludeRandom bool) {
	for _, c := range d.Script.Cells {
		if c.Type != 3 {
			continue
		}
		switch c.Text {
		case "[exclude gold drop]":
			excludeGold = true
		case "[exclude monster random drop]":
			excludeRandom = true
		}
		// [exclude monster card area drop] 刻意不解析 —— 见上方注释。
	}
	return
}

// filterDungeonAwards 按副本脚本自己声明的 exclude 标记过滤掉落。
//
// === 两个标记的实际作用范围（按 Roll 的产出结构）===
//
// rules.go 的 Roll 产出 Award 只带 {Template, Amount}，**没有来源字段**。按产出顺序：
//
//	category 0 → Award{0, amount}   金币        （Template == 0）
//	category 1 → Award{id, 1}       消耗品
//	category 2 → Award{ID, 1}       装备
//	category 3 → Award{id, 1}       消耗品
//
// 因此：
//
//	「怪物随机掉落」= Template != 0 的全部（消耗品 + 装备）
//	「金币」       = Template == 0
func filterDungeonAwards(d catalog.DungeonDefinition, awards []Award) []Award {
	excludeGold, excludeRandom := dungeonDropExclusions(d)
	if !excludeGold && !excludeRandom {
		return awards
	}
	out := make([]Award, 0, len(awards))
	for _, a := range awards {
		// 金币：Template == 0，只有 gold 标记能切。
		if a.Template == 0 {
			if !excludeGold {
				out = append(out, a)
			}
			continue
		}
		// 非金币（消耗品 / 装备）：只有 random 标记能切。
		if !excludeRandom {
			out = append(out, a)
		}
	}
	return out
}
