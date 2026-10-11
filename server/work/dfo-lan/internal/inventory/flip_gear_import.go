package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
)

// ImportFlipGearPool discovers current EQU identities, without a historical ID
// list. Level/rarities are explicit operator policy inherited from the old pool.
func ImportFlipGearPool(a *pvf.Archive, index catalog.ItemIndex, level int32, rarities []int32) ([]uint32, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum || level <= 0 || len(rarities) == 0 {
		return nil, fmt.Errorf("flip gear source/policy missing")
	}
	var ids, out []uint32
	for id, item := range index.Items {
		if item.Kind == "equipment" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for n, id := range ids {
		if n%1024 == 0 {
			a.ReleaseReadCaches()
		}
		script, err := catalog.ResolveScript(a, index.Items[id].Path)
		if err != nil {
			return nil, fmt.Errorf("flip gear %d: %w", id, err)
		}
		if flipGearCandidate(equipmentDefinitionFromScript(id, script), level, rarities) {
			out = append(out, id)
		}
	}
	return out, nil
}
func flipGearCandidate(row EquipmentDefinition, level int32, rarities []int32) bool {
	lv, rarity, kind := row.Fields["[minimum level]"], row.Fields["[rarity]"], row.Fields["[equipment type]"]
	if len(lv) != 1 || lv[0].Type != 0 || lv[0].Value != level || len(rarity) != 1 || rarity[0].Type != 0 || len(kind) == 0 || kind[0].Type != 6 {
		return false
	}
	allowed := false
	for _, r := range rarities {
		if rarity[0].Value == r {
			allowed = true
		}
	}
	if !allowed {
		return false
	}
	switch kind[0].Text {
	case "[weapon]", "[coat]", "[pants]", "[shoulder]", "[waist]", "[shoes]", "[amulet]", "[wrist]", "[ring]", "[support]", "[magic stone]", "[earring]":
	default:
		return false
	}
	c := EquipmentCatalog{index: map[uint32]EquipmentDefinition{row.ID: row}}
	_, err := c.Reward(row.ID)
	return err == nil
}
