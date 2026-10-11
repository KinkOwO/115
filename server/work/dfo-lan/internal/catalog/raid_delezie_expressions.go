package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
)

// Scalar operands are bound to the same Source's symbol declarations. This is
// an operand index, not an event interpreter: event order, nested predicates,
// object actions and notification timing remain in the original CTPTree.
type DelezieScalarOperand struct {
	IsSymbol bool
	Symbol   string
	SymbolID uint32
	Value    int32
}

type DelezieScalarExpression struct {
	Row      int
	Kind     string
	Target   string
	TargetID uint32
	Operator string
	Operand  DelezieScalarOperand
}

// ScalarProgram binds a fresh program to the exact mode owned by these source
// rules. It never trusts an exported audit or a caller-supplied expression map.
// A program contains scalar operations only, not the surrounding event actions.
func (r *DelezieRaidRules) ScalarProgram(mode *DelezieModeRules) (map[int]DelezieScalarExpression, error) {
	if r == nil || mode == nil || mode.Difficulty.Name() == "" || r.Modes[mode.Difficulty] != mode {
		return nil, fmt.Errorf("Delezie scalar mode does not belong to source rules")
	}
	if len(r.SymbolIDs) == 0 {
		return nil, fmt.Errorf("Delezie scalar symbol declarations missing")
	}
	ids := make(map[uint32]bool, len(r.SymbolIDs))
	for name, id := range r.SymbolIDs {
		if name == "" || ids[id] {
			return nil, fmt.Errorf("Delezie scalar invalid symbol declaration %d/%q", id, name)
		}
		ids[id] = true
	}
	return parseDelezieScalarExpressions(mode.Phase, r.SymbolIDs)
}

func delezieScalarSymbol(cell pvf.CTPCell, symbols map[string]uint32) (DelezieScalarOperand, error) {
	if cell.Kind != "name" || cell.Name == "" {
		return DelezieScalarOperand{}, fmt.Errorf("expected an unlocalized symbol name")
	}
	id, ok := symbols[cell.Name]
	if !ok {
		return DelezieScalarOperand{}, fmt.Errorf("undeclared symbol %q", cell.Name)
	}
	return DelezieScalarOperand{IsSymbol: true, Symbol: cell.Name, SymbolID: id}, nil
}

func delezieScalarOperand(cell pvf.CTPCell, symbols map[string]uint32) (DelezieScalarOperand, error) {
	if cell.Kind == "name" {
		return delezieScalarSymbol(cell, symbols)
	}
	// 1474CD320 accepts native cell tag 4 only, then CVTTSD2SI truncates to
	// signed int32. Float-alt/tag 5 is not accepted by that reader. Reject
	// invalid conversions instead of silently producing its indefinite value.
	v := math.Trunc(cell.Float)
	if cell.Kind != "float" || cell.Tag != 4 || math.IsNaN(v) || math.IsInf(v, 0) || v < math.MinInt32 || v > math.MaxInt32 {
		return DelezieScalarOperand{}, fmt.Errorf("invalid native signed scalar cell (tag=%d kind=%q)", cell.Tag, cell.Kind)
	}
	return DelezieScalarOperand{Value: int32(v)}, nil
}

func parseDelezieScalarExpressions(tree *pvf.CTPTree, symbols map[string]uint32) (map[int]DelezieScalarExpression, error) {
	if tree == nil || tree.Table == nil {
		return nil, fmt.Errorf("Delezie scalar source missing")
	}
	out := map[int]DelezieScalarExpression{}
	for _, row := range tree.Table.Records {
		if row.Name != "[IF]" && row.Name != "[SET]" {
			continue
		}
		fail := func(err error) (map[int]DelezieScalarExpression, error) {
			return nil, fmt.Errorf("%s row %d %s: %w", tree.Table.Path, row.Index, row.Name, err)
		}
		// Current Delezie [IF] uses target/operator/operand. [SET] also has
		// target/number shorthand. No fallback to Bakal flattened bytecode.
		if len(row.Cells) != 3 && !(row.Name == "[SET]" && len(row.Cells) == 2) {
			return fail(fmt.Errorf("unsupported scalar arity %d", len(row.Cells)))
		}
		target, err := delezieScalarSymbol(row.Cells[0], symbols)
		if err != nil {
			return fail(err)
		}
		x := DelezieScalarExpression{Row: row.Index, Kind: row.Name, Target: target.Symbol, TargetID: target.SymbolID, Operator: "[=]"}
		operandCell := row.Cells[len(row.Cells)-1]
		if len(row.Cells) == 2 {
			if operandCell.Kind != "float" {
				return fail(fmt.Errorf("two-cell SET requires a numeric operand"))
			}
		} else {
			op := row.Cells[1]
			if op.Kind != "name" || !delezieScalarOperator(row.Name, op.Name) {
				return fail(fmt.Errorf("unsupported scalar operator %q", op.Name))
			}
			x.Operator = op.Name
		}
		x.Operand, err = delezieScalarOperand(operandCell, symbols)
		if err != nil {
			return fail(err)
		}
		out[row.Index] = x
	}
	return out, nil
}

// These are reader contract names (1470A9A20 for IF, 1477FB710 for SET),
// not gameplay values. Numeric native op enums and execution are not inferred
// here. Only forms used by the current source and verified readers are bound.
func delezieScalarOperator(kind, op string) bool {
	if kind == "[IF]" {
		switch op {
		case "[==]", "[>]", "[<]", "[>=]", "[<=]", "[!=]":
			return true
		}
	} else if kind == "[SET]" {
		switch op {
		case "[=]", "[+]", "[-]", "[*]", "[/]", "[MOD]":
			return true
		}
	}
	return false
}
