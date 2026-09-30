package inventory

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

const (
	ShieldActiveSpace byte = 31
	ShieldGridSpace   byte = 32
)

type KnightShieldRow struct {
	Position      uint16 `json:"position"`
	Item          uint32 `json:"item"`
	EquSHA256     string `json:"equ_sha256"`
	GrowType      uint32 `json:"grow_type"`
	Condition     string `json:"condition"`
	RequiredLevel byte   `json:"required_level,omitempty"`
	OpenQuest     uint32 `json:"open_quest,omitempty"`
	ClearQuest    uint32 `json:"clear_quest,omitempty"`
	WearSlot      uint16 `json:"wear_slot"`
	SubType       int32  `json:"sub_type"`
}

type KnightShields struct {
	Source       pvf.ArchiveSnapshot `json:"source"`
	Chain        string              `json:"chain"`
	WindowSHA256 string              `json:"window_sha256"`
	WearSource   string              `json:"wear_source"`
	Profession   byte                `json:"profession"`
	Rows         []KnightShieldRow   `json:"rows"`
	index        map[uint32]KnightShieldRow
}

func LoadKnightShields(path, source string) (*KnightShields, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c KnightShields
	if err = json.Unmarshal(raw, &c); err != nil {
		return nil, err
	}
	if err = c.Validate(source); err != nil {
		return nil, err
	}
	return &c, nil
}

func (c *KnightShields) Validate(source string) error {
	if c == nil || len(source) != 64 || c.Source.Checksum != source || c.WearSource != source || len(c.WindowSHA256) != 64 || c.Chain != "etc/character/knight/shieldwindownewdata.etc" || len(c.Rows) != 25 {
		return fmt.Errorf("knight shield catalog provenance/row count mismatch")
	}
	c.index = make(map[uint32]KnightShieldRow, len(c.Rows))
	for i, r := range c.Rows {
		if r.Position != uint16(i+1) || r.Item == 0 || len(r.EquSHA256) != 64 || r.WearSlot != 24 || c.index[r.Item].Item != 0 {
			return fmt.Errorf("invalid knight shield row %d", i+1)
		}
		switch r.Condition {
		case "level":
			if r.RequiredLevel == 0 {
				return fmt.Errorf("shield %d missing level", r.Item)
			}
		case "quest":
			if r.OpenQuest == 0 || r.ClearQuest == 0 {
				return fmt.Errorf("shield %d missing quest", r.Item)
			}
		default:
			return fmt.Errorf("shield %d unknown condition %q", r.Item, r.Condition)
		}
		c.index[r.Item] = r
	}
	return nil
}

func (c *KnightShields) Slot() uint16 { return 24 }
func (c *KnightShields) Offers(item uint32) bool {
	if c == nil {
		return false
	}
	_, ok := c.index[item]
	return ok
}
func (c *KnightShields) Allowed(item uint32, level byte) (bool, string) {
	if c == nil {
		return false, "knight shield catalog unavailable"
	}
	r, ok := c.index[item]
	if !ok {
		return false, "shield not offered by window"
	}
	if r.Condition == "quest" {
		return false, "shield quest gate not verified"
	}
	if level < r.RequiredLevel {
		return false, "shield level gate not met"
	}
	return true, ""
}

// Code 5 is outside CMD19's local text-selection cases. Keep ordinary
// refusals at their existing code; shield UI refusals must not claim bag-full.
type MoveRefusal struct {
	Code uint16
	Msg  string
}

func (e *MoveRefusal) Error() string { return e.Msg }
func shieldRefusal(msg string) error { return &MoveRefusal{Code: 5, Msg: msg} }
func MoveRefusalCode(err error) uint16 {
	var refusal *MoveRefusal
	if errors.As(err, &refusal) {
		return refusal.Code
	}
	return 4
}
func IsKnightShieldMove(r protocol.ItemMoveRequest) bool {
	return r.SourceList == ShieldActiveSpace || r.SourceList == ShieldGridSpace || r.DestinationList == ShieldActiveSpace || r.DestinationList == ShieldGridSpace
}
