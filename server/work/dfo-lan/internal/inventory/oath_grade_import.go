package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"
)

func ImportOathGrades(a *pvf.Archive) (*OathGradeTable, error) {
	if a == nil {
		return nil, fmt.Errorf("missing oath archive")
	}
	source, err := json.Marshal(a.Snapshot())
	if err != nil {
		return nil, err
	}
	out := &OathGradeTable{Source: source, Entries: map[string]OathGradeEntry{}}
	err = a.IterateFilesUnder("equipment/character/common", func(f pvf.File) error {
		p := strings.ToLower(f.ArchivePath)
		family := ""
		if strings.HasPrefix(p, "equipment/character/common/oath/") {
			family = "oath"
		}
		if strings.HasPrefix(p, "equipment/character/common/primer/") {
			family = "primer"
		}
		if family == "" || path.Ext(p) != ".equ" {
			return nil
		}
		s, err := catalog.ReadScript(a, p)
		if err != nil {
			return err
		}
		d := equipmentDefinitionFromScript(0, s)
		e := OathGradeEntry{Family: family, Path: p, Rarity: -1}
		rarity := d.Fields["[rarity]"]
		kind := d.Fields["[equipment type]"]
		grade := d.Fields["[grade]"]
		if len(rarity) != 1 || rarity[0].Type != 0 || len(kind) == 0 || kind[0].Type != 6 || kind[0].Text != "["+family+"]" {
			return fmt.Errorf("invalid oath source %s", p)
		}
		e.Rarity = rarity[0].Value
		e.Type = kind[0].Text
		if len(grade) > 0 {
			if len(grade) != 1 || grade[0].Type != 0 {
				return fmt.Errorf("invalid oath grade %s", p)
			}
			e.Grade = grade[0].Value
		}
		id := strings.TrimSuffix(path.Base(p), ".equ")
		if _, err := strconv.ParseUint(id, 10, 32); err != nil {
			return err
		}
		if _, dup := out.Entries[id]; dup {
			return fmt.Errorf("duplicate oath item %s", id)
		}
		out.Entries[id] = e
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, out.Validate()
}
