package loot

import "dfolan/internal/catalog"

func filterDungeonAwards(d catalog.DungeonDefinition, awards []Award) []Award {
	excludeGold := false
	for _, c := range d.Script.Cells {
		if c.Type == 3 && c.Text == "[exclude gold drop]" {
			excludeGold = true
		}
	}
	if !excludeGold {
		return awards
	}
	out := make([]Award, 0, len(awards))
	for _, a := range awards {
		if a.Template != 0 {
			out = append(out, a)
		}
	}
	return out
}
