package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

func (w *worldSession) useWeaponShape(slot uint16, template uint32, event func(map[string]any)) ([]outboundPacket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, skin, applied, e := w.characters.RegisterWeaponShape(ctx, w.role, slot, template)
	if e != nil {
		return nil, e
	}
	w.role = saved
	bag, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	row := bagRowOrEmpty(bag, slot)
	update, e := protocol.InventoryUpdate([][protocol.CurrentItemRecordSize]byte{row})
	if e != nil {
		return nil, e
	}
	page, e := skinCargoRestore(saved.State, w.characters.WeaponSkinUsableFor(saved))
	if e != nil {
		return nil, e
	}
	if page == nil {
		return nil, fmt.Errorf("registered weapon shape cargo is empty")
	}
	out := []outboundPacket{{"weapon_shape_item_spent", 0, 14, update}, {"skin_cargo_info", 0, 1545, page}}
	recent, e := skinRecentRestore(ctx, w.store, saved.AccountID, saved.State, w.skinCatalog, w.characters.WeaponSkinUsableFor(saved))
	if e != nil {
		event(map[string]any{"kind": "weapon_shape_recent_error", "reason": e.Error()})
		return out, nil
	}
	if recent != nil {
		out = append(out, outboundPacket{"skin_recent_add", 0, 1547, recent})
	}
	event(map[string]any{"kind": "weapon_shape_registered", "character_id": saved.ID, "template": template, "slot": slot, "weapon": skin, "applied": applied})
	return out, nil
}
