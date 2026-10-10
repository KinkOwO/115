package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"log"
	"sort"
)

// S4 Hell compatibility uses source creation weights across equipment
// rarities. Ordinary Basic's rarity<=2 gate must not suppress a Hell epic.
// Only main-bag combat gear with positive explicit creation rate is projected;
// avatars/creatures and specialised records require their own award path.
func ImportHellPartyDropPool(a *pvf.Archive, index catalog.ItemIndex, excluded []uint32) ([]EquipmentDrop, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("Hell equipment source mismatch")
	}
	deny := map[uint32]bool{}
	for _, id := range excluded {
		deny[id] = true
	}
	var ids []uint32
	for id, item := range index.Items {
		if item.Kind == "equipment" && !deny[id] {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var pool []EquipmentDrop
	missing := 0
	var missingSamples []string
	for n, id := range ids {
		if n%1024 == 0 {
			a.ReleaseReadCaches()
		}
		s, err := catalog.ResolveScript(a, index.Items[id].Path)
		if err != nil {
			// devpack 基线差异：对方历史整合的物品源脚本不在本基线 PVF（见
			// 合并记录-20261005-devpack服务端合并.md）。跳过并计数，其余照常。
			missing++
			if len(missingSamples) < 8 {
				missingSamples = append(missingSamples, fmt.Sprintf("%d %v", id, err))
			}
			continue
		}
		if candidate, ok := hellPartyDropCandidate(equipmentDefinitionFromScript(id, s)); ok {
			pool = append(pool, candidate)
		}
	}
	if missing > 0 {
		log.Printf("PVF hell party drop pool: %d equipment scripts missing (devpack baseline gap); samples: %v", missing, missingSamples)
	}
	return pool, nil
}

func hellPartyDropCandidate(row EquipmentDefinition) (EquipmentDrop, bool) {
	rate, grade, rarity, kind := row.Fields["[creation rate]"], row.Fields["[grade]"], row.Fields["[rarity]"], row.Fields["[equipment type]"]
	if len(rate) != 1 || rate[0].Type != 0 || rate[0].Value <= 0 || len(grade) != 1 || grade[0].Type != 0 || grade[0].Value < 1 || grade[0].Value > 200 || len(rarity) != 1 || rarity[0].Type != 0 || rarity[0].Value < 0 || rarity[0].Value > 8 || len(kind) == 0 {
		return EquipmentDrop{}, false
	}
	switch kind[0].Text {
	case "[weapon]", "[coat]", "[pants]", "[shoulder]", "[waist]", "[shoes]", "[amulet]", "[wrist]", "[ring]", "[support]", "[magic stone]", "[earring]":
	default:
		return EquipmentDrop{}, false
	}
	c := EquipmentCatalog{index: map[uint32]EquipmentDefinition{row.ID: row}}
	durability, err := c.Reward(row.ID)
	if err != nil {
		return EquipmentDrop{}, false
	}
	return EquipmentDrop{ID: row.ID, Grade: grade[0].Value, Rarity: rarity[0].Value, Durability: durability, Weight: uint32(rate[0].Value)}, true
}
