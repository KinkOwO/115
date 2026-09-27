package main

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"dfolan/internal/loot"
)

// logAttunementRebalance 把调参后的关键数字打出来。
//
// 这是**与官服的显式差异**，所以它必须自己说出来：不打日志的话，半年后有人拿这份
// 表去对官方数据，只会得出「我们的掉落表是错的」这个结论，而真相是我们有意调的。
func logAttunementRebalance(a *loot.AttunementRewards, r loot.Rebalance) {
	if a == nil || !r.Enabled() {
		return
	}
	log.Printf("attunement rebalance ON — this is a deliberate deviation from the official table "+
		"(omen idle halved: %v, fixed common tilt: %d%%)", r.OmenHalveIdle, r.FixedTiltPercent)
	for _, d := range a.Dungeons() {
		if rows := a.OmenStages(d); len(rows) > 0 {
			var parts []string
			for _, st := range rows {
				idle := int(attunementOmenSpace) - int(st.ObtainProb) - int(st.DropProb)
				parts = append(parts,
					fmt.Sprintf("hold %d: idle %d%% / +1 stage %d%% / settle %d%%",
						st.Index, idle/10000, st.ObtainProb/10000, st.DropProb/10000))
			}
			log.Printf("  dungeon %d omen: %s", d, strings.Join(parts, " | "))
		}
		if tiers := a.FixedTiers(d, 0); len(tiers) > 0 {
			log.Printf("  dungeon %d fixed(maze 0): %s", d, summariseFixedTiers(tiers))
		}
	}
}

// attunementOmenSpace 只是给上面的百分比换算用的分母。它与 internal/loot 里那个
// 1e6 必须一致，但那个常量没有导出；写在这里并加一条断言，比导出它更省事。
const attunementOmenSpace = 1000000

// summariseFixedTiers 把一个档位汇总排成「档 权重(百分比)」的一行，按权重降序，
// 方便一眼看出调整后的形状。
func summariseFixedTiers(tiers map[string]uint32) string {
	keys := make([]string, 0, len(tiers))
	var total uint64
	for k, w := range tiers {
		keys = append(keys, k)
		total += uint64(w)
	}
	sort.Slice(keys, func(i, j int) bool {
		if tiers[keys[i]] != tiers[keys[j]] {
			return tiers[keys[i]] > tiers[keys[j]]
		}
		return keys[i] < keys[j]
	})
	var parts []string
	for _, k := range keys {
		pct := float64(tiers[k]) * 100 / float64(total)
		parts = append(parts, fmt.Sprintf("%s %d(%.2f%%)", k, tiers[k], pct))
	}
	return strings.Join(parts, " · ")
}
