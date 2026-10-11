package legion

import (
	"dfolan/internal/catalog"
	"fmt"
)

// Resolve the destination the player actually walked into. Never choose a
// route on their behalf. Cross-check CTP flow against the actual warp object.
func (r GateRules) PhysicalDoorPath(board [4]byte, door catalog.ApocalypseDoor) ([]byte, error) {
	target := door.Target
	if int(door.Slot) >= len(board) || target >= 6 || door.Object == 0 || door.Dungeon == 0 {
		return nil, fmt.Errorf("not an apocalypse physical branch door")
	}
	key := board[door.Slot]
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
		if ended || v > 5 {
			return nil, fmt.Errorf("invalid physical door route")
		}
		route = append(route, byte(v))
	}
	if len(route) == 0 || route[0] != target {
		return nil, fmt.Errorf("CTP route and physical warp destination disagree")
	}
	return route, nil
}
