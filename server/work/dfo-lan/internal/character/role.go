package character

import (
	"encoding/json"
	"time"
)

type Character struct {
	ID            int64
	AccountID     int64
	WireID        uint16
	FixedSlot     byte
	Name          string
	Profession    byte
	Request       []byte
	ConfigVersion string
	State         json.RawMessage
	CreatedAt     time.Time
}

type CharacterSlotChange struct {
	Swap, Before       bool
	FromFixed, ToFixed bool
	From, To           uint32
}

type FatigueState struct {
	Day     string `json:"day"`
	Used    uint16 `json:"used"`
	Limit   uint16 `json:"limit"`
	UsedMax uint16 `json:"used_max"`
}

type FatigueRecovery struct {
	Day           string
	Limit, Amount uint16
	Template      uint32
	DailyUses     uint32
	Cooldown      time.Duration
	Now           time.Time
}
