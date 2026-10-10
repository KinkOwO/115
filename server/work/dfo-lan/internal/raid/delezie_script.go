package raid

import (
	"dfolan/internal/catalog"
	"fmt"
)

// DelezieScript executes complete source events in the supported scalar/raw
// timer subset. Its owner explicitly advances time and selects events; native
// cascades, mixed predicate grouping and physical time conversion are not guessed.
type DelezieScript struct {
	scalars *DelezieScalars
	timers  *DelezieTimers
}

type DelezieScriptChanges struct {
	Scalars []DelezieScalarChange
	Timers  []DelezieTimerWindow
}

func NewDelezieScript(rules *catalog.DelezieRaidRules, mode *catalog.DelezieModeRules, initial map[string]int32) (*DelezieScript, error) {
	scalars, err := NewDelezieScalars(rules, mode, initial)
	if err != nil {
		return nil, err
	}
	if len(scalars.events) == 0 {
		return nil, fmt.Errorf("%s: source phase events missing", scalars.path)
	}
	timers, err := NewDelezieTimers(rules, mode)
	if err != nil {
		return nil, err
	}
	return &DelezieScript{scalars: scalars, timers: timers}, nil
}

func (s *DelezieScript) Values() map[string]int32 { return s.scalars.Values() }
func (s *DelezieScript) Timers() map[catalog.DelezieTimerKey]DelezieTimerWindow {
	return s.timers.Snapshot()
}
func (s *DelezieScript) NowRaw() uint64 { return s.timers.NowRaw() }
func (s *DelezieScript) AdvanceRaw(now uint64) ([]DelezieTimerWindow, error) {
	return s.timers.AdvanceRaw(now)
}

// ValidateEvent checks completeness before predicates or mutations. A false
// predicate must not make an unknown action look like a supported event.
func (s *DelezieScript) ValidateEvent(triggerRow int) error {
	plan, ok := s.scalars.events[triggerRow]
	if !ok || len(plan.trigger) != 1 || len(plan.behavior) == 0 {
		return fmt.Errorf("%s row %d: missing event or unresolved predicate grouping", s.scalars.path, triggerRow)
	}
	trigger := plan.trigger[0]
	if trigger.hasChildren {
		return fmt.Errorf("%s row %d: nested event predicates unresolved", s.scalars.path, trigger.index)
	}
	switch trigger.kind {
	case "[IF]":
		if _, err := s.scalars.expression(trigger.index, "[IF]"); err != nil {
			return err
		}
	case "[CHECK TIMER END]":
		ref, ok := s.timers.program.Phase[trigger.index]
		if !ok || ref.Kind != "[CHECK TIMER END]" || ref.HasValue {
			return fmt.Errorf("%s row %d: missing compiled timer predicate", s.scalars.path, trigger.index)
		}
	default:
		return fmt.Errorf("%s row %d: unsupported event predicate %s", s.scalars.path, trigger.index, trigger.kind)
	}
	for _, row := range plan.behavior {
		if row.hasChildren {
			return fmt.Errorf("%s row %d: nested event action unresolved", s.scalars.path, row.index)
		}
		switch row.kind {
		case "[SET]":
			if _, err := s.scalars.expression(row.index, "[SET]"); err != nil {
				return err
			}
		case "[SET TIMER]":
			ref, ok := s.timers.program.Phase[row.index]
			if !ok || ref.Kind != "[SET TIMER]" || !ref.HasValue || ref.Value <= 0 {
				return fmt.Errorf("%s row %d: unresolved timer action/value", s.scalars.path, row.index)
			}
		default:
			return fmt.Errorf("%s row %d: unsupported complete-event action %s", s.scalars.path, row.index, row.kind)
		}
	}
	return nil
}

// ExecuteEvent preserves source action order and commits scalar changes and
// timer windows together or neither. No subset of an unsupported event runs.
func (s *DelezieScript) ExecuteEvent(triggerRow int) (bool, *DelezieScriptChanges, error) {
	if err := s.ValidateEvent(triggerRow); err != nil {
		return false, nil, err
	}
	plan := s.scalars.events[triggerRow]
	trigger := plan.trigger[0]
	var matched bool
	var err error
	if trigger.kind == "[IF]" {
		matched, err = s.scalars.Check(trigger.index)
	} else {
		matched, err = s.timers.CheckPhase(trigger.index)
	}
	if err != nil || !matched {
		return false, nil, err
	}
	stagedScalars, stagedTimers := *s.scalars, *s.timers
	stagedScalars.values, stagedTimers.windows = s.scalars.Values(), s.timers.Snapshot()
	changes := &DelezieScriptChanges{}
	for _, row := range plan.behavior {
		if row.kind == "[SET]" {
			part, err := stagedScalars.ApplyAssignments([]int{row.index})
			if err != nil {
				return false, nil, err
			}
			changes.Scalars = append(changes.Scalars, part...)
		} else {
			part, err := stagedTimers.ArmPhase([]int{row.index})
			if err != nil {
				return false, nil, err
			}
			changes.Timers = append(changes.Timers, part...)
		}
	}
	s.scalars.values, s.timers.windows = stagedScalars.values, stagedTimers.windows
	return true, changes, nil
}
