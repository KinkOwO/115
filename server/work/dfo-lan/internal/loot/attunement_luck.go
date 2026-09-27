package loot

// 「幸运事件」的两个档。
//
// tier 标签是 luck15 / luck30 —— 官方社区实测里叫「小幸运 ×15 / 大幸运 ×50」，
// 客户端那边的盒子名是 "Endkeeper of Order Mystical Fortune … Reward (CS)"。
//
// 这条线**不需要服务端新增任何逻辑**：它们本来就落在 fixed 池里（小深渊 maze 0 的
// 权重是 3333 / 500，即 0.3333% / 0.05%，与社区实测 7/1710 与 1/1710 吻合），抽中后
// 由通用的开箱路径展开 —— 而盒子的 pool1 是 draw=14 / draw=29，「倍数」就写在
// 那张表里，不在代码里。
//
// 这个访问器存在的唯一目的，是让启动日志能把它念出来，免得它一直是一件
// 「看不见的活」。
const (
	luckTierSmall = "luck15"
	luckTierLarge = "luck30"
)

// LuckTemplates 返回两个幸运档用到的模板，按 fixed 表里出现的顺序
// （先小幸运、后大幸运）。
func (a *AttunementRewards) LuckTemplates() []uint32 {
	if a == nil {
		return nil
	}
	seen := map[uint32]bool{}
	var out []uint32
	for _, t := range a.Tables {
		for _, f := range t.Fixed {
			for _, e := range f.Entries {
				if e.Tier != luckTierSmall && e.Tier != luckTierLarge {
					continue
				}
				if e.Item != 0 && !seen[e.Item] {
					seen[e.Item] = true
					out = append(out, e.Item)
				}
			}
		}
	}
	return out
}

// LuckWeights 返回某副本某迷宫段上两个幸运档的权重（百万空间）。
// 注意小深渊只有 maze 0 带这两条，maze 1 把它们的位置全给了普通档。
func (a *AttunementRewards) LuckWeights(dungeon, maze uint32) (small, large uint32) {
	tiers := a.FixedTiers(dungeon, maze)
	return tiers[luckTierSmall], tiers[luckTierLarge]
}
