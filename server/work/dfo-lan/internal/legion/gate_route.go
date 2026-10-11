package legion

import "fmt"

// StableMainRoute is the complete route of the primary physical gate. Unlike
// a shortcut, that gate must reference the same flow in every source schedule.
// First is the already delivered initial battle, not the flow's lookup key.
func (r GateRules) StableMainRoute(first byte) ([]byte, bool, error) {
	if first == 0 || first > 5 || len(r.Schedule) == 0 {
		return nil, false, nil
	}
	key := r.Schedule[0].FlowKeys[0]
	if key == 0 {
		return nil, false, nil
	}
	for _, row := range r.Schedule {
		if row.FlowKeys[0] != key {
			return nil, false, nil
		}
	}
	flow, ok := r.Flow(key)
	if !ok {
		return nil, false, fmt.Errorf("primary source gate references missing flow")
	}
	path := []byte{first}
	seen, padded := map[byte]bool{first: true}, false
	for _, target := range flow.Targets {
		if target == -1 {
			padded = true
			continue
		}
		if target <= 0 || target > 5 || padded || seen[byte(target)] {
			return nil, false, fmt.Errorf("ambiguous primary source route")
		}
		seen[byte(target)] = true
		path = append(path, byte(target))
	}
	return path, len(path) > 1, nil
}

// LinearRoute resolves a source board with one fixed flow. A branching or
// changing board needs its actual owned gate selection and elapsed clock;
// the common phase-duration table is not that selection.
func (r GateRules) LinearRoute() ([]byte, bool, error) {
	if len(r.Flows) != 1 {
		return nil, false, nil
	}
	flow := r.Flows[0]
	referenced := false
	for _, row := range r.Schedule {
		for _, key := range row.FlowKeys {
			if key != 0 && key != flow.Key {
				return nil, false, nil
			}
			if key == flow.Key {
				referenced = true
			}
		}
	}
	if len(r.Schedule) == 0 || !referenced {
		return nil, false, fmt.Errorf("source route has no schedule")
	}
	out := []byte{flow.Key}
	seen := map[byte]bool{flow.Key: true}
	padding := false
	for _, target := range flow.Targets {
		if target == -1 {
			padding = true
			continue
		}
		if target <= 0 || padding || seen[byte(target)] {
			return nil, false, fmt.Errorf("source linear gateflow is ambiguous")
		}
		seen[byte(target)] = true
		out = append(out, byte(target))
	}
	return out, true, nil
}
