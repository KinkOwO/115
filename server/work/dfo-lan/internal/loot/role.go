package loot

import "encoding/json"

// Role is the character projection consumed by loot state transitions.
type Role struct {
	AccountID, ID int64
	WireID        uint16
	Profession    byte
	ConfigVersion string
	State         json.RawMessage
}
