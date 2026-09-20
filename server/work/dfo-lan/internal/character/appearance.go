package character

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// Decode only the inventory projection needed by character packets.
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

func wornCreature(raw json.RawMessage) (uint32, string) {
	var state struct {
		Inventory struct {
			Worn []struct {
				Slot     uint16 `json:"slot"`
				Template uint32 `json:"template"`
			} `json:"worn"`
		} `json:"inventory"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return 0, ""
	}
	for _, item := range state.Inventory.Worn {
		if item.Slot == 26 && item.Template != 0 {
			name := inventory.CreatureDefaultNames[item.Template]
			if name == "" {
				name = "Creature"
			}
			return item.Template, name
		}
	}
	return 0, ""
}
