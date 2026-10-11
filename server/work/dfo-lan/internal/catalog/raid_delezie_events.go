package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// EventProgram rebinds phase events to the current owned CTP table. Phases is
// an import-time index, not a second authority for executable event membership.
// Rebuilding the tree also checks current parent/ref relations before execution.
func (r *DelezieRaidRules) EventProgram(mode *DelezieModeRules) ([]DelezieScriptEvent, error) {
	if r == nil || mode == nil || mode.Difficulty.Name() == "" || r.Modes[mode.Difficulty] != mode {
		return nil, fmt.Errorf("Delezie event mode does not belong to source rules")
	}
	if mode.Phase == nil || mode.Phase.Table == nil {
		return nil, fmt.Errorf("Delezie event source missing")
	}
	tree, err := pvf.NewCTPTree(mode.Phase.Table)
	if err != nil {
		return nil, err
	}
	var out []DelezieScriptEvent
	for _, phase := range tree.Children(-1, "[PHASE]") {
		events, err := parseDeleziePhaseEvents(tree, phase.Index)
		if err != nil {
			return nil, err
		}
		out = append(out, events...)
	}
	return out, nil
}

func parseDeleziePhaseEvents(tree *pvf.CTPTree, phaseRow int) ([]DelezieScriptEvent, error) {
	var out []DelezieScriptEvent
	children := tree.Children(phaseRow, "")
	for i := 0; i < len(children); i++ {
		row := children[i]
		if row.Name == "[BEHAVIOR]" {
			return nil, fmt.Errorf("%s row %d: orphan behavior", tree.Table.Path, row.Index)
		}
		if row.Name != "[TRIGGER]" {
			continue
		}
		if i+1 >= len(children) || children[i+1].Name != "[BEHAVIOR]" {
			return nil, fmt.Errorf("%s row %d: trigger without behavior", tree.Table.Path, row.Index)
		}
		behavior := children[i+1]
		event := DelezieScriptEvent{TriggerRow: row.Index, BehaviorRow: behavior.Index, Trigger: tree.Children(row.Index, ""), Behavior: tree.Children(behavior.Index, "")}
		if len(event.Trigger) == 0 || len(event.Behavior) == 0 {
			return nil, fmt.Errorf("%s row %d: empty event", tree.Table.Path, row.Index)
		}
		out = append(out, event)
		i++
	}
	return out, nil
}
