package legion

import (
	"fmt"
	"time"
)

// Native1476E7DC0 reads both tables in five-integer groups. The schedule
// key is kept as source seconds; its four values reference gateflow keys.
// These declarations are NOT four player seats or raw dungeon IDs.
type GateScheduleRow struct {
	Seconds  int64
	FlowKeys [4]byte
}
type GateFlowRow struct {
	Key     byte
	Targets [4]int8 // -1 padding is preserved, never cast to an unsigned ID
}
type GateRules struct {
	Schedule []GateScheduleRow
	Flows    []GateFlowRow
}

func CompileGateRules(schedule, flow []int64) (GateRules, error) {
	var rules GateRules
	if len(schedule) == 0 || len(schedule)%5 != 0 || len(schedule) > 20 || len(flow) == 0 || len(flow)%5 != 0 {
		return rules, fmt.Errorf("apocalypse gate tables have invalid row widths/counts")
	}
	keys := map[byte]bool{}
	for i := 0; i < len(flow); i += 5 {
		key := flow[i]
		if key < 1 || key > 5 || keys[byte(key)] {
			return GateRules{}, fmt.Errorf("apocalypse gateflow key is invalid or repeated")
		}
		row := GateFlowRow{Key: byte(key)}
		for j := 0; j < 4; j++ {
			v := flow[i+1+j]
			if v < -1 || v > 5 {
				return GateRules{}, fmt.Errorf("apocalypse gateflow target outside source phase table")
			}
			row.Targets[j] = int8(v)
		}
		keys[row.Key] = true
		rules.Flows = append(rules.Flows, row)
	}
	seconds := map[int64]bool{}
	for i := 0; i < len(schedule); i += 5 {
		if schedule[i] <= 0 || seconds[schedule[i]] {
			return GateRules{}, fmt.Errorf("apocalypse gate schedule key is invalid or repeated")
		}
		row := GateScheduleRow{Seconds: schedule[i]}
		seconds[row.Seconds] = true
		for j := 0; j < 4; j++ {
			v := schedule[i+1+j]
			if v < 0 || v > 5 || v != 0 && !keys[byte(v)] {
				return GateRules{}, fmt.Errorf("apocalypse schedule references absent flow")
			}
			row.FlowKeys[j] = byte(v)
		}
		rules.Schedule = append(rules.Schedule, row)
	}
	return rules, nil
}

func (r GateRules) Flow(key byte) (GateFlowRow, bool) {
	for _, row := range r.Flows {
		if row.Key == key {
			return row, true
		}
	}
	return GateFlowRow{}, false
}

// Gate board projection from a server-measured first-boss elapsed time. The
// source keys are remaining seconds: maxKey is the initial board; crossing a
// smaller key selects that board. Do not round elapsed down (45.001s must not
// reopen the45s shortcut), and never consume a client-reported clear time.
// This pure projection does not authorize a portal or fabricate a death.
func (r GateRules) AtElapsed(elapsed time.Duration) ([4]byte, error) {
	var empty [4]byte
	if elapsed < 0 || len(r.Schedule) == 0 {
		return empty, fmt.Errorf("invalid owned gate elapsed time")
	}
	initial := int64(0)
	for _, row := range r.Schedule {
		if row.Seconds > initial {
			initial = row.Seconds
		}
	}
	if initial <= 0 || initial > 86400 {
		return empty, fmt.Errorf("gate schedule clock outside supported range")
	}
	left := time.Duration(initial)*time.Second - elapsed
	if left <= 0 {
		return empty, nil
	}
	var best *GateScheduleRow
	for i := range r.Schedule {
		row := &r.Schedule[i]
		if time.Duration(row.Seconds)*time.Second >= left && (best == nil || row.Seconds < best.Seconds) {
			best = row
		}
	}
	if best == nil {
		return empty, fmt.Errorf("no source gate board for remaining time")
	}
	return best.FlowKeys, nil
}
