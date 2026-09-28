package legion

import "fmt"

// Source 2f_to_3f_manager creates right/left/centre/bottom warp objects.
// Their formula move_to_dungeon addresses target indices2/3/4/5; the source
// shortcut tests use slots0/1/2/3. These are not compass-enum values on wire.
type PhysicalDoor struct {
	Slot, Target byte
	Object       uint32
	Name         string
}

func ApocalypsePhysicalDoors() [4]PhysicalDoor {
	return [4]PhysicalDoor{{0, 2, 109133355, "right"}, {1, 3, 109133354, "left"}, {2, 4, 109133353, "centre"}, {3, 5, 109133356, "bottom"}}
}

// Resolve the destination the player actually walked into. Never choose a
// route on their behalf. Cross-check CTP flow against the actual warp object.
func (r GateRules) PhysicalDoorPath(board [4]byte, target byte) ([]byte, error) {
	if target < 2 || target > 5 {
		return nil, fmt.Errorf("not an apocalypse physical branch door")
	}
	slot := target - 2
	key := board[slot]
	if key == 0 {
		return nil, fmt.Errorf("apocalypse physical door is closed")
	}
	flow, ok := r.Flow(key)
	if !ok {
		return nil, fmt.Errorf("physical door references missing source route")
	}
	var route []byte
	ended := false
	for _, v := range flow.Targets {
		if v < 0 {
			ended = true
			continue
		}
		if ended || v < 2 || v > 5 {
			return nil, fmt.Errorf("invalid physical door route")
		}
		route = append(route, byte(v))
	}
	if len(route) == 0 || route[0] != target {
		return nil, fmt.Errorf("CTP route and physical warp destination disagree")
	}
	return route, nil
}
