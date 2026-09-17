package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

const odysseySource = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
const odysseyArmorEvent = "odyssey-create-10417791-armor-10417790-v1"
const odysseyWeaponBoxEvent = "odyssey-create-10417791-weapon-box-10417789-v1"

// Source create reward10417791 contains armor box10417790. Its single
// selection-num0 category awards these eight pieces, not one random item.
// Keep weapon10417789 and potion10418028 settlement separate until supported.
var odysseyArmor = [...]uint32{100051399, 100101277, 100151218, 100201190, 100251230, 100302054, 100313767, 100323647}

func isOdysseyRewardRole(role storage.Character) bool {
	r, e := protocol.DecodeCreateRequest(role.Request)
	return e == nil && len(r.Options) == 12 && r.Options[10] == 2
}

func applyOdysseyArmor(role storage.Character, wear *inventory.WearService) (json.RawMessage, json.RawMessage, error) {
	if !isOdysseyRewardRole(role) || role.ConfigVersion != odysseySource || wear == nil || wear.Catalog == nil || wear.Catalog.Source.Checksum != odysseySource || wear.BagRules.Source != odysseySource {
		return nil, nil, fmt.Errorf("Odyssey armor requires matching character and source catalogs")
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, nil, e
	}
	for _, id := range odysseyArmor {
		b, _, e = b.AddEquipment(wear.Catalog, wear.BagRules.EquipmentSlots, id, 1)
		if e != nil {
			return nil, nil, e
		}
	}
	raw, e := inventory.SaveBag(role.State, b)
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(map[string]any{"create_reward": 10417791, "armor_box": 10417790, "templates": odysseyArmor, "before": role.State, "after": raw, "weapon_settled": false, "potion_settled": false})
	return raw, receipt, e
}

func grantOdysseyArmor(ctx context.Context, store *storage.Store, wear *inventory.WearService, role storage.Character) (storage.Character, bool, error) {
	if !isOdysseyRewardRole(role) {
		return role, false, nil
	}
	return store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, odysseyArmorEvent, "odyssey-source-armor-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		return applyOdysseyArmor(current, wear)
	})
}

func applyOdysseyWeaponBox(role storage.Character) (json.RawMessage, json.RawMessage, error) {
	if !isOdysseyRewardRole(role) || role.ConfigVersion != odysseySource {
		return nil, nil, fmt.Errorf("Odyssey weapon box requires source mode")
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, nil, e
	}
	occupied := map[uint16]bool{}
	for _, v := range b.Items {
		occupied[v.Slot] = true
	}
	for _, v := range b.Equipment {
		occupied[v.Slot] = true
	}
	var slot uint16
	for n := uint16(65); n <= 120; n++ {
		if !occupied[n] {
			slot = n
			break
		}
	}
	if slot == 0 {
		return nil, nil, fmt.Errorf("consumable bag full; weapon box remains owed")
	}
	b.Items = append(b.Items, inventory.BagItem{Slot: slot, Template: 10417789, Amount: 1})
	raw, e := inventory.SaveBag(role.State, b)
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(map[string]any{"create_reward": 10417791, "template": 10417789, "quantity": 1, "slot": slot, "before": role.State, "after": raw, "selection_settled": false})
	return raw, receipt, e
}

func grantOdysseyWeaponBox(ctx context.Context, store *storage.Store, role storage.Character) (storage.Character, bool, error) {
	if !isOdysseyRewardRole(role) {
		return role, false, nil
	}
	return store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, odysseyWeaponBoxEvent, "odyssey-source-weapon-box-v1", applyOdysseyWeaponBox)
}
