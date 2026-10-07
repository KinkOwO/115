package boostup

import (
	"fmt"
)

type GroupLookup interface{ ForTemplate(uint32) []int32 }

// GroupIndex 是 equipmentgrouping 的「模板 -> 分组号」视图。分组号由装配层从
// character.FameRules.Groups 注入（名望侧已经有同一张 .etc 的唯一解析器），
// 本包不再自己读第二遍源。
type GroupIndex map[uint32][]int32

func (g GroupIndex) ForTemplate(template uint32) []int32 { return g[template] }

type WornFact struct {
	Slot     uint16
	Template uint32
	Kind     string // source equipment type, not a client-supplied label
}
type WearRequirement struct {
	Enchanted bool
	Kinds     []string
	Groups    []int32
	Count     uint32
}

// These names are the event's labels, not raw worn slot numbers. Source item
// type/slot validation is done by the inventory adapter before facts arrive.
var missionEquipmentKinds = map[string]string{
	"weapon": "[weapon]", "jacket": "[coat]", "pants": "[pants]", "shoulder": "[shoulder]", "waist": "[waist]", "shoes": "[shoes]",
	"amulet": "[amulet]", "wrist": "[wrist]", "ring": "[ring]", "support": "[support]", "magic stone": "[magic stone]", "earring": "[earring]",
	"title": "[title name]", "creature": "[creature]", "aurora": "[aurora avatar]",
	"artifact red": "[artifact red]", "artifact blue": "[artifact blue]", "artifact green": "[artifact green]",
}

func (row Step) WearRequirement() (*WearRequirement, error) {
	if row.Mission != "equip item" && row.Mission != "transform equip journal or equip item" && row.Mission != "equip enchanted item" {
		return nil, nil
	}
	conditions, err := sections(row.MissionCells, "[equip condition]", "[/equip condition]")
	if err != nil {
		return nil, err
	}
	groups, err := sections(row.MissionCells, "[equip grouping]", "[/equip grouping]")
	if err != nil {
		return nil, err
	}
	if len(conditions)+len(groups) != 1 {
		return nil, fmt.Errorf("ambiguous/missing wear mission source")
	}
	r := &WearRequirement{Enchanted: row.Mission == "equip enchanted item"}
	if len(conditions) == 1 {
		cells := conditions[0]
		seen := map[string]bool{}
		for at := 0; at < len(cells); {
			if at+3 >= len(cells) || cells[at].Type != 3 || cells[at].Text != "[type]" || cells[at+1].Type != 6 || cells[at+2].Type != 3 || cells[at+3].Type != 3 || cells[at+2].Text != "[condition]" || cells[at+3].Text != "[/condition]" {
				return nil, fmt.Errorf("unsupported nonempty wear condition")
			}
			kind, ok := missionEquipmentKinds[cells[at+1].Text]
			if !ok || seen[kind] {
				return nil, fmt.Errorf("unknown/duplicate wear condition")
			}
			seen[kind] = true
			r.Kinds = append(r.Kinds, kind)
			at += 4
		}
		if len(r.Kinds) == 0 {
			return nil, fmt.Errorf("empty wear conditions")
		}
		return r, nil
	}
	r.Count, err = number(groups[0], "[count]", 64)
	if err != nil || r.Count == 0 {
		return nil, fmt.Errorf("invalid wear group count")
	}
	seen := map[int32]bool{}
	counts := 0
	for at := 0; at < len(groups[0]); at += 2 {
		v := groups[0]
		if at+1 >= len(v) || v[at].Type != 3 || v[at+1].Type != 0 {
			return nil, fmt.Errorf("invalid wear group source")
		}
		switch v[at].Text {
		case "[count]":
			counts++
		case "[index]":
			id := v[at+1].Value
			if id < 0 || seen[id] {
				return nil, fmt.Errorf("invalid/duplicate wear group")
			}
			seen[id] = true
			r.Groups = append(r.Groups, id)
		default:
			return nil, fmt.Errorf("unknown wear group field")
		}
	}
	if len(r.Groups) == 0 || counts != 1 {
		return nil, fmt.Errorf("empty wear groups")
	}
	return r, nil
}

func (r *WearRequirement) Satisfied(worn []WornFact, groups GroupLookup, enchantments ...map[uint16]uint32) (bool, error) {
	if r == nil {
		return false, nil
	}
	if len(enchantments) > 1 || r.Enchanted && len(enchantments) != 1 {
		return false, fmt.Errorf("missing enchantment proof")
	}
	if len(r.Kinds) == 0 && (len(r.Groups) == 0 || r.Count == 0) {
		return false, fmt.Errorf("empty wear requirement")
	}
	seen := map[uint16]bool{}
	kinds := map[string]bool{}
	count := uint32(0)
	wanted := map[int32]bool{}
	for _, g := range r.Groups {
		wanted[g] = true
	}
	if len(wanted) > 0 && groups == nil {
		return false, fmt.Errorf("wear mission grouping source missing")
	}
	for _, item := range worn {
		if seen[item.Slot] || item.Template == 0 || item.Template == ^uint32(0) {
			return false, fmt.Errorf("invalid worn identity")
		}
		seen[item.Slot] = true
		if !r.Enchanted || enchantments[0][item.Slot] != 0 {
			kinds[item.Kind] = true
		}
		if len(wanted) > 0 {
			for _, g := range groups.ForTemplate(item.Template) {
				if wanted[g] {
					count++
					break
				} // overlapping groups still count ONE item
			}
		}
	}
	for _, k := range r.Kinds {
		if !kinds[k] {
			return false, nil
		}
	}
	return len(r.Kinds) > 0 || count >= r.Count, nil
}

// Validate the source clauses we currently consume before exposing the event.
func (c *Catalog) ValidateWearMissions() error {
	for _, step := range c.Steps {
		if _, err := step.WearRequirement(); err != nil {
			return fmt.Errorf("boost step%d: %w", step.Number, err)
		}
	}
	return nil
}
