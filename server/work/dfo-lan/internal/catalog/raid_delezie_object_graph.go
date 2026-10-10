package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

type DelezieObjectPreset struct {
	ID      uint32
	Row     int
	Objects []uint32
}

// Preserve material order; lookup/consumption and collision timing are not
// inferred from these rows. A source combination is not a network action.
type DelezieObjectCombo struct {
	Row       int
	Materials [2]uint32
	Result    uint32
}

// First/Second are positional native int operands, not yet named as AI timing
// or gameplay values. START RESULT has one, RESULTS has two per instruction.
type DelezieAIInstruction struct {
	Row           int
	Kind          string
	First, Second int32
	HasSecond     bool
}

type DelezieAIRule struct {
	ID                    uint32
	Row                   int
	StartResults, Results []DelezieAIInstruction
}

type DelezieObjectGraph struct {
	Presets   map[uint32]DelezieObjectPreset
	Combos    []DelezieObjectCombo
	AIs       map[uint32]DelezieAIRule
	ObjectAIs map[uint32]uint32
}

func delezieOptionalRoot(tree *pvf.CTPTree, tag string) (int, error) {
	rows := tree.Children(-1, tag)
	if len(rows) > 1 {
		return -1, fmt.Errorf("%s: duplicate %s root", tree.Table.Path, tag)
	}
	if len(rows) == 0 {
		return -1, nil
	}
	return rows[0].Index, nil
}

func parseDelezieAIInstructions(row pvf.CTPRecord, width int) ([]DelezieAIInstruction, error) {
	if len(row.Cells)%width != 0 {
		return nil, fmt.Errorf("row %d %s: incomplete AI operand group", row.Index, row.Name)
	}
	var out []DelezieAIInstruction
	for i := 0; i < len(row.Cells); i += width {
		cell := row.Cells[i]
		if cell.Kind != "name" || !delezieAIKind(cell.Name) {
			return nil, fmt.Errorf("row %d %s: unsupported AI instruction %q", row.Index, row.Name, cell.Name)
		}
		first, err := delezieScalarOperand(row.Cells[i+1], nil)
		if err != nil {
			return nil, fmt.Errorf("row %d %s: %w", row.Index, row.Name, err)
		}
		x := DelezieAIInstruction{Row: row.Index, Kind: cell.Name, First: first.Value}
		if width == 3 {
			second, err := delezieScalarOperand(row.Cells[i+2], nil)
			if err != nil {
				return nil, fmt.Errorf("row %d %s: %w", row.Index, row.Name, err)
			}
			x.Second, x.HasSecond = second.Value, true
		}
		out = append(out, x)
	}
	return out, nil
}

// 1476F13A0's native instruction names; these are parser contracts, not a
// separate AI content table. Operation dispatch/semantics remain unimplemented.
func delezieAIKind(name string) bool {
	switch name {
	case "MOVING ISLAND", "MOVING MY LINK OBJECT", "MOVING SYNTHESIZE", "CONSUMING", "DEAL CONSUMING", "BETWEEN DEAL CONSUMING", "AFTER DEAL CONSUMING", "CLASH":
		return true
	}
	return false
}

func parseDelezieObjectGraph(tree *pvf.CTPTree, objects map[uint32]DelezieObjectRule) (*DelezieObjectGraph, error) {
	if tree == nil || tree.Table == nil {
		return nil, fmt.Errorf("Delezie object graph source missing")
	}
	g := &DelezieObjectGraph{Presets: map[uint32]DelezieObjectPreset{}, AIs: map[uint32]DelezieAIRule{}, ObjectAIs: map[uint32]uint32{}}
	checkObject := func(row int, id uint32) error {
		if _, ok := objects[id]; !ok {
			return fmt.Errorf("%s row %d: undeclared mode object %d", tree.Table.Path, row, id)
		}
		return nil
	}
	root, err := delezieOptionalRoot(tree, "[OBJECT PRESETS]")
	if err != nil {
		return nil, err
	}
	if root >= 0 {
		for _, row := range tree.Children(root, "[OBJECT PRESET]") {
			id, err := ctpID(tree, row.Index, "[INDEX]")
			if err != nil {
				return nil, err
			}
			if _, exists := g.Presets[id]; exists {
				return nil, fmt.Errorf("%s row %d: duplicate object preset %d", tree.Table.Path, row.Index, id)
			}
			list, err := tree.One(row.Index, "[OBJECT INDEX]")
			if err != nil {
				return nil, err
			}
			ids, err := ctpUintList(list)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", tree.Table.Path, err)
			}
			for _, objectID := range ids {
				if err := checkObject(list.Index, objectID); err != nil {
					return nil, err
				}
			}
			g.Presets[id] = DelezieObjectPreset{ID: id, Row: row.Index, Objects: ids}
		}
	}
	root, err = delezieOptionalRoot(tree, "[OBJECT COMBOS]")
	if err != nil {
		return nil, err
	}
	if root >= 0 {
		for _, row := range tree.Children(root, "[OBJECT COMBO]") {
			material, err := tree.One(row.Index, "[MATERIAL]")
			if err != nil {
				return nil, err
			}
			ids, err := ctpUintList(material)
			if err != nil || len(ids) != 2 {
				return nil, fmt.Errorf("%s row %d: expected two combo materials (parse error: %v)", tree.Table.Path, row.Index, err)
			}
			result, err := ctpID(tree, row.Index, "[RESULT]")
			if err != nil {
				return nil, err
			}
			for _, id := range append(ids, result) {
				if err := checkObject(row.Index, id); err != nil {
					return nil, err
				}
			}
			g.Combos = append(g.Combos, DelezieObjectCombo{Row: row.Index, Materials: [2]uint32{ids[0], ids[1]}, Result: result})
		}
	}
	root, err = delezieOptionalRoot(tree, "[AIS]")
	if err != nil {
		return nil, err
	}
	if root >= 0 {
		for _, row := range tree.Children(root, "[AI]") {
			id, err := ctpID(tree, row.Index, "[INDEX]")
			if err != nil {
				return nil, err
			}
			if _, exists := g.AIs[id]; exists {
				return nil, fmt.Errorf("%s row %d: duplicate AI %d", tree.Table.Path, row.Index, id)
			}
			ai := DelezieAIRule{ID: id, Row: row.Index}
			for _, item := range []struct {
				tag   string
				width int
			}{{"[START RESULT]", 2}, {"[RESULTS]", 3}} {
				rows := tree.Children(row.Index, item.tag)
				if len(rows) > 1 {
					return nil, fmt.Errorf("%s row %d: duplicate AI %s", tree.Table.Path, row.Index, item.tag)
				}
				if len(rows) == 0 {
					continue
				}
				instructions, err := parseDelezieAIInstructions(rows[0], item.width)
				if err != nil {
					return nil, fmt.Errorf("%s: %w", tree.Table.Path, err)
				}
				if item.width == 2 {
					ai.StartResults = instructions
				} else {
					ai.Results = instructions
				}
			}
			g.AIs[id] = ai
		}
	}
	for id, object := range objects {
		rows := tree.Children(object.Row, "[AI INDEX]")
		if len(rows) == 0 {
			continue
		}
		aiID, err := ctpID(tree, object.Row, "[AI INDEX]")
		if err != nil {
			return nil, err
		}
		if _, ok := g.AIs[aiID]; !ok {
			return nil, fmt.Errorf("%s row %d: undeclared mode AI %d", tree.Table.Path, rows[0].Index, aiID)
		}
		g.ObjectAIs[id] = aiID
	}
	return g, nil
}
