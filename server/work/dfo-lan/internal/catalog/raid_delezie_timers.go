package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
)

// Main/Sub is the native identity pair. Subtype IDs are scoped to Main;
// merging different main types whose subtype is zero corrupts timer state.
type DelezieTimerKey struct{ Main, Sub uint32 }

type DelezieTimerReference struct {
	Row               int
	Kind              string
	MainName, SubName string
	Key               DelezieTimerKey
	HasValue          bool
	Value             int32
}

func delezieTimerKey(cells []pvf.CTPCell, timers map[string]DelezieTimerRule) (DelezieTimerReference, error) {
	var out DelezieTimerReference
	if len(cells) != 2 {
		return out, fmt.Errorf("expected timer main/sub pair")
	}
	if cells[0].Kind == "name" && cells[1].Kind == "name" {
		main, ok := timers[cells[0].Name]
		if !ok {
			return out, fmt.Errorf("undeclared timer main %q", cells[0].Name)
		}
		sub, ok := main.Subtypes[cells[1].Name]
		if !ok {
			return out, fmt.Errorf("undeclared timer subtype %q under %q", cells[1].Name, main.Name)
		}
		out.MainName, out.SubName = cells[0].Name, cells[1].Name
		out.Key = DelezieTimerKey{Main: main.ID, Sub: sub}
		if main.ID > math.MaxInt32 || sub > math.MaxInt32 {
			return out, fmt.Errorf("timer identity exceeds native signed integer range")
		}
		return out, nil
	}
	// SET TIMER, RESET TIMER and CHECK TIMER END also have native numeric
	// main/sub forms. Bind them to declarations, never fabricate missing IDs.
	mainID, err := delezieScalarOperand(cells[0], nil)
	if err != nil || mainID.IsSymbol || mainID.Value < 0 {
		return out, fmt.Errorf("invalid numeric timer main")
	}
	subID, err := delezieScalarOperand(cells[1], nil)
	if err != nil || subID.IsSymbol || subID.Value < 0 {
		return out, fmt.Errorf("invalid numeric timer subtype")
	}
	for mainName, main := range timers {
		if main.ID != uint32(mainID.Value) {
			continue
		}
		for subName, id := range main.Subtypes {
			if id == uint32(subID.Value) {
				out.MainName, out.SubName = mainName, subName
				out.Key = DelezieTimerKey{Main: main.ID, Sub: id}
				return out, nil
			}
		}
	}
	return out, fmt.Errorf("undeclared numeric timer %d/%d", mainID.Value, subID.Value)
}

// This binds source operands, not elapsed-time behavior. In particular -1 and
// zero are retained; neither is interpreted as delete, pause or immediate fire
// before the native timer executor contract is established.
func parseDelezieTimerReferences(tree *pvf.CTPTree, timers map[string]DelezieTimerRule) (map[int]DelezieTimerReference, error) {
	if tree == nil || tree.Table == nil {
		return nil, fmt.Errorf("Delezie timer source missing")
	}
	out := map[int]DelezieTimerReference{}
	for _, row := range tree.Table.Records {
		arity := 0
		switch row.Name {
		case "[CHECK TIMER END]", "[RESET TIMER]", "[TIMER]":
			arity = 2
		case "[SET TIMER]", "[HP PENALTY SET TIMER]", "[ING SET TIMER]", "[REPEAT SET TIMER]", "[SUMMON SET TIMER]", "[NORMAL SUMMON SET TIMER]", "[FINAL SUMMON SET TIMER]":
			arity = 3
		default:
			continue
		}
		if len(row.Cells) != arity {
			return nil, fmt.Errorf("%s row %d %s: expected %d timer cells, got %d", tree.Table.Path, row.Index, row.Name, arity, len(row.Cells))
		}
		ref, err := delezieTimerKey(row.Cells[:2], timers)
		if err == nil && arity == 3 {
			var value DelezieScalarOperand
			value, err = delezieScalarOperand(row.Cells[2], nil)
			if err == nil {
				ref.HasValue, ref.Value = true, value.Value
			}
		}
		if err != nil {
			return nil, fmt.Errorf("%s row %d %s: %w", tree.Table.Path, row.Index, row.Name, err)
		}
		ref.Row, ref.Kind = row.Index, row.Name
		out[row.Index] = ref
	}
	return out, nil
}
