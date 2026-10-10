package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

// makeSkin creates the source recipe's consumable first. Registration is a
// separate CMD507 action169 transaction, using the native instance weapon id.
func (w *worldSession) makeSkin(plaintext []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.characters == nil {
		return nil, fmt.Errorf("make skin before character selection")
	}
	req, e := protocol.DecodeMakeSkin(plaintext)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, cost, applied, e := w.characters.GenerateWeaponShape(ctx, w.role, req.Index, req.Mode)
	if e != nil {
		return nil, e
	}
	w.role = saved
	bag, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	restore, e := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if e != nil {
		return nil, e
	}
	event(map[string]any{"kind": "weapon_shape_product_created", "character_id": saved.ID, "weapon": cost.Skin, "weapon_slot": cost.Slot, "mold": cost.Mold, "mold_slot": cost.MoldSlot, "mold_left": cost.MoldAmount, "product": cost.Product, "product_slot": cost.ProductSlot, "applied": applied, "mode": req.Mode})
	// Authoritative inventory precedes the existing native success ack, clearing
	// the forge's reserved icon. No owned-page push occurs before item use.
	return []outboundPacket{{"skin_replicate_bag_resynced", 0, 13, restore}, {"skin_replicate_completed", 1, 1592, protocol.MakeSkinSuccess()}}, nil
}

// skinCargoRestore rebuilds the NOTI1545 the skin storage window needs at entry.
//
// The container only ever got a push alongside the replication confirmation, so
// a reconnect showed an empty storage even though the applied skin was still on
// the character. The saved list is replayed here; the entry packet order in
// entryPayloads.packets() places it after the actor/appearance block.
//
// `usable` is the job gate the apply path already refuses by, and nil means "keep
// everything" — the state without an equipment catalog, which is the pre-fix list.
func skinCargoRestore(state json.RawMessage, usable func(uint32) bool) ([]byte, error) {
	bag, e := inventory.ReadBag(state)
	if e != nil {
		return nil, e
	}
	ids := weaponSkinPageIDs(bag, usable)
	if len(ids) == 0 {
		return nil, nil
	}
	entries := make([]protocol.SkinCargoEntry, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, protocol.SkinCargoEntry{SkinID: id, Extra: 1})
	}
	return protocol.SkinCargoInfo(byte(protocol.SkinCargoWeaponShape), entries, nil)
}

// weaponSkinPageIDs is the 武器外观 page as one character should see it: the saved list
// minus the shapes this job cannot wear.
//
// The panel itself does no job check — its filler 0x1441e47b0 snapshots page 4 and keeps
// every row, because the predicate it applies reads the *constant* 0x9C40 registry record
// rather than the entry (analysis/dumps/skin-noti/df22_ui_1441E47B0.c:40-41). So the
// server listing an id is the only thing that puts it on screen, and the server dropping
// it is the only thing that takes it off. Entries that predate the replication job gate
// therefore still stared at the player (live 2026-09-28: 不适用于该武器类型的角色职业下
// 出现幻化的武器外观); nothing is deleted from the save, only not pushed.
//
// The worn skin is kept unconditionally: its row is where the 解除 button lives, so
// hiding a legacy worn shape would leave the player wearing something they can no
// longer take off.
func weaponSkinPageIDs(bag inventory.Bag, usable func(uint32) bool) []uint32 {
	ids := bag.WeaponSkinStorage()
	if usable == nil {
		return ids
	}
	out := make([]uint32, 0, len(ids))
	for _, id := range ids {
		if id == bag.WeaponSkin || usable(id) {
			out = append(out, id)
		}
	}
	return out
}

// skinSelectionRestore rebuilds the NOTI1546 the storage window needs at entry.
//
// The worn row's golden frame is drawn from the client's own per-page selection
// table, and opening the window rebuilds that frame from the table rather than from
// the save. Nothing ever fed the table: the Apply handler only sets the window-local
// detail cell (0x1441d7b70) and the frame vanishes as soon as the window closes
// (live 2026-09-27, screenshot pair). Entry re-states the worn skin once, so the
// first open after a relog already highlights it.
//
// A character with no skin applied gets nothing: a zero id is not a selection, and
// the entry packet list skips empty payloads.
func skinSelectionRestore(state json.RawMessage) ([]byte, error) {
	bag, e := inventory.ReadBag(state)
	if e != nil {
		return nil, e
	}
	if bag.WeaponSkin == 0 {
		return nil, nil
	}
	return protocol.SkinCargoSelectionInfo(bag.WeaponSkin), nil
}

// syncSkin answers CMD1565, the skin storage window's container sync. The client
// sends the same 88-byte frame when a tab is opened and when the player presses
// Apply; the Apply handler hard-codes the weapon-shape subtype, so a subtype-4
// frame naming a skin is the "wear this on my weapon" message. Nothing in the
// body separates an apply from a browse, so the reported id is treated as the
// applied weapon skin - browsing a single-entry container re-sends the same id
// and is a no-op here.
//
// An empty id list on subtype 4 is the window's 解除 (unapply) button: the frame
// names no skin, and the stored skin has to go to zero so the appearance
// projection falls back to the worn weapon's own model. Dropping that frame is
// exactly why the button did nothing (live 2026-09-27, the ids=[] / ids=[27359]
// alternation in the session log as the player toggled it).
//
// The skin is only stored and the actor rebuilt when it actually changes: the
// client re-sends this frame on every step through the list, and one mode0
// userinfo per click would rebuild the world actor needlessly.
func (w *worldSession) syncSkin(plaintext []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.characters == nil {
		return nil, fmt.Errorf("skin sync before a character is loaded")
	}
	req, e := protocol.DecodeSkinCargoSync(plaintext)
	if e != nil {
		return nil, e
	}
	event(map[string]any{
		"kind": "skin_cargo_sync", "character_id": w.role.ID,
		"subtype": req.Subtype, "reserved": req.Reserved, "ids": req.IDs,
	})
	if req.Subtype != protocol.SkinCargoWeaponShape {
		return nil, nil
	}
	// An empty list is the unapply button, so it resolves to the zero skin rather
	// than being dropped.
	skin := uint32(0)
	if len(req.IDs) > 0 {
		skin = req.IDs[0]
	}
	bag, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	if bag.WeaponSkin == skin {
		return nil, nil
	}
	// Key on the transition plus the stored sequence: the client repeats identical
	// frames, and apply-A / apply-B / apply-A-again must be three distinct events.
	// The (previous, next) pair alone is not enough - the third call would replay
	// the first one's receipt, the closure would never run, and the actor would stay
	// on B (live 2026-09-27, "替换不生效"). WeaponSkinSeq increments on every real
	// change, so including it makes each transition a fresh key.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := fmt.Sprintf("weapon-skin-v2:%d:%d:%d", w.role.ID, bag.WeaponSkinSeq, skin)
	saved, changed, e := w.characters.ApplyWeaponSkin(ctx, w.role, key, skin)
	if e != nil {
		if errors.Is(e, inventory.ErrWeaponSkinNotUsable) || errors.Is(e, inventory.ErrWeaponSkinNotOwned) {
			// 尚未拥有或本职业戴不上这件外观：不写档、不换
			// 外观、也不报错回滚整个入场。但要把客户端刚点出来的本地选中拨回存档里的
			// 真实值——客户端是乐观绘制的，只丢一个错误会让那一行一直停在「解除」，
			// 玩家再点也只是重复发同一个空帧（实机 2026-09-27 的同类现象）。
			event(map[string]any{
				"kind": "weapon_skin_refused", "character_id": w.role.ID,
				"skin": skin, "stored": bag.WeaponSkin, "reason": e.Error(),
			})
			return []outboundPacket{
				{"skin_cargo_selected", 0, 1546, protocol.SkinCargoSelectionInfo(bag.WeaponSkin)},
			}, nil
		}
		return nil, e
	}
	w.role = saved
	if !changed {
		return nil, nil
	}
	kind := "weapon_skin_applied"
	if skin == 0 {
		kind = "weapon_skin_cleared"
	}
	event(map[string]any{
		"kind": kind, "character_id": w.role.ID,
		"skin": skin, "previous": bag.WeaponSkin,
	})
	if w.activeDungeon != nil {
		// Native CMD1565 case 4 changes the weapon without rebuilding the
		// actor, whose mode-0 reconstruction can break dungeon door state.
		return weaponSkinNativeRefresh(skin, "weapon_skin_dungeon_refreshed"), nil
	}
	// Keep the established town appearance projection: slot 12 carries
	// the skin template as both placeholder (the town
	// model lookup) and model, and falls back to the worn weapon's own template
	// when the stored skin is zero - which is what unapply needs.
	probe, e := w.characters.AppearanceProbe(saved, [2]byte{})
	if e != nil {
		return nil, e
	}
	// State the choice back through NOTI1546 as well. Apply only paints the
	// window-local detail cell, so without this the worn row loses its frame the
	// moment the storage is reopened; the appearance block cannot carry it.
	//
	// The frame goes out for an unapply too, carrying a zero id. The reader prunes
	// the page's selection before it even looks at the id - 0x1444eecd8 walks the
	// manager's per-subtype map to the node for this subtype and does
	// node[+0x30] = node[+0x28], i.e. vector::clear() - and only writes an id back
	// when it resolves to a live entry. So a zero id leaves the page with an empty
	// selection, which is exactly what "nothing is worn" means. Skipping the frame
	// left the client still calling the cleared skin the worn one: its row kept the
	// 解除 button, and clicking that re-sent the empty frame, which the server
	// correctly ignores - so the only way out was to apply a different skin first
	// (live 2026-09-27, four consecutive ids:[] frames in the session log).
	return []outboundPacket{
		{"skin_cargo_selected", 0, 1546, protocol.SkinCargoSelectionInfo(skin)},
		{"weapon_skin_applied_refreshed", 0, 2, probe},
		// Set the native override after the town weapon-avatar rebuild.
		{"weapon_skin_town_refreshed", 1, 1565, protocol.WeaponSkinSelectionEcho(skin)},
	}, nil
}

func weaponSkinNativeRefresh(skin uint32, name string) []outboundPacket {
	return []outboundPacket{
		{"skin_cargo_selected", 0, 1546, protocol.SkinCargoSelectionInfo(skin)},
		{name, 1, 1565, protocol.WeaponSkinSelectionEcho(skin)},
	}
}

// Restore after the complete worn/Clone reconstruction, which can replace slot
// 12 with the real weapon. This changes the live view, never the item instance.
func dungeonWeaponSkinRestore(w *worldSession) ([]outboundPacket, error) {
	if w == nil || w.characters == nil || w.activeDungeon == nil {
		return nil, nil
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, err
	}
	if bag.WeaponSkin == 0 {
		return nil, nil
	}
	return weaponSkinNativeRefresh(bag.WeaponSkin, "dungeon_weapon_skin_restored"), nil
}

func appendDungeonWeaponSkinAfterMove(plan []outboundPacket, w *worldSession, move protocol.ItemMoveRequest) ([]outboundPacket, error) {
	if len(plan) == 0 || !moveTouchesWorn(move) || w == nil || w.characters == nil {
		return plan, nil
	}
	// Town equipment/clone rebuilds need the same final native override as a
	// dungeon rebuild. In particular wearing slot 10 must not cover the skin.
	echo, err := weaponSkinActorRestore(w.role.State)
	if err != nil {
		return nil, err
	}
	if len(echo) == 0 {
		return plan, nil
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, err
	}
	return append(plan, weaponSkinNativeRefresh(bag.WeaponSkin, "weapon_skin_equipment_restored")...), nil
}

func weaponSkinActorRestore(state json.RawMessage) ([]byte, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, err
	}
	if bag.WeaponSkin == 0 {
		return nil, nil
	}
	return protocol.WeaponSkinSelectionEcho(bag.WeaponSkin), nil
}

// skinSlotItem resolves the list-0 slot the replication window reported. Bag
// equipment (slots 9..64) is stored apart from the ordinary rows, so both are
// searched; the container name is reported so the event log shows which one the
// index actually addressed.
func skinSlotItem(bag inventory.Bag, slot uint16) (uint32, string, bool) {
	for _, item := range bag.Equipment {
		if item.Slot == slot {
			return item.Template, "equipment", true
		}
	}
	for _, item := range bag.Items {
		if item.Slot == slot {
			return item.Template, "items", true
		}
	}
	return 0, "", false
}

// skinRecentList builds the whole NOTI1547 body the storage window's 最近获得 strip
// renders.
//
// Two properties of the client decide the shape (analysis/dumps/CLIENT-MECHANICS.md 20):
// the reader sub_1444ED400 clears the manager vector before its loop, so one frame is the
// list's absolute state and every acquisition has to re-send all of it; and the row cell
// sub_1441E2620 refuses to draw an entry whose second byte is not the family class its
// static registry has for that id, so `kind` is the skin's own page rather than a flag.
// Page 4 is the one family whose id is a weapon item template instead of a skin id.
//
// Order is oldest first because sub_1441EAF20 walks the vector back from the end and puts
// the tail in the first cell. The count is one byte, and a list longer than it drops from
// the head so the newest entries stay.
func skinRecentList(skins []database.AccountSkin, entries map[uint32]catalog.SkinStorageEntry,
	weapons []uint32) ([]byte, error) {
	ordered := make([]database.AccountSkin, len(skins))
	copy(ordered, skins)
	sort.SliceStable(ordered, func(i, j int) bool {
		if !ordered[i].UnlockedAt.Equal(ordered[j].UnlockedAt) {
			return ordered[i].UnlockedAt.Before(ordered[j].UnlockedAt)
		}
		return ordered[i].SourceTemplate < ordered[j].SourceTemplate
	})
	out := make([]protocol.RecentAddSkinEntry, 0, len(ordered)+len(weapons))
	seen := make(map[uint32]bool, len(ordered)+len(weapons))
	for _, skin := range ordered {
		entry, ok := entries[skin.SourceTemplate]
		if !ok {
			continue
		}
		page, ok := skinFamilyPage(entry.Family())
		if !ok || seen[skin.SkinKey] {
			continue
		}
		seen[skin.SkinKey] = true
		out = append(out, protocol.RecentAddSkinEntry{Kind: page, SkinID: skin.SkinKey})
	}
	for _, id := range weapons {
		if id == 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, protocol.RecentAddSkinEntry{
			Kind: byte(protocol.SkinCargoWeaponShape), SkinID: id})
	}
	if len(out) > 255 {
		out = out[len(out)-255:]
	}
	// An empty frame is not a no-op: the reader clears the vector before its loop, so a
	// zero count erases the whole strip. Nothing drawn yet means nothing to send.
	if len(out) == 0 {
		return nil, nil
	}
	return protocol.RecentAddSkinList(out)
}

// skinRecentPayload is skinRecentList for one saved character: the account's registered
// skins plus that character's replicated weapon shapes, which is every id the window can
// call recently acquired. `usable` narrows the weapon ids to the shapes this job can wear,
// for the same reason the page frame does — see weaponSkinPageIDs.
func skinRecentPayload(skins []database.AccountSkin, entries map[uint32]catalog.SkinStorageEntry,
	state json.RawMessage, usable func(uint32) bool) ([]byte, error) {
	bag, e := inventory.ReadBag(state)
	if e != nil {
		return nil, e
	}
	return skinRecentList(skins, entries, weaponSkinPageIDs(bag, usable))
}

// skinRecentRestore reads the account's registrations and encodes the entry push. A
// storage read failure leaves the frame unset, which is the pre-fix state: the town must
// not depend on the skin storage the way the character save does.
func skinRecentRestore(ctx context.Context, store *database.Store, account int64,
	state json.RawMessage, entries map[uint32]catalog.SkinStorageEntry,
	usable func(uint32) bool) ([]byte, error) {
	if store == nil {
		return nil, nil
	}
	skins, e := store.ListSkins(ctx, account)
	if e != nil {
		return nil, e
	}
	return skinRecentPayload(skins, entries, state, usable)
}
