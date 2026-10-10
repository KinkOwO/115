package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
)

// STATIC HP uses native 1474CD3B0 (CVTTSD2SI RAX), distinct from the
// int32 scalar reader. Reject invalid conversions, including 2^63, which
// float64(MaxInt64) also rounds to; do not let conversion wrap to MinInt64.
func delezieInt64(cell pvf.CTPCell) (int64, error) {
	v := math.Trunc(cell.Float)
	if cell.Tag != 4 || cell.Kind != "float" || math.IsNaN(v) || math.IsInf(v, 0) || v < -0x1p63 || v >= 0x1p63 {
		return 0, fmt.Errorf("invalid native int64 cell tag=%d kind=%q", cell.Tag, cell.Kind)
	}
	return int64(v), nil
}

func bindDelezieObjectExecution(tree *pvf.CTPTree, parent int, symbols map[string]uint32, object *DelezieObjectRule) error {
	for _, tag := range []string{"[STATIC HP]", "[CREATABLE LOCATION INDEX]", "[LOCK SYMBOL]"} {
		rows := tree.Children(parent, tag)
		if len(rows) == 0 {
			continue
		}
		if len(rows) != 1 {
			return fmt.Errorf("%s row %d: duplicate %s", tree.Table.Path, parent, tag)
		}
		row := rows[0]
		fail := func(err error) error { return fmt.Errorf("%s row %d %s: %w", tree.Table.Path, row.Index, tag, err) }
		switch tag {
		case "[STATIC HP]":
			if len(row.Cells) != 1 {
				return fail(fmt.Errorf("expected one int64 HP operand"))
			}
			v, err := delezieInt64(row.Cells[0])
			if err != nil {
				return fail(err)
			}
			object.StaticHP, object.HasStaticHP = v, true
		case "[CREATABLE LOCATION INDEX]":
			if len(row.Cells) == 0 {
				return fail(fmt.Errorf("empty location pool"))
			}
			for _, cell := range row.Cells {
				v, err := delezieScalarOperand(cell, nil)
				if err != nil || v.IsSymbol || v.Value < 0 {
					return fail(fmt.Errorf("invalid native location operand"))
				}
				object.CreatableLocations = append(object.CreatableLocations, uint32(v.Value))
			}
		case "[LOCK SYMBOL]":
			if len(row.Cells) != 1 {
				return fail(fmt.Errorf("expected one lock symbol"))
			}
			v, err := delezieScalarSymbol(row.Cells[0], symbols)
			if err != nil {
				return fail(err)
			}
			object.LockSymbol, object.LockSymbolID = v.Symbol, v.SymbolID
		}
	}
	return nil
}

// ObjectProgram compiles from the same owned mode's native object tree.
// Location pools remain source declarations; spawning also needs the selected
// phase to declare the destination. No normal-mode locations are inherited.
func (r *DelezieRaidRules) ObjectProgram(mode *DelezieModeRules) (map[uint32]DelezieObjectRule, error) {
	if r == nil || mode == nil || mode.Difficulty.Name() == "" || r.Modes[mode.Difficulty] != mode {
		return nil, fmt.Errorf("Delezie object mode does not belong to source rules")
	}
	return parseDelezieObjects(mode.Objects, r.SymbolIDs)
}

// ObjectGraphProgram recompiles the exact owned mode's current source tree;
// execution must not borrow a cached or another difficulty's preset list.
func (r *DelezieRaidRules) ObjectGraphProgram(mode *DelezieModeRules) (*DelezieObjectGraph, error) {
	objects, err := r.ObjectProgram(mode)
	if err != nil {
		return nil, err
	}
	return parseDelezieObjectGraph(mode.Objects, objects)
}
