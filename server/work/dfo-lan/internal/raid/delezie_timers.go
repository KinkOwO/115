package raid

import (
	"dfolan/internal/catalog"
	"fmt"
	"math"
	"sort"
)

// DelezieTimers is a serialized-owner countdown core in explicit raw source
// units. It does not convert these units to seconds, start a wall clock, dispatch
// mixed events or emit notifications. The native time conversion is still open.
type DelezieTimers struct {
	program *catalog.DelezieTimerProgram
	now     uint64
	windows map[catalog.DelezieTimerKey]DelezieTimerWindow
}

type DelezieTimerWindow struct {
	Key               catalog.DelezieTimerKey
	Path              string
	Row               int
	Value             int32
	Started, Deadline uint64
	Ended             bool
}

func NewDelezieTimers(rules *catalog.DelezieRaidRules, mode *catalog.DelezieModeRules) (*DelezieTimers, error) {
	program, err := rules.TimerProgram(mode)
	if err != nil {
		return nil, err
	}
	return &DelezieTimers{program: program, windows: map[catalog.DelezieTimerKey]DelezieTimerWindow{}}, nil
}

func (s *DelezieTimers) Snapshot() map[catalog.DelezieTimerKey]DelezieTimerWindow {
	out := make(map[catalog.DelezieTimerKey]DelezieTimerWindow, len(s.windows))
	for key, value := range s.windows {
		out[key] = value
	}
	return out
}

func (s *DelezieTimers) NowRaw() uint64 { return s.now }

// ArmPhase is an explicit owner decision, not automatic evaluation
// of source fields or triggers. Nonpositive source values and RESET remain
// unsupported; they must not silently become delete/pause/immediate expiry.
func (s *DelezieTimers) ArmPhase(rows []int) ([]DelezieTimerWindow, error) {
	return s.arm(s.program.PhasePath, s.program.Phase, rows)
}

func (s *DelezieTimers) arm(path string, program map[int]catalog.DelezieTimerReference, rows []int) ([]DelezieTimerWindow, error) {
	staged := s.Snapshot()
	var changes []DelezieTimerWindow
	for _, row := range rows {
		ref, ok := program[row]
		if !ok || !ref.HasValue || ref.Value <= 0 {
			return nil, fmt.Errorf("%s row %d: unresolved timer operation/value", path, row)
		}
		if uint64(ref.Value) > math.MaxUint64-s.now {
			return nil, fmt.Errorf("%s row %d: timer deadline overflow", path, row)
		}
		window := DelezieTimerWindow{Key: ref.Key, Path: path, Row: row, Value: ref.Value, Started: s.now, Deadline: s.now + uint64(ref.Value)}
		staged[ref.Key] = window
		changes = append(changes, window)
	}
	s.windows = staged
	return changes, nil
}

// AdvanceRaw reports each expired window once. Sorting is only a stable
// observation order; it does not establish native mixed-event priority.
func (s *DelezieTimers) AdvanceRaw(now uint64) ([]DelezieTimerWindow, error) {
	if now < s.now {
		return nil, fmt.Errorf("Delezie raw timer time moved backwards: %d -> %d", s.now, now)
	}
	var ended []DelezieTimerWindow
	for key, window := range s.windows {
		if !window.Ended && window.Deadline <= now {
			window.Ended = true
			s.windows[key] = window
			ended = append(ended, window)
		}
	}
	sort.Slice(ended, func(i, j int) bool {
		if ended[i].Deadline != ended[j].Deadline {
			return ended[i].Deadline < ended[j].Deadline
		}
		if ended[i].Key.Main != ended[j].Key.Main {
			return ended[i].Key.Main < ended[j].Key.Main
		}
		return ended[i].Key.Sub < ended[j].Key.Sub
	})
	s.now = now
	return ended, nil
}

func (s *DelezieTimers) CheckPhase(row int) (bool, error) {
	ref, ok := s.program.Phase[row]
	if !ok || ref.Kind != "[CHECK TIMER END]" || ref.HasValue {
		return false, fmt.Errorf("%s row %d: missing source timer-end predicate", s.program.PhasePath, row)
	}
	window, armed := s.windows[ref.Key]
	if !armed {
		return false, fmt.Errorf("%s row %d: timer has no authoritative armed state", s.program.PhasePath, row)
	}
	return window.Ended, nil
}

// CancelResolved invalidates explicitly chosen owner windows. It is not an
// implementation of native RESET TIMER, whose restore/reset semantics are open.
func (s *DelezieTimers) CancelResolved(keys []catalog.DelezieTimerKey) error {
	seen := map[catalog.DelezieTimerKey]bool{}
	for _, key := range keys {
		if _, ok := s.windows[key]; !ok || seen[key] {
			return fmt.Errorf("Delezie timer cancellation references missing/duplicate %d/%d", key.Main, key.Sub)
		}
		seen[key] = true
	}
	for _, key := range keys {
		delete(s.windows, key)
	}
	return nil
}
