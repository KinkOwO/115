package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
)

func ImportKnightShields(a *pvf.Archive, index catalog.ItemIndex, jobs catalog.Characters, rules WearRules) (*KnightShields, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum || jobs.Source.Checksum != index.Source.Checksum || rules.Source != index.Source.Checksum || rules.Slots["[support weapon]"] != 24 {
		return nil, fmt.Errorf("knight shield source/slot mismatch")
	}
	var profession byte
	found := false
	for id, job := range jobs.Professions {
		if job.Job == "[knight]" {
			if found {
				return nil, fmt.Errorf("ambiguous knight profession")
			}
			profession = id
			found = true
		}
	}
	if !found {
		return nil, fmt.Errorf("knight profession missing")
	}
	window, err := catalog.ResolveScript(a, "etc/character/knight/shieldwindownewdata.etc")
	if err != nil {
		return nil, err
	}
	rows, err := ParseKnightShieldWindow(window.Cells)
	if err != nil {
		return nil, err
	}
	c := &KnightShields{Source: a.Snapshot(), Chain: window.Path, WindowSHA256: window.SHA256, WearSource: rules.Source, Profession: profession}
	for _, r := range rows {
		item, ok := index.Items[r.Item]
		if !ok || item.Kind != "equipment" {
			return nil, fmt.Errorf("shield %d not in equipment source list", r.Item)
		}
		script, err := catalog.ResolveScript(a, item.Path)
		if err != nil {
			return nil, err
		}
		d := equipmentDefinitionFromScript(r.Item, script)
		kind, sub := d.Fields["[equipment type]"], d.Fields["[sub type]"]
		if len(kind) == 0 || kind[0].Text != "[support weapon]" {
			return nil, fmt.Errorf("shield %d is not support weapon", r.Item)
		}
		r.EquSHA256 = d.SHA256
		r.WearSlot = rules.Slots["[support weapon]"]
		if len(sub) > 0 {
			r.SubType = sub[0].Value
		} else if len(kind) > 1 {
			r.SubType = kind[1].Value
		}
		c.Rows = append(c.Rows, r)
	}
	return c, c.Validate(index.Source.Checksum)
}

func ParseKnightShieldWindow(cells []pvf.Token) ([]KnightShieldRow, error) {
	var rows []KnightShieldRow
	var current *KnightShieldRow
	tag := ""
	seen := map[string]bool{}
	for _, t := range cells {
		if t.Type == 3 {
			if t.Text == "[shield]" {
				if current != nil {
					return nil, fmt.Errorf("nested shield entry")
				}
				current = &KnightShieldRow{Position: uint16(len(rows) + 1)}
				seen = map[string]bool{}
			} else if t.Text == "[/shield]" && current != nil {
				rows = append(rows, *current)
				current = nil
			}
			tag = t.Text
			continue
		}
		if current == nil {
			continue
		}
		switch tag {
		case "[item index]", "[grow type]", "[get condition]", "[get shield level]", "[open quest index]", "[clear quest index]":
			if seen[tag] {
				return nil, fmt.Errorf("duplicate shield field %s", tag)
			}
			seen[tag] = true
			if tag == "[get condition]" {
				if t.Type != 6 || (t.Text != "level" && t.Text != "quest") {
					return nil, fmt.Errorf("unknown shield condition %q", t.Text)
				}
				current.Condition = t.Text
				continue
			}
			if t.Type != 0 || t.Value < 0 {
				return nil, fmt.Errorf("invalid shield field %s", tag)
			}
			switch tag {
			case "[item index]":
				current.Item = uint32(t.Value)
			case "[grow type]":
				current.GrowType = uint32(t.Value)
			case "[get shield level]":
				if t.Value > 255 {
					return nil, fmt.Errorf("shield level exceeds byte")
				}
				current.RequiredLevel = byte(t.Value)
			case "[open quest index]":
				current.OpenQuest = uint32(t.Value)
			case "[clear quest index]":
				current.ClearQuest = uint32(t.Value)
			}
		}
	}
	if current != nil {
		return nil, fmt.Errorf("unterminated shield entry")
	}
	return rows, nil
}
