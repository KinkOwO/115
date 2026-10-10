package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// 147722295 compiles native action 0x7f: count, then count pairs of
// symbol/numeric operands. 1477221e4 is the distinct preset action 0x80,
// whose count covers individual operands. Do not apply that reader here.
// First/Second are positional until the action consumer's meaning is proven.
type DelezieSummonPair struct {
	First, Second DelezieScalarOperand
}

type DelezieSummonAction struct {
	Row         int
	Pairs       []DelezieSummonPair
	IgnoredTail []pvf.CTPCell
}

func (r *DelezieRaidRules) SummonProgram(mode *DelezieModeRules) (map[int]DelezieSummonAction, error) {
	if r == nil || mode == nil || mode.Difficulty.Name() == "" || r.Modes[mode.Difficulty] != mode {
		return nil, fmt.Errorf("Delezie summon mode does not belong to source rules")
	}
	return parseDelezieSummonActions(mode.Phase, r.SymbolIDs)
}

func parseDelezieSummonActions(tree *pvf.CTPTree, symbols map[string]uint32) (map[int]DelezieSummonAction, error) {
	if tree == nil || tree.Table == nil {
		return nil, fmt.Errorf("Delezie summon source missing")
	}
	out := map[int]DelezieSummonAction{}
	for _, row := range tree.Table.Records {
		if row.Name != "[SUMMON OBJECTS]" {
			continue
		}
		fail := func(err error) (map[int]DelezieSummonAction, error) {
			return nil, fmt.Errorf("%s row %d %s: %w", tree.Table.Path, row.Index, row.Name, err)
		}
		if len(row.Cells) == 0 {
			return fail(fmt.Errorf("missing pair count"))
		}
		count, err := delezieScalarOperand(row.Cells[0], nil)
		if err != nil || count.IsSymbol || count.Value < 0 || int64(count.Value) > int64((len(row.Cells)-1)/2) {
			return fail(fmt.Errorf("invalid or truncated native pair count"))
		}
		x := DelezieSummonAction{Row: row.Index}
		end := 1 + int(count.Value)*2 // bounded by the source cell count above
		for i := 1; i < end; i += 2 {
			first, err := delezieScalarOperand(row.Cells[i], symbols)
			if err != nil {
				return fail(err)
			}
			second, err := delezieScalarOperand(row.Cells[i+1], symbols)
			if err != nil {
				return fail(err)
			}
			x.Pairs = append(x.Pairs, DelezieSummonPair{First: first, Second: second})
		}
		// The native branch ends after the counted pairs. Preserve unused
		// source cells for inspection; do not invent an optional action flag.
		x.IgnoredTail = append([]pvf.CTPCell(nil), row.Cells[end:]...)
		out[row.Index] = x
	}
	return out, nil
}
