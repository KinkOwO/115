package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
)

const OdysseyGrowthPath = "contents/2026/aradodyssey/etc/aradodyssey.etc"

// ImportOdysseyGrowth follows gift and graduation references from the native
// definition. Creation rewards follow the same ETC and native item references.
func ImportOdysseyGrowth(a *pvf.Archive, index ItemIndex) (*OdysseyGrowth, error) {
	if a == nil || a.Snapshot().Checksum != OdysseySource || index.Source.Checksum != OdysseySource {
		return nil, fmt.Errorf("Odyssey growth source mismatch")
	}
	definition, err := ReadScript(a, OdysseyGrowthPath)
	if err != nil {
		return nil, err
	}
	c := OdysseyGrowth{Source: a.Snapshot().Checksum, Definition: definition, Items: map[uint32]ScriptRecord{}}
	ids := map[uint32]bool{}
	rewards := sectionCells(definition.Cells, "[reward info]")
	if len(rewards) == 0 || len(rewards)%2 != 0 {
		return nil, fmt.Errorf("invalid Odyssey reward references")
	}
	for i := 1; i < len(rewards); i += 2 {
		if rewards[i].Type != 0 || rewards[i].Value <= 0 {
			return nil, fmt.Errorf("invalid Odyssey gift reference")
		}
		ids[uint32(rewards[i].Value)] = true
	}
	graduate, err := sectionRewardTemplate(definition.Cells, "[complete reward info]")
	if err != nil {
		return nil, err
	}
	ids[graduate] = true
	for id := range ids {
		entry, ok := index.Items[id]
		if !ok || entry.Kind != "stackable" {
			return nil, fmt.Errorf("missing Odyssey stackable %d", id)
		}
		script, err := ReadScript(a, entry.Path)
		if err != nil {
			return nil, err
		}
		c.Items[id] = script
	}
	c.Creation, err = ImportOdysseyCreateRewards(a, index, definition)
	if err != nil {
		return nil, err
	}
	return NewOdysseyGrowth(c)
}

type OdysseyWeaponChoices struct {
	Item       LootItem     `json:"-"`
	Definition ScriptRecord `json:"definition"`
	Source     string       `json:"source"`
	Template   uint32       `json:"template"`
	Categories []struct {
		Category [2]byte  `json:"category"`
		Items    []uint32 `json:"items"`
	} `json:"categories"`
}

func ValidateOdysseyWeaponChoices(c OdysseyWeaponChoices) (OdysseyWeaponChoices, error) {
	if c.Source != OdysseySource || c.Template != 10417789 || len(c.Categories) != 85 || c.Definition.SHA256 != "d67f5042a5e17ef30581e297f030561de39f92ad5e215d742ec067aeaad96bec" {
		return c, fmt.Errorf("invalid Odyssey weapon source")
	}
	return c, nil
}

func LoadOdysseyWeaponChoices(path string) (OdysseyWeaponChoices, error) {
	var c OdysseyWeaponChoices
	raw, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, err
	}
	return ValidateOdysseyWeaponChoices(c)
}

func ImportOdysseyWeaponChoices(a *pvf.Archive, index ItemIndex) (OdysseyWeaponChoices, error) {
	var c OdysseyWeaponChoices
	if a == nil || a.Snapshot().Checksum != OdysseySource || index.Source.Checksum != OdysseySource {
		return c, fmt.Errorf("Odyssey weapon source mismatch")
	}
	definition, err := ReadScript(a, OdysseyGrowthPath)
	if err != nil {
		return c, err
	}
	creation, err := ImportOdysseyCreateRewards(a, index, definition)
	if err != nil {
		return c, err
	}
	script := creation.Items.Items[creation.Weapon.Template].Script
	c.Source, c.Template, c.Definition = a.Snapshot().Checksum, creation.Weapon.Template, script
	c.Item = creation.Items.Items[c.Template]
	categories, fixed := ParseSelectionCells(script.Cells)
	if fixed {
		return c, fmt.Errorf("Odyssey creation box has fixed rewards")
	}
	for _, category := range categories {
		row := struct {
			Category [2]byte  `json:"category"`
			Items    []uint32 `json:"items"`
		}{Category: category.Category}
		for _, item := range category.Items {
			if item.Template == 0 || item.Count != 1 {
				return c, fmt.Errorf("invalid Odyssey weapon choice count")
			}
			definition, ok := index.Items[item.Template]
			if !ok || definition.Kind != "equipment" {
				return c, fmt.Errorf("Odyssey weapon choice %d missing equipment", item.Template)
			}
			row.Items = append(row.Items, item.Template)
		}
		c.Categories = append(c.Categories, row)
	}
	if len(c.Categories) == 0 {
		return c, fmt.Errorf("empty native Odyssey weapon choices")
	}
	return c, nil
}

// ImportStackableItem projects a source template for a specific reward table.
// Weight remains zero: these templates do not enter the ordinary random pool.
func ImportStackableItem(a *pvf.Archive, entry ItemIndexEntry) (LootItem, error) {
	var item LootItem
	if a == nil || entry.ID == 0 || entry.Kind != "stackable" {
		return item, fmt.Errorf("invalid reward stackable")
	}
	script, err := ReadScript(a, entry.Path)
	if err != nil {
		return item, err
	}
	grade, ok := lootInt(script.Cells, "[grade]")
	if !ok {
		return item, fmt.Errorf("missing reward grade %d", entry.ID)
	}
	rarity, ok := lootInt(script.Cells, "[rarity]")
	if !ok {
		return item, fmt.Errorf("missing reward rarity %d", entry.ID)
	}
	return LootItem{ID: entry.ID, Kind: entry.Kind, Grade: grade, Rarity: rarity, StackableType: entry.StackableType, StackLimit: entry.StackLimit, Script: script}, nil
}
