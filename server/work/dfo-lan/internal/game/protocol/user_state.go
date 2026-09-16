package protocol

import "fmt"

// NOTI3 updates CharacterInfo+0x2c. Keeping the town value0 inside a
// dungeon makes native144d34be9 refuse room movement before sending CMD45.
const (
	UserStateTown    byte = 0
	UserStateDungeon byte = 1
)

func UserState(actor uint16, state byte) ([]byte, error) {
	if actor == 0 || actor == 65535 || state > UserStateDungeon {
		return nil, fmt.Errorf("unsupported owned actor state")
	}
	return append(add16([]byte{1}, actor), state), nil
}
