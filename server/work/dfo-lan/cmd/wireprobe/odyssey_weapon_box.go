package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
)

type odysseyWeaponChoices struct {
	Definition catalog.ScriptRecord `json:"definition"`
	Source     string               `json:"source"`
	Template   uint32               `json:"template"`
	Categories []struct {
		Category [2]byte  `json:"category"`
		Items    []uint32 `json:"items"`
	} `json:"categories"`
}

func loadOdysseyWeaponChoices(path string) (odysseyWeaponChoices, error) {
	var c odysseyWeaponChoices
	p, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(p, &c)
	if e == nil && (c.Source != odysseySource || c.Template != 10417789 || len(c.Categories) != 85 || c.Definition.SHA256 != "d67f5042a5e17ef30581e297f030561de39f92ad5e215d742ec067aeaad96bec") {
		e = fmt.Errorf("invalid Odyssey weapon source")
	}
	return c, e
}
func (c odysseyWeaponChoices) allows(r protocol.WeaponBoxSelection) bool {
	if c.Source != odysseySource || c.Template != 10417789 {
		return false
	}
	for _, cat := range c.Categories {
		if cat.Category == r.Category {
			for _, id := range cat.Items {
				if id == r.Template {
					return true
				}
			}
		}
	}
	return false
}

type odysseyWeaponReceipt struct {
	Request protocol.WeaponBoxSelection
	Granted inventory.BagEquipment
}

const odysseyWeaponChoiceEvent = "odyssey-create-weapon-choice-10417789-v1"

func applyOdysseyWeaponChoice(role storage.Character, wear *inventory.WearService, choices odysseyWeaponChoices, r protocol.WeaponBoxSelection) (json.RawMessage, json.RawMessage, error) {
	if !isOdysseyRewardRole(role) || role.ConfigVersion != odysseySource || !choices.allows(r) || wear == nil || wear.Catalog == nil || wear.Catalog.Source.Checksum != odysseySource || wear.BagRules.Source != odysseySource {
		return nil, nil, fmt.Errorf("selection not in source Odyssey category")
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, nil, e
	}
	index := -1
	for i, v := range b.Items {
		if v.Slot == r.Slot && v.Template == 10417789 && v.Amount == 1 {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, nil, fmt.Errorf("selected slot does not own the creation weapon box")
	}
	b, slots, e := b.AddEquipment(wear.Catalog, wear.BagRules.EquipmentSlots, r.Template, 1)
	if e != nil {
		return nil, nil, e
	}
	b.Items = append(b.Items[:index:index], b.Items[index+1:]...)
	var granted inventory.BagEquipment
	for _, v := range b.Equipment {
		if v.Slot == slots[0] {
			granted = v
		}
	}
	raw, e := inventory.SaveBag(role.State, b)
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(odysseyWeaponReceipt{r, granted})
	return raw, receipt, e
}

func selectOdysseyWeapon(ctx context.Context, store *storage.Store, wear *inventory.WearService, role storage.Character, choices odysseyWeaponChoices, r protocol.WeaponBoxSelection) (storage.Character, []outboundPacket, error) {
	saved, applied, e := store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, odysseyWeaponChoiceEvent, "odyssey-weapon-selection-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		return applyOdysseyWeaponChoice(current, wear, choices, r)
	})
	if e != nil {
		return role, nil, e
	}
	receipt, e := store.CharacterEventReceipt(ctx, role.AccountID, role.ID, odysseyWeaponChoiceEvent)
	if e != nil {
		return role, nil, e
	}
	var outcome odysseyWeaponReceipt
	if e = json.Unmarshal(receipt, &outcome); e != nil {
		return role, nil, e
	}
	if outcome.Request != r {
		return saved, nil, fmt.Errorf("creation weapon box already selected differently")
	}
	var plan []outboundPacket
	if applied {
		rows := [][protocol.CurrentItemRecordSize]byte{protocol.EmptyOrdinaryItem(r.Slot), inventory.EquipmentRow(outcome.Granted)}
		update, e := protocol.InventoryUpdate(rows)
		if e != nil {
			return saved, nil, e
		}
		plan = append(plan, outboundPacket{"odyssey_weapon_inventory_updated", 0, 14, update})
	}
	plan = append(plan, outboundPacket{"odyssey_weapon_selection_ack", 1, 160, protocol.WeaponBoxSuccess(r)})
	return saved, plan, nil
}
