package loot

import "dfolan/internal/inventory"

// dropCreatureEggs 从奖励清单里剔掉「宠物蛋」模板。
//
// 背景（2026-10-05，业主实机口径）：普通副本的掉落池按装备表展开时会把宠物蛋一起
// 带进来，但蛋只能通过**指定的继承/魔盒/礼包**发放，不该从地牢地面掉落。蛋一旦掉
// 在地上并被打包拾取，客户端会在孵化路径上给出错误结果。
//
// 判定真源：`inventory.EggHatchOutputs` —— 当前 PVF 已解析出的「蛋模板 → 孵化产物」
// 表（`internal/inventory/equipment_family.go`），蛋的定义就是这张表的键；不新增
// 平行清单，也不再从 PVF 之外的地方派生。
//
// 说明（交给下一轮核对的缺口）：交付包里 `internal/loot/session.go` 调用了本函数，
// 但**函数本体随附文件（原树内名为 `egg_drop_filter.go`）没有进包**，本仓历史里也从
// 未有该符号。因此这里按上面的口径做了忠实重写；语义边界（是否还要按地下城/难度
// 分类、是否只过滤特定稀有度）以实机为准，若与包作者原实现有差异，以实机日志校正。
func dropCreatureEggs(_ *inventory.EquipmentCatalog, awards []Award) []Award {
	if len(awards) == 0 || len(inventory.EggHatchOutputs) == 0 {
		return awards
	}
	kept := awards[:0]
	for _, award := range awards {
		if _, isEgg := inventory.EggHatchOutputs[award.Template]; isEgg {
			continue
		}
		kept = append(kept, award)
	}
	return kept
}
