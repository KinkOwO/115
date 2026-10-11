package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// Native declarations are source identities, not behavior implementations.
// In particular timer subtype IDs are scoped to one main type; subtype zero
// occurs in several timers and must not become a shared global timer.
type DelezieTimerRule struct {
	ID       uint32
	Name     string
	Row      int
	Subtypes map[string]uint32
}

type DelezieBuffRule struct {
	Type string
	Row  int
}

type delezieNamedID struct {
	ID   uint32
	Name string
}

// Declaration cells alternate number/name. Localized name_indexed cells have
// different semantics and must not be accepted as an executable symbol name.
func delezieNamedIDs(row pvf.CTPRecord) ([]delezieNamedID, error) {
	if len(row.Cells) == 0 || len(row.Cells)%2 != 0 {
		return nil, fmt.Errorf("row %d %s: incomplete number/name declarations", row.Index, row.Name)
	}
	ids, names := map[uint32]bool{}, map[string]bool{}
	var out []delezieNamedID
	for i := 0; i < len(row.Cells); i += 2 {
		n := row
		n.Cells = row.Cells[i : i+1]
		id, err := ctpUint(n)
		if err != nil {
			return nil, err
		}
		c := row.Cells[i+1]
		if c.Kind != "name" || c.Name == "" || ids[id] || names[c.Name] {
			return nil, fmt.Errorf("row %d %s: invalid or duplicate declaration %d/%q", row.Index, row.Name, id, c.Name)
		}
		ids[id], names[c.Name] = true, true
		out = append(out, delezieNamedID{id, c.Name})
	}
	return out, nil
}

func parseDelezieSymbols(tree *pvf.CTPTree) (map[string]uint32, error) {
	row, err := tree.One(-1, "[SYMBOL]")
	if err != nil {
		return nil, err
	}
	pairs, err := delezieNamedIDs(row)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tree.Table.Path, err)
	}
	out := make(map[string]uint32, len(pairs))
	for _, p := range pairs {
		out[p.Name] = p.ID
	}
	return out, nil
}

func parseDelezieTimers(tree *pvf.CTPTree) (map[string]DelezieTimerRule, error) {
	out, ids := map[string]DelezieTimerRule{}, map[uint32]bool{}
	for _, root := range tree.Children(-1, "[TIMER]") {
		main, err := tree.One(root.Index, "[MAIN TYPE]")
		if err != nil {
			return nil, err
		}
		pairs, err := delezieNamedIDs(main)
		if err != nil || len(pairs) != 1 {
			return nil, fmt.Errorf("%s row %d: expected one timer main declaration (parse error: %v)", tree.Table.Path, root.Index, err)
		}
		p := pairs[0]
		if _, exists := out[p.Name]; exists || ids[p.ID] {
			return nil, fmt.Errorf("%s row %d: duplicate timer %d/%q", tree.Table.Path, root.Index, p.ID, p.Name)
		}
		sub, err := tree.One(root.Index, "[SUB TYPE]")
		if err != nil {
			return nil, err
		}
		subPairs, err := delezieNamedIDs(sub)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", tree.Table.Path, err)
		}
		rule := DelezieTimerRule{ID: p.ID, Name: p.Name, Row: root.Index, Subtypes: map[string]uint32{}}
		for _, s := range subPairs {
			rule.Subtypes[s.Name] = s.ID
		}
		out[p.Name], ids[p.ID] = rule, true
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no timer declarations", tree.Table.Path)
	}
	return out, nil
}

func parseDelezieBuffs(tree *pvf.CTPTree) (map[string]DelezieBuffRule, error) {
	root, err := tree.One(-1, "[RAID BUFF]")
	if err != nil {
		return nil, err
	}
	out := map[string]DelezieBuffRule{}
	for _, row := range tree.Children(root.Index, "[BUFF INFO]") {
		typ, err := tree.One(row.Index, "[TYPE]")
		if err != nil {
			return nil, err
		}
		if len(typ.Cells) != 1 || typ.Cells[0].Kind != "name" || typ.Cells[0].Name == "" {
			return nil, fmt.Errorf("%s row %d: invalid buff type", tree.Table.Path, row.Index)
		}
		name := typ.Cells[0].Name
		if _, exists := out[name]; exists {
			return nil, fmt.Errorf("%s row %d: duplicate buff type %q", tree.Table.Path, row.Index, name)
		}
		// Passive index -1 and arbitrary INT DATA remain in the original tree.
		// This index never treats a missing/passive-less effect as buff ID zero.
		out[name] = DelezieBuffRule{Type: name, Row: row.Index}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no buff declarations", tree.Table.Path)
	}
	return out, nil
}
