package inventory

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

type DropPolicy struct {
	Version           int      `json:"version"`
	Provenance        string   `json:"provenance"`
	MaximumLootGrade  uint32   `json:"maximum_loot_grade"`
	BasicEquipmentIDs []uint32 `json:"basic_equipment_ids"`
	ExcludedLootIDs   []uint32 `json:"excluded_loot_ids"`
}

func ReadDropPolicy(path string) (DropPolicy, error) {
	var out DropPolicy
	b, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&out); err != nil {
		return out, err
	}
	var tail any
	if d.Decode(&tail) != io.EOF || out.Version != 1 || out.MaximumLootGrade > 200 {
		return out, fmt.Errorf("invalid source-free drop policy")
	}
	// basic_equipment_ids 与 maximum_loot_grade 为可选（operator 策展边界，保留 JSON 不变；
	// 真正驱动掉落的是 PVF 的 etc/dungeondropinfo.cos + etc/dungeondroptablebygroup.etc，
	// 见 server/AGENTS.md §0 单一内容真源铁律）。
	if len(out.BasicEquipmentIDs) > 0 {
		seen := map[uint32]bool{}
		for _, id := range out.BasicEquipmentIDs {
			if id == 0 || seen[id] {
				return out, fmt.Errorf("duplicate/invalid basic equipment %d", id)
			}
			seen[id] = true
		}
	}
	seen := map[uint32]bool{}
	for _, id := range out.ExcludedLootIDs {
		if id == 0 || seen[id] {
			return out, fmt.Errorf("invalid/duplicate excluded loot ID %d", id)
		}
		seen[id] = true
	}
	return out, nil
}

// The basic selection is server policy. Quest additions, paths, hashes and
// eligibility fields are derived afresh from source quests/equipment scripts.
func ImportEquipmentSelection(a *pvf.Archive, index catalog.ItemIndex, quests catalog.QuestCatalog, policy DropPolicy) (*EquipmentCatalog, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum || quests.Source.Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("equipment selection source mismatch")
	}
	ids := map[uint32]bool{}
	for _, id := range policy.BasicEquipmentIDs {
		ids[id] = true
	}
	for _, q := range quests.Quests {
		active := false
		for _, t := range q.Script.Cells {
			if t.Type == 3 {
				active = t.Text == "[reward int data]" || t.Text == "[reward selection int data]"
				continue
			}
			if active && t.Type == 0 && t.Value > 0 {
				id := uint32(t.Value)
				item, ok := index.Items[id]
				if ok && (item.Kind == "equipment" || item.Kind == "avatar") {
					ids[id] = true
				}
			}
		}
	}
	order := make([]uint32, 0, len(ids))
	for id := range ids {
		order = append(order, id)
	}
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	wanted := map[string]bool{"[usable job]": true, "[usable grow type]": true, "[minimum level]": true, "[equipment type]": true, "[attach type]": true, "[rarity]": true, "[durability]": true, "[name]": true, "[grade]": true}
	c := EquipmentCatalog{Source: a.Snapshot()}
	for n, id := range order {
		if n%512 == 0 {
			a.ReleaseReadCaches()
		}
		item, ok := index.Items[id]
		if !ok || (item.Kind != "equipment" && item.Kind != "avatar") {
			return nil, fmt.Errorf("selected basic item %d missing in PVF equipment list", id)
		}
		s, err := catalog.ResolveScript(a, item.Path)
		if err != nil {
			return nil, err
		}
		row := equipmentDefinitionFromScript(id, s)
		row.fameFields = nil
		row.fameLevels = nil
		for tag := range row.Fields {
			if !wanted[tag] {
				delete(row.Fields, tag)
			}
		}
		c.Rows = append(c.Rows, row)
	}
	return NewEquipmentCatalog(c, index.Source.Checksum)
}
