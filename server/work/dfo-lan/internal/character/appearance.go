package character

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

// Decode only the inventory projection needed by character packets. Inventory
// owns validation/mutation; importing that package here would create a cycle.
func wornAppearance(raw json.RawMessage) ([]protocol.Equipment, error) {
	var state struct {
		Inventory struct {
			Worn []struct {
				Slot     uint16 `json:"slot"`
				Template uint32 `json:"template"`
			} `json:"worn"`
		} `json:"inventory"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	var rows []protocol.Equipment
	for _, item := range state.Inventory.Worn {
		if item.Slot >= 48 {
			return nil, fmt.Errorf("invalid worn appearance slot")
		}
		rows = append(rows, protocol.Equipment{Slot: byte(item.Slot), Item: item.Template})
	}
	return rows, nil
}
