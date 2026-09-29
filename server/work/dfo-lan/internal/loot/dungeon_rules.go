package loot

import "dfolan/internal/catalog"

func dungeonDropExclusions(d catalog.DungeonDefinition) (excludeGold, excludeRandom bool) {
	for _, c := range d.Script.Cells {
		if c.Type == 3 && c.Text == "[exclude gold drop]" {
			excludeGold = true
		}
		if c.Type == 3 && c.Text == "[exclude monster random drop]" {
			excludeRandom = true
		}
	}
	return
}

func filterDungeonAwards(d catalog.DungeonDefinition, awards []Award) []Award {
	excludeGold, excludeRandom := dungeonDropExclusions(d)
	if !excludeGold && !excludeRandom {
		return awards
	}
	out := make([]Award, 0, len(awards))
	for _, a := range awards {
		if a.Template == 0 && !excludeGold || a.Template != 0 && !excludeRandom {
			out = append(out, a)
		}
	}
	return out
}
