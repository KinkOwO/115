package raid

import (
	"dfolan/internal/catalog"
	"fmt"
)

type DelezieEventGap struct {
	TriggerRow int
	Reason     string
}

type DelezieEventCoverage struct {
	SourcePath                   string
	TotalEvents, SupportedEvents int
	Unresolved                   []DelezieEventGap
}

// InspectDelezieEventCoverage performs no initialization or mutation. It checks
// every source event against the existing complete-event executor, instead
// of treating parsed operands as implemented battle actions. Even complete
// event coverage would not establish the initial notification wire contract.
func InspectDelezieEventCoverage(rules *catalog.DelezieRaidRules, mode *catalog.DelezieModeRules) (*DelezieEventCoverage, error) {
	script, err := NewDelezieScript(rules, mode, nil)
	if err != nil {
		return nil, err
	}
	out := &DelezieEventCoverage{SourcePath: mode.Phase.Table.Path}
	for _, row := range script.scalars.eventRows {
		out.TotalEvents++
		if err := script.ValidateEvent(row); err != nil {
			out.Unresolved = append(out.Unresolved, DelezieEventGap{TriggerRow: row, Reason: err.Error()})
		} else {
			out.SupportedEvents++
		}
	}
	if out.TotalEvents == 0 {
		return nil, fmt.Errorf("%s: source phase events missing", out.SourcePath)
	}
	return out, nil
}
