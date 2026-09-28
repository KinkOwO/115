package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"fmt"
	"time"
)

// useAddSkinStorage answers CMD507 action 169, the frame the client sends when a
// player uses an `[action type] [add skin storage]` stackable such as a damage
// font. The item is spent with the same idempotent consume transaction the
// fatigue potion uses on this frame and the committed bag goes back as NOTI 14;
// the registered skin's family page then also goes back as NOTI1545, whole, for
// every family in skinFamilyTable plus the damage font. A family with no measured
// panel consumer (the 武器外观 tab, which the replication path owns) stays
// durable-only, so no frame is invented for it.
func (w *worldSession) useAddSkinStorage(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil || w.characters == nil {
		return nil, fmt.Errorf("skin registration before character selection")
	}
	slot, e := protocol.DecodeAddSkinStorageAction(p)
	if e != nil {
		return nil, e
	}
	if w.skinCatalog == nil {
		return nil, fmt.Errorf("skin storage catalog is not loaded")
	}
	bag, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	var template uint32
	for _, row := range bag.Items {
		if row.Slot == slot {
			template = row.Template
		}
	}
	if template == 0 {
		return nil, fmt.Errorf("slot %d holds no stackable", slot)
	}
	entry, ok := w.skinCatalog[template]
	if !ok {
		return nil, fmt.Errorf("template %d is not an [add skin storage] item", template)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, _, e := w.loot.Consume(ctx, w.role, protocol.UseStackableRequest{
		Slot: slot, Template: template,
	})
	if e != nil {
		return nil, e
	}
	w.role = saved
	// The spend committed before this insert, so a database failure here is
	// recovered by the next press: loot.Consume replays the original receipt
	// instead of spending a second unit and UnlockSkin lands then.
	record := map[string]any{"character_id": saved.ID, "template": template,
		"slot": slot, "skin_key": entry.SkinKey(), "remaining": receipt.Remaining,
		"damage_font": entry.IsDamageFont()}
	unlockErr := w.characters.Store.UnlockSkin(ctx, saved.AccountID, template, entry.SkinKey())
	if unlockErr != nil {
		record["kind"] = "skin_storage_unlock_failed"
		record["reason"] = unlockErr.Error()
	} else {
		record["kind"] = "skin_storage_registered"
	}
	event(record)
	// Absolute state for the spent slot, matching the live-verified fatigue
	// path on this same frame: an emptied stack is sent as an empty row.
	row := protocol.EmptyOrdinaryItem(slot)
	after, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	for _, item := range after.Items {
		if item.Slot == slot {
			row = protocol.OrdinaryItem(slot, item.Template, item.Amount, item.ExpireTime)
		}
	}
	update, e := protocol.InventoryUpdate([][protocol.CurrentItemRecordSize]byte{row})
	if e != nil {
		return nil, e
	}
	out := []outboundPacket{{"skin_storage_inventory", 0, 14, update}}
	// A page frame rebuilds the whole page, so the push carries every
	// damage-font skin the account holds, not only the one just registered.
	// Other families have no proven page, so their unlock stays durable only.
	if unlockErr == nil && entry.IsDamageFont() {
		cargo, e := damageFontCargo(ctx, w.characters.Store, saved.AccountID, w.skinCatalog)
		if e != nil {
			event(map[string]any{"kind": "skin_cargo_damage_font_error",
				"character_id": saved.ID, "reason": e.Error()})
		} else {
			out = append(out, outboundPacket{"skin_cargo_damage_font", 0, 1545, cargo})
		}
	}
	// Every family in skinFamilyTable (边框, 觉醒插图, 表情, 涂鸦, 飞空艇特效) works the
	// same way on its own page: the spend is answered with that family's whole page, so
	// the panel's grid sees the new row without the client having to ask. The 武器外观
	// tab is not in that table — its page belongs to the replication path — so its
	// unlock stays durable-only.
	if frame, ok := skinFamilyForEntry(entry.Family()); ok {
		cargo, push, e := skinFamilyCargo(ctx, w.characters.Store, saved.AccountID, saved.ID, w.skinCatalog, frame)
		if e != nil {
			event(map[string]any{"kind": "skin_cargo_family_error",
				"character_id": saved.ID, "category": frame.category, "reason": e.Error()})
		} else if push {
			out = append(out, outboundPacket{"skin_cargo_family", 0, 1545, cargo})
		}
	}
	// The 最近获得 strip (NOTI1547) is a whole-list frame like the pages above, and the
	// row it just registered is its newest tail element. Only once the row actually
	// stored: a spend whose registration failed has nothing to show, and re-pressing the
	// hotkey lands both frames.
	if unlockErr == nil {
		recent, e := skinRecentRestore(ctx, w.characters.Store, saved.AccountID, saved.State,
			w.skinCatalog, w.characters.WeaponSkinUsableFor(saved))
		if e != nil {
			event(map[string]any{"kind": "skin_recent_list_error",
				"character_id": saved.ID, "reason": e.Error()})
		} else if recent != nil {
			out = append(out, outboundPacket{"skin_recent_add", 0, 1547, recent})
		}
	}
	return out, nil
}

// damageFontCargo builds the absolute NOTI1545 damage-font page for an account.
// The page is pushed whole because sub_1444EFF40 rebuilds it from the frame: a
// partial list would delete the skins it omits.
func damageFontCargo(ctx context.Context, store *storage.Store, account int64, entries map[uint32]catalog.SkinStorageEntry) ([]byte, error) {
	skins, e := store.ListSkins(ctx, account)
	if e != nil {
		return nil, e
	}
	return protocol.SkinCargoPage(protocol.SkinCargoDamageFontPage, damageFontSkinIDs(skins, entries))
}

// damageFontSkinIDs selects the registered skins the damage-font page owns. The
// other families are left out until their page is proven, and several items can
// register the same skin, so identifiers are deduplicated.
func damageFontSkinIDs(skins []storage.AccountSkin, entries map[uint32]catalog.SkinStorageEntry) []uint32 {
	ids := make([]uint32, 0, len(skins))
	seen := make(map[uint32]bool, len(skins))
	for _, skin := range skins {
		entry, ok := entries[skin.SourceTemplate]
		if !ok || !entry.IsDamageFont() || seen[skin.SkinKey] {
			continue
		}
		seen[skin.SkinKey] = true
		ids = append(ids, skin.SkinKey)
	}
	return ids
}
