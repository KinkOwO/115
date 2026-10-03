package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"strings"
)

// NoDrop keeps the existing server exclusion for a source-defined failure
// dummy. All containers, chance tables and composition results come from PVF.
type BleedingMinePolicy struct {
	NoDrop []uint32 `json:"no_drop_items"`
}

func ImportBleedingMineRewards(a *pvf.Archive, index catalog.ItemIndex, policy BleedingMinePolicy) (*BleedingMineRewards, error) {
	if a == nil || a.Snapshot().Checksum != catalog.OdysseySource || index.Source.Checksum != catalog.OdysseySource {
		return nil, fmt.Errorf("mine source mismatch")
	}
	if len(policy.NoDrop) != 1 || policy.NoDrop[0] != 10330673 {
		return nil, fmt.Errorf("mine failure-dummy exclusion must be retained")
	}
	base := "contents/2025/bleedingmine/etc/"
	rules, err := a.CTP(base + "bleedingmine.ctp")
	if err != nil {
		return nil, err
	}
	rewards, err := a.CTP(base + "bleedingminerewardscript.ctp")
	if err != nil {
		return nil, err
	}
	c := BleedingMineRewards{Source: a.Snapshot().Checksum, BossBoxes: map[uint32]uint32{}, Boxes: map[uint32][]BleedingMineDraw{}, Items: map[uint32]catalog.LootItem{}, NoDrop: append([]uint32(nil), policy.NoDrop...), Undefined: []uint32{}}
	stages, err := mineColumn(rewards, "[dungeon clear reward]")
	if err != nil {
		return nil, err
	}
	if len(stages) != 24 {
		return nil, fmt.Errorf("mine stage reward shape changed")
	}
	for n := 0; n < 12; n++ {
		if stages[n*2] != uint32(n) || stages[n*2+1] == 0 {
			return nil, fmt.Errorf("invalid mine stage reference")
		}
		c.StageBoxes = append(c.StageBoxes, stages[n*2+1])
	}
	bosses, err := mineColumn(rules, "[monster piece by dungeon]")
	if err != nil {
		return nil, err
	}
	if len(bosses) != 24 {
		return nil, fmt.Errorf("invalid mine boss references")
	}
	for i := 0; i < len(bosses); i += 2 {
		if bosses[i] == 0 || bosses[i+1] == 0 || c.BossBoxes[bosses[i]] != 0 {
			return nil, fmt.Errorf("invalid or duplicate mine boss")
		}
		c.BossBoxes[bosses[i]] = bosses[i+1]
	}
	groupRows := rules.RecordsOf("[monster piece by difficulty]")
	if len(groupRows) != 1 || len(groupRows[0].Cells) != 6 {
		return nil, fmt.Errorf("invalid mine difficulty references")
	}
	for n, difficulty := range []string{"easy", "medium", "hard"} {
		key := groupRows[0].Cells[n*2]
		if key.Kind != "name" || key.Name != difficulty {
			return nil, fmt.Errorf("mine difficulty order changed")
		}
		value, err := mineNumbers(groupRows[0].Cells[n*2+1 : n*2+2])
		if err != nil || value[0] == 0 {
			return nil, fmt.Errorf("invalid mine difficulty reward")
		}
		c.GroupBoxes = append(c.GroupBoxes, value[0])
	}
	chances, err := mineColumn(rewards, "[give bind chance]")
	if err != nil {
		return nil, err
	}
	if len(chances) != 6 {
		return nil, fmt.Errorf("invalid mine bind chance table")
	}
	for i := 1; i < len(chances); i += 2 {
		c.Combine.Chances = append(c.Combine.Chances, chances[i])
	}
	maximum, err := mineColumn(rewards, "[max bind chance]")
	if err != nil || len(maximum) != 1 {
		return nil, fmt.Errorf("invalid mine bind maximum")
	}
	c.Combine.Maximum = maximum[0]
	pending := map[uint32]bool{}
	for _, id := range c.StageBoxes {
		pending[id] = true
	}
	for _, id := range c.BossBoxes {
		pending[id] = true
	}
	for _, id := range c.GroupBoxes {
		pending[id] = true
	}
	for _, family := range []string{"equipment", "stackable"} {
		parents := rewards.RecordsOf("[" + family + " bind]")
		if len(parents) != 1 {
			return nil, fmt.Errorf("ambiguous mine %s bind table", family)
		}
		table := map[string][]uint32{}
		for _, row := range rewards.Records {
			if row.Parent != parents[0].Index {
				continue
			}
			key := strings.Trim(row.Name, "[]")
			if _, duplicate := table[key]; duplicate {
				return nil, fmt.Errorf("duplicate mine bind column %s", key)
			}
			values, err := mineNumbers(row.Cells)
			if err != nil {
				return nil, err
			}
			table[key] = values
			if strings.HasSuffix(key, "list") {
				for _, id := range values {
					pending[id] = true
				}
			}
		}
		if family == "equipment" {
			c.Combine.Equipment = table
		} else {
			c.Combine.Stackable = table
		}
	}
	smart, err := mineSmartGroups(a)
	if err != nil {
		return nil, err
	}
	for len(pending) > 0 {
		var id uint32
		for n := range pending {
			id = n
			break
		}
		delete(pending, id)
		if id == 0 {
			continue
		}
		if _, seen := c.Items[id]; seen {
			continue
		}
		entry, ok := index.Items[id]
		if !ok {
			return nil, fmt.Errorf("mine reward template %d missing source index", id)
		}
		// Preserve the original index-only storage projection. Grade/Weight and
		// Script were not projected by the old reward importer.
		meta := catalog.LootItem{ID: id, Kind: entry.Kind, StackableType: entry.StackableType, StackLimit: entry.StackLimit}
		if entry.Kind != "stackable" {
			c.Items[id] = meta
			continue
		}
		script, err := catalog.ReadScript(a, entry.Path)
		if err != nil {
			return nil, err
		}
		if rarity, ok := blackNativeInt(script.Cells, "[rarity]"); ok {
			meta.Rarity = rarity
		}
		c.Items[id] = meta
		var smartID uint32
		if value, ok := blackNativeInt(script.Cells, "[smart drop group id]"); ok && value > 0 {
			smartID = uint32(value)
		}
		inside := false
		for at, cell := range script.Cells {
			if cell.Type != 3 {
				continue
			}
			if cell.Text == "[booster info]" {
				inside = true
				continue
			}
			if cell.Text == "[/booster info]" {
				inside = false
				continue
			}
			if !(inside && (cell.Text == "[etc]" || cell.Text == "[stackable]" || cell.Text == "[equipment]" || cell.Text == "[creature]")) && !(entry.StackableType == "[upgradable legacy]" && cell.Text == "[int data]") {
				continue
			}
			var values []int32
			for _, v := range script.Cells[at+1:] {
				if v.Type == 3 {
					break
				}
				if v.Type != 0 {
					return nil, fmt.Errorf("non-integer mine reward token %d", id)
				}
				values = append(values, v.Value)
			}
			draws := uint32(1)
			if inside {
				if len(values) == 0 || values[0] <= 0 {
					return nil, fmt.Errorf("invalid mine reward draw count")
				}
				draws = uint32(values[0])
				values = values[1:]
			}
			if len(values)%3 != 0 || draws > 100 {
				return nil, fmt.Errorf("invalid mine reward pool %d", id)
			}
			pool := BleedingMineDraw{Draws: draws}
			substitute := false
			for n := 0; n < len(values); n += 3 {
				if values[n+1] <= 0 || values[n+2] <= 0 {
					return nil, fmt.Errorf("invalid mine reward weight/count")
				}
				candidate := BleedingMineChoice{Template: int64(values[n]), Weight: uint32(values[n+1]), Count: uint32(values[n+2])}
				pool.Candidates = append(pool.Candidates, candidate)
				if candidate.Template >= 490000000 && candidate.Template < 490001000 {
					substitute = true
				}
			}
			if substitute {
				if len(smart[smartID]) == 0 {
					return nil, fmt.Errorf("mine reward %d unresolved smart group %d", id, smartID)
				}
				pool.Candidates = append([]BleedingMineChoice(nil), smart[smartID]...)
			}
			for _, candidate := range pool.Candidates {
				if candidate.Template > 0 {
					pending[uint32(candidate.Template)] = true
				}
			}
			c.Boxes[id] = append(c.Boxes[id], pool)
		}
	}
	seenNoDrop := map[uint32]bool{}
	for _, id := range c.NoDrop {
		if id == 0 || seenNoDrop[id] || c.Items[id].ID != id {
			return nil, fmt.Errorf("invalid mine no-drop exclusion %d", id)
		}
		seenNoDrop[id] = true
	}
	return NewBleedingMineRewards(c)
}

func mineNumbers(cells []pvf.CTPCell) ([]uint32, error) {
	values := make([]uint32, 0, len(cells))
	for _, c := range cells {
		var value float64
		switch c.Kind {
		case "float":
			value = c.Float
		case "byte":
			value = float64(c.Byte)
		default:
			return nil, fmt.Errorf("unsupported mine numeric cell %s", c.Kind)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > math.MaxUint32 || value != math.Trunc(value) {
			return nil, fmt.Errorf("invalid mine integer cell")
		}
		values = append(values, uint32(value))
	}
	return values, nil
}

func mineColumn(table *pvf.CTPTable, name string) ([]uint32, error) {
	rows := table.RecordsOf(name)
	if len(rows) != 1 {
		return nil, fmt.Errorf("ambiguous mine source column %s", name)
	}
	return mineNumbers(rows[0].Cells)
}

func mineSmartGroups(a *pvf.Archive) (map[uint32][]BleedingMineChoice, error) {
	script, err := catalog.ReadScript(a, "etc/dungeondroptablebygroup.etc")
	if err != nil {
		return nil, err
	}
	groups := map[uint32][]BleedingMineChoice{}
	var current uint32
	var section string
	var values []int32
	flush := func() {
		if (section != "[drop item]" && section != "[smart drop item]") || len(values) == 0 {
			return
		}
		if len(values)%2 != 0 {
			groups[current] = nil
			return
		}
		for n := 0; n < len(values); n += 2 {
			groups[current] = append(groups[current], BleedingMineChoice{Template: int64(values[n]), Weight: uint32(values[n+1]), Count: 1})
		}
	}
	for _, cell := range script.Cells {
		if cell.Type == 3 {
			flush()
			section = cell.Text
			values = nil
		} else if cell.Type == 0 {
			if section == "[group]" {
				current = uint32(cell.Value)
			} else if section == "[drop item]" || section == "[smart drop item]" {
				values = append(values, cell.Value)
			}
		}
	}
	flush()
	return groups, nil
}
