package protocol

import "fmt"

// AreaUser is the nine-byte row consumed by current NOTI 24, 0x1452fcfc4.
// The final three bytes remain explicit experimental values; their complete
// gameplay meanings are not yet recovered.
type AreaUser struct {
	ActorServerID uint16
	X, Y          uint16
	Flags         [3]byte
}

func AreaUsers(townID, areaID uint32, users []AreaUser) ([]byte, error) {
	if townID > 65535 || areaID > 65535 || len(users) > 65535 {
		return nil, fmt.Errorf("area-users identity overflow")
	}
	p := add16(add32(add32(nil, townID), areaID), uint16(len(users)))
	seen := map[uint16]bool{}
	for _, u := range users {
		if u.ActorServerID == 0 || u.ActorServerID == 65535 || seen[u.ActorServerID] {
			return nil, fmt.Errorf("invalid or duplicate area actor")
		}
		seen[u.ActorServerID] = true
		p = append(add16(add16(add16(p, u.ActorServerID), u.X), u.Y), u.Flags[:]...)
	}
	return p, nil
}
