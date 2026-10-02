package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
)

// A rate entry owns its declared item class, not all non-gold awards. In
// particular a unique/legendary-only table cannot erase ordinary stackables
// or white/blue gear when neither of its rare equipment rolls hits.
func replaceOrdinaryRateAwards(base, from []Award, entries []catalog.DungeonDropRateEntry, c catalog.LootCatalog, pool []inventory.EquipmentDrop) []Award {
	kept := make([]Award, 0, len(base)+len(from))
	for _, item := range base {
		covered := false
		if item.Template != 0 {
			stackable := c.Items[item.Template].Kind == "stackable"
			var rarity int32
			gear := false
			if !stackable {
				for _, candidate := range pool {
					if candidate.ID == item.Template {
						rarity, gear = candidate.Rarity, true
						break
					}
				}
			}
			for _, entry := range entries {
				covered = covered || entry.Grade == "random" || stackable && entry.Grade == "stackable" || gear && ordinaryEquipmentGradeMatches(entry.Grade, rarity)
			}
		}
		if !covered {
			kept = append(kept, item)
		}
	}
	return append(kept, from...)
}

// These are EQU [rarity] values, not DungeonDropInfo's independent native
// type enum (special/epic/legendary/unique/rare/random/stackable/primeval).
// Current source group 11005 has unique rarity 3; group 11006 has legendary 6.
func ordinaryEquipmentGradeMatches(grade string, rarity int32) bool {
	switch grade {
	case "rare":
		return rarity == 2
	case "unique":
		return rarity == 3
	case "epic":
		return rarity == 4
	case "legendary":
		return rarity == 6
	default:
		return false
	}
}
