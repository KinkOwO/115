package raid

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
)

// DelezieScalars executes source-bound IF/SET operations. The future raid
// owner must provide authoritative initial values and serialize its calls.
// This core does not initialize absent symbols to zero, schedule timers, emit
// packets, or skip unsupported actions to pretend an entire event succeeded.
type DelezieScalars struct {
	path      string
	symbols   map[string]uint32
	program   map[int]catalog.DelezieScalarExpression
	values    map[string]int32
	events    map[int]delezieScalarEvent
	eventRows []int
}

type delezieScalarRow struct {
	index       int
	kind        string
	hasChildren bool
}

type delezieScalarEvent struct {
	trigger, behavior []delezieScalarRow
}

type DelezieScalarChange struct {
	Row           int
	Symbol        string
	SymbolID      uint32
	Before, After int32
	PreviouslySet bool
}

func NewDelezieScalars(rules *catalog.DelezieRaidRules, mode *catalog.DelezieModeRules, initial map[string]int32) (*DelezieScalars, error) {
	program, err := rules.ScalarProgram(mode)
	if err != nil {
		return nil, err
	}
	events, err := rules.EventProgram(mode)
	if err != nil {
		return nil, err
	}
	s := &DelezieScalars{
		path: mode.Phase.Table.Path, symbols: make(map[string]uint32, len(rules.SymbolIDs)),
		program: program, values: make(map[string]int32, len(initial)), events: map[int]delezieScalarEvent{},
	}
	for name, id := range rules.SymbolIDs {
		s.symbols[name] = id
	}
	for name, value := range initial {
		if _, ok := s.symbols[name]; !ok {
			return nil, fmt.Errorf("%s: initial value for undeclared symbol %q", s.path, name)
		}
		s.values[name] = value
	}
	// Retain complete direct event rows, including unsupported actions. Never
	// project a mixed event to its scalar subset in this execution API.
	for _, event := range events {
		plan := delezieScalarEvent{}
		for _, part := range []struct {
			source []pvf.CTPRecord
			rows   *[]delezieScalarRow
		}{{event.Trigger, &plan.trigger}, {event.Behavior, &plan.behavior}} {
			for _, row := range part.source {
				nested := false
				for _, ref := range row.Refs {
					nested = nested || len(ref.Values) != 0
				}
				*part.rows = append(*part.rows, delezieScalarRow{row.Index, row.Name, nested})
			}
		}
		s.events[event.TriggerRow] = plan
		s.eventRows = append(s.eventRows, event.TriggerRow)
	}
	return s, nil
}

func (s *DelezieScalars) Values() map[string]int32 {
	out := make(map[string]int32, len(s.values))
	for name, value := range s.values {
		out[name] = value
	}
	return out
}

func (s *DelezieScalars) expression(row int, kind string) (catalog.DelezieScalarExpression, error) {
	x, ok := s.program[row]
	if !ok || x.Kind != kind {
		return x, fmt.Errorf("%s row %d: expected compiled %s operation", s.path, row, kind)
	}
	return x, nil
}

func (s *DelezieScalars) value(values map[string]int32, name string) (int32, error) {
	value, initialized := values[name]
	if !initialized {
		return 0, fmt.Errorf("symbol %q has no initialized value", name)
	}
	return value, nil
}

func (s *DelezieScalars) operand(values map[string]int32, x catalog.DelezieScalarExpression) (int32, error) {
	if x.Operand.IsSymbol {
		return s.value(values, x.Operand.Symbol)
	}
	return x.Operand.Value, nil
}

func (s *DelezieScalars) Check(row int) (bool, error) {
	x, err := s.expression(row, "[IF]")
	if err != nil {
		return false, err
	}
	left, err := s.value(s.values, x.Target)
	if err != nil {
		return false, s.failure(x, err)
	}
	right, err := s.operand(s.values, x)
	if err != nil {
		return false, s.failure(x, err)
	}
	switch x.Operator {
	case "[==]":
		return left == right, nil
	case "[!=]":
		return left != right, nil
	case "[>]":
		return left > right, nil
	case "[<]":
		return left < right, nil
	case "[>=]":
		return left >= right, nil
	case "[<=]":
		return left <= right, nil
	}
	return false, s.failure(x, fmt.Errorf("unsupported comparator"))
}

func (s *DelezieScalars) failure(x catalog.DelezieScalarExpression, err error) error {
	return fmt.Errorf("%s row %d %s %s: %w", s.path, x.Row, x.Kind, x.Operator, err)
}

// ApplyAssignments commits all rows in source order or none. Later operands
// observe earlier staged assignments, and errors never publish partial changes.
// Signed overflow is rejected; native overflow behavior has not been established.
func (s *DelezieScalars) ApplyAssignments(rows []int) ([]DelezieScalarChange, error) {
	staged := s.Values()
	var changes []DelezieScalarChange
	for _, row := range rows {
		x, err := s.expression(row, "[SET]")
		if err != nil {
			return nil, err
		}
		right, err := s.operand(staged, x)
		if err != nil {
			return nil, s.failure(x, err)
		}
		before, initialized := staged[x.Target]
		if x.Operator != "[=]" && !initialized {
			return nil, s.failure(x, fmt.Errorf("symbol %q has no initialized value", x.Target))
		}
		left, rhs := int64(before), int64(right)
		var result int64
		switch x.Operator {
		case "[=]":
			result = rhs
		case "[+]":
			result = left + rhs
		case "[-]":
			result = left - rhs
		case "[*]":
			result = left * rhs
		case "[/]", "[MOD]":
			if rhs == 0 || (left == math.MinInt32 && rhs == -1) {
				return nil, s.failure(x, fmt.Errorf("invalid signed division %d/%d", left, rhs))
			}
			if x.Operator == "[/]" {
				result = left / rhs
			} else {
				result = left % rhs
			}
		default:
			return nil, s.failure(x, fmt.Errorf("unsupported assignment"))
		}
		if result < math.MinInt32 || result > math.MaxInt32 {
			return nil, s.failure(x, fmt.Errorf("signed scalar overflow: %d", result))
		}
		staged[x.Target] = int32(result)
		changes = append(changes, DelezieScalarChange{x.Row, x.Target, x.TargetID, before, int32(result), initialized})
	}
	s.values = staged
	return changes, nil
}

// ExecuteScalarEvent accepts only complete events containing one direct IF
// and direct SET rows. Predicate grouping/cascades are not established yet.
// A mixed event (objects, timers, dungeon triggers, etc.) is rejected in
// full before checking predicates or changing values. No event cascade is run.
func (s *DelezieScalars) ExecuteScalarEvent(triggerRow int) (bool, []DelezieScalarChange, error) {
	plan, ok := s.events[triggerRow]
	if !ok || len(plan.trigger) == 0 || len(plan.behavior) == 0 {
		return false, nil, fmt.Errorf("%s row %d: missing/empty scalar event", s.path, triggerRow)
	}
	if len(plan.trigger) != 1 {
		return false, nil, fmt.Errorf("%s row %d: scalar predicate grouping is not supported (%d triggers)", s.path, triggerRow, len(plan.trigger))
	}
	for _, part := range []struct {
		rows []delezieScalarRow
		kind string
	}{{plan.trigger, "[IF]"}, {plan.behavior, "[SET]"}} {
		for _, row := range part.rows {
			if row.kind != part.kind || row.hasChildren {
				return false, nil, fmt.Errorf("%s row %d: unsupported scalar event action %s (nested=%t)", s.path, row.index, row.kind, row.hasChildren)
			}
		}
	}
	for _, row := range plan.trigger {
		matched, err := s.Check(row.index)
		if err != nil || !matched {
			return false, nil, err
		}
	}
	rows := make([]int, len(plan.behavior))
	for i, row := range plan.behavior {
		rows[i] = row.index
	}
	changes, err := s.ApplyAssignments(rows)
	return err == nil, changes, err
}
