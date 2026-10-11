package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

// TransferCombined keeps the upstream equipment schema while applying the
// captured stack rules and replay-protected transaction to both item families.
func (s *VaultService) TransferCombined(role Role, state VaultState, r protocol.ItemMoveRequest) (json.RawMessage, json.RawMessage, error) {
	v, err := ReadExtendedVault(state)
	if err != nil {
		return nil, nil, err
	}
	b, err := ReadBag(role.State)
	if err != nil {
		return nil, nil, err
	}
	if state.ConfigVersion != s.Rules.SourceSHA256 || !s.Rules.allows(state.Slots) || r.Extra != 0 || r.Selection != 0xffffffff || r.Flags != [3]byte{} {
		return nil, nil, fmt.Errorf("vault source or request mismatch")
	}
	var equipment bool
	var from, to uint32
	for _, loc := range []struct {
		list   byte
		slot   uint16
		source bool
	}{{r.SourceList, r.SourceSlot, true}, {r.DestinationList, r.DestinationSlot, false}} {
		var template uint32
		switch loc.list {
		case 0:
			for _, item := range b.Items {
				if item.Slot == loc.slot {
					template = item.Template
				}
			}
			for _, item := range b.Equipment {
				if item.Slot == loc.slot {
					template = item.Template
					equipment = true
				}
			}
		case 2:
			if item := v.ItemAt(loc.slot); item != nil {
				template = item.Template
				equipment = equipment || item.IsEquip
			}
		default:
			return nil, nil, fmt.Errorf("unsupported vault container")
		}
		if loc.source {
			from = template
		} else {
			to = template
		}
	}
	if from != r.SourceItem || to != r.DestinationItem {
		return nil, nil, fmt.Errorf("stale vault identity")
	}
	if equipment {
		if r.Count > 1 {
			return nil, nil, fmt.Errorf("invalid equipment count")
		}
		if r.SourceList == 2 && r.DestinationList == 2 && from == 0 && to != 0 && r.Count == 0 {
			r.SourceSlot, r.DestinationSlot = r.DestinationSlot, r.SourceSlot
		}
		b, v, _, err = MoveVaultItem(b, v, s.BagRules, r)
		if err != nil {
			return nil, nil, err
		}
		bag, err := SaveBag(role.State, b)
		if err != nil {
			return nil, nil, err
		}
		items, err := SaveVault(v)
		if err != nil {
			return nil, nil, err
		}
		state.Items = items
		if _, err = VaultPayload(state); err != nil {
			return nil, nil, err
		}
		if _, err = protocol.InventoryRestore(b.Rows(), b.Expansion); err != nil {
			return nil, nil, err
		}
		return bag, items, nil
	}
	stacks := make([]BagItem, 0, len(v.Items))
	gear := make([]VaultItem, 0)
	for _, item := range v.Items {
		if item.IsEquip {
			gear = append(gear, item)
		} else {
			stacks = append(stacks, BagItem{Slot: item.Slot, Template: item.Template, Amount: item.Amount, ExpireTime: item.ExpireTime})
		}
	}
	state.Items, err = json.Marshal(stacks)
	if err != nil {
		return nil, nil, err
	}
	bag, items, err := s.TransferStacks(role, state, r)
	if err != nil {
		return nil, nil, err
	}
	state.Items = items
	merged, err := ReadExtendedVault(state)
	if err != nil {
		return nil, nil, err
	}
	merged.Items = append(merged.Items, gear...)
	items, err = SaveVault(merged)
	if err != nil {
		return nil, nil, err
	}
	state.Items = items
	if _, err = VaultPayload(state); err != nil {
		return nil, nil, err
	}
	return bag, items, nil
}
