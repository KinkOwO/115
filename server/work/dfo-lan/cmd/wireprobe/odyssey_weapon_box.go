package main

import (
	"dfolan/internal/savecontract"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

type odysseyWeaponChoices catalog.OdysseyWeaponChoices

func loadOdysseyWeaponChoices(path string) (odysseyWeaponChoices, error) {
	c, err := catalog.LoadOdysseyWeaponChoices(path)
	return odysseyWeaponChoices(c), err
}
func (c odysseyWeaponChoices) allows(r protocol.WeaponBoxSelection) bool {
	if c.Template != 10417789 {
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
	// 与 applyOdysseyArmor 同一口径：不写「X.Source.SaveIdentity() != savecontract.Identity()」
	// 那种恒假子句（SaveIdentity() 是常量）。目录来源的 L3 校验要在别处比 `.Source.Checksum`。
	if !isOdysseyRewardRole(role) || role.ConfigVersion != savecontract.Identity() || !choices.allows(r) || wear == nil || wear.Catalog == nil {
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
