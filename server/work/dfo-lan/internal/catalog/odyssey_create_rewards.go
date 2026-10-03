package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// OdysseyCreateRewards follows [create reward] -> fixed stackable package ->
// deferred weapon selection, fixed armor package and consumable supplies.
// Transaction keys are owned by the server and are not content identifiers.
type OdysseyCreateRewards struct {
	Source   string
	Template uint32
	ArmorBox uint32
	Armor    []SelectionItem
	Weapon   SelectionItem
	Supplies []SelectionItem
	Items    LootCatalog
}

func ImportOdysseyCreateRewards(a *pvf.Archive, index ItemIndex, definition ScriptRecord) (*OdysseyCreateRewards, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("Odyssey creation source mismatch")
	}
	out, err := parseOdysseyCreateRewards(index, definition, func(p string) (ScriptRecord, error) { return ResolveScript(a, p) })
	if err != nil {
		return nil, err
	}
	for id := range out.Items.Items {
		item, err := ImportStackableItem(a, index.Items[id])
		if err != nil {
			return nil, err
		}
		out.Items.Items[id] = item
	}
	return out, nil
}

func parseOdysseyCreateRewards(index ItemIndex, definition ScriptRecord, read func(string) (ScriptRecord, error)) (*OdysseyCreateRewards, error) {
	start, end, ok := findSectionBounds(definition.Cells, "[create reward]")
	if !ok {
		return nil, fmt.Errorf("missing Odyssey create reward")
	}
	refs := sectionCells(definition.Cells[start:end+1], "[reward data]")
	if len(refs) != 1 || refs[0].Type != 0 || refs[0].Value <= 0 {
		return nil, fmt.Errorf("invalid Odyssey create reward reference")
	}
	out := &OdysseyCreateRewards{Source: index.Source.Checksum, Template: uint32(refs[0].Value), Items: LootCatalog{Source: index.Source, Items: map[uint32]LootItem{}}}
	readItem := func(id uint32) (ScriptRecord, error) {
		entry, ok := index.Items[id]
		if !ok || entry.Kind != "stackable" {
			return ScriptRecord{}, fmt.Errorf("Odyssey create item %d missing stackable binding", id)
		}
		return read(entry.Path)
	}
	root, err := readItem(out.Template)
	if err != nil {
		return nil, err
	}
	if err = odysseyFixedCategory(root); err != nil {
		return nil, err
	}
	rootCategories, fixed := ParseSelectionCells(root.Cells)
	if fixed || len(rootCategories) != 1 || len(rootCategories[0].Sections) != 1 || rootCategories[0].Sections[0] != "[stackable]" {
		return nil, fmt.Errorf("unsupported Odyssey creation package sections")
	}
	values := sectionCells(root.Cells, "[stackable]")
	if len(values) == 0 || len(values)%2 != 0 {
		return nil, fmt.Errorf("invalid Odyssey create package rows")
	}
	seen := map[uint32]bool{}
	for i := 0; i < len(values); i += 2 {
		if values[i].Type != 0 || values[i+1].Type != 0 || values[i].Value <= 0 || values[i+1].Value <= 0 {
			return nil, fmt.Errorf("invalid Odyssey create package reward")
		}
		row := SelectionItem{Template: uint32(values[i].Value), Count: uint32(values[i+1].Value)}
		if seen[row.Template] {
			return nil, fmt.Errorf("duplicate Odyssey create reward %d", row.Template)
		}
		seen[row.Template] = true
		script, err := readItem(row.Template)
		if err != nil {
			return nil, err
		}
		entry := index.Items[row.Template]
		if entry.StackableType == "[booster selection]" {
			if row.Count != 1 {
				return nil, fmt.Errorf("unsupported repeated Odyssey create box")
			}
			categories, fixed := ParseSelectionCells(script.Cells)
			num := sectionCells(script.Cells, "[booster selection num]")
			if fixed || len(num) != 1 || num[0].Type != 0 {
				return nil, fmt.Errorf("unsupported Odyssey create selection box %d", row.Template)
			}
			if num[0].Value == 0 {
				if out.ArmorBox != 0 || odysseyFixedCategory(script) != nil || len(categories) != 1 || len(categories[0].Items) == 0 || len(categories[0].Sections) != 1 || categories[0].Sections[0] != "[equipment]" {
					return nil, fmt.Errorf("unsupported Odyssey fixed armor package")
				}
				out.ArmorBox = row.Template
				out.Armor = categories[0].Items
				for _, reward := range out.Armor {
					item, ok := index.Items[reward.Template]
					if !ok || item.Kind != "equipment" || reward.Count == 0 {
						return nil, fmt.Errorf("invalid Odyssey armor reward %d", reward.Template)
					}
				}
			} else if num[0].Value == 1 && len(categories) > 0 && out.Weapon.Template == 0 {
				out.Weapon = row
			} else {
				return nil, fmt.Errorf("unsupported Odyssey create selection count")
			}
		} else if entry.StackableType == "[waste]" {
			out.Supplies = append(out.Supplies, row)
		} else {
			return nil, fmt.Errorf("unsupported Odyssey create reward type %s", entry.StackableType)
		}
		out.Items.Items[row.Template] = LootItem{ID: row.Template, Kind: entry.Kind, StackableType: entry.StackableType, StackLimit: entry.StackLimit, Script: script}
	}
	if out.ArmorBox == 0 || out.Weapon.Template == 0 || len(out.Supplies) != 1 {
		return nil, fmt.Errorf("incomplete Odyssey creation reward chain")
	}
	return out, nil
}

func odysseyFixedCategory(script ScriptRecord) error {
	num := sectionCells(script.Cells, "[booster selection num]")
	category := sectionCells(script.Cells, "[booster select category]")
	if len(num) != 1 || num[0].Type != 0 || num[0].Value != 0 || len(category) < 2 || category[0].Type != 0 || category[1].Type != 0 || category[0].Value != 0 || category[1].Value != 0 {
		return fmt.Errorf("unsupported Odyssey fixed package category")
	}
	return nil
}

func (r *OdysseyCreateRewards) ArmorTemplates() []uint32 {
	if r == nil {
		return nil
	}
	ids := make([]uint32, len(r.Armor))
	for i, row := range r.Armor {
		ids[i] = row.Template
	}
	return ids
}
