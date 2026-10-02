package inventory

import (
	"fmt"
	"sort"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
)

// Ordinary pool membership is discovered from EQU definitions using the
// existing Basic acceptance rule. Explicit creation rates are respected.
// Unspecified rates retain the existing compatibility model's uniform weight
// (one); that is server policy, not a recovered native creation-rate default.
// Do not derive generation weights from ItemDictionary's flag column.
func ImportOrdinaryDropPool(a *pvf.Archive, index catalog.ItemIndex, excluded []uint32) ([]EquipmentDrop, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("ordinary equipment source mismatch")
	}
	deny := map[uint32]bool{}
	for _, id := range excluded {
		deny[id] = true
	}
	ids := make([]uint32, 0)
	for id, item := range index.Items {
		if item.Kind == "equipment" && !deny[id] {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var pool []EquipmentDrop
	for n, id := range ids {
		if n%1024 == 0 {
			a.ReleaseReadCaches()
		}
		script, err := catalog.ResolveScript(a, index.Items[id].Path)
		if err != nil {
			return nil, fmt.Errorf("ordinary equipment %d: %w", id, err)
		}
		row := equipmentDefinitionFromScript(id, script)
		if candidate, ok := ordinaryDropCandidate(row); ok {
			pool = append(pool, candidate)
		}
	}
	return pool, nil
}

func ordinaryDropCandidate(row EquipmentDefinition) (EquipmentDrop, bool) {
	weight := uint32(1)
	if rate, exists := row.Fields["[creation rate]"]; exists {
		if len(rate) != 1 || rate[0].Type != 0 || rate[0].Value <= 0 {
			return EquipmentDrop{}, false
		}
		weight = uint32(rate[0].Value)
	}
	grade := row.Fields["[grade]"]
	if len(grade) != 1 || grade[0].Type != 0 || grade[0].Value <= 0 || grade[0].Value > 200 {
		return EquipmentDrop{}, false
	}
	// Main equipment only. Avatar, creature and specialised inventories have
	// different presentation/storage paths and cannot enter an ordinary card.
	kind := row.Fields["[equipment type]"]
	if len(kind) == 0 {
		return EquipmentDrop{}, false
	}
	switch kind[0].Text {
	case "[weapon]", "[coat]", "[pants]", "[shoulder]", "[waist]", "[shoes]", "[amulet]", "[wrist]", "[ring]", "[support]", "[magic stone]", "[earring]":
	default:
		return EquipmentDrop{}, false
	}
	c := EquipmentCatalog{index: map[uint32]EquipmentDefinition{row.ID: row}}
	durability, err := c.Basic(row.ID)
	if err != nil {
		return EquipmentDrop{}, false
	}
	return EquipmentDrop{ID: row.ID, Grade: grade[0].Value, Rarity: row.Fields["[rarity]"][0].Value, Durability: durability, Weight: weight}, true
}
