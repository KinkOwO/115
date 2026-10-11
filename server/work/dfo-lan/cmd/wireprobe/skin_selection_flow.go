package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

// selectSkin answers CMD1565, the frame the skin cargo's 应用 and 解除 buttons
// send. The client applies nothing on its own: the live 2026-09-27 capture shows
// the request landing while the damage numbers stayed unchanged, because both
// paths that reach a font setter (NOTI1546's selection reader sub_1444EECA0, and
// this opcode's own receive handler sub_1444EE820) wait for the server first.
//
// The first u32 of the body is the selection category, not a cargo page. The
// damage-font panel has two tabs that both enumerate owned page 2, and each keeps
// its own vector (sub_1444EECA0 case 2 / case 6), so the answer has to carry the
// category the click came from — answering the other one lights the wrong tab.
// Categories outside that pair are the list panels — 边框, 觉醒插图, 表情, 涂鸦 and
// 飞空艇特效 — whose click carries a whole selection and is answered in
// skin_family_flow.go, and the 武器外观 tab, answered by the replication path's
// syncSkin; a category with no reversed reader at all is logged and refused instead of
// being answered with an invented frame.
func (w *worldSession) selectSkin(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.characters == nil {
		return nil, fmt.Errorf("skin selection before character load")
	}
	request, e := protocol.DecodeSelectSkin(p)
	if e != nil {
		return nil, e
	}
	record := map[string]any{"character_id": w.role.ID, "category": request.Category,
		"result": request.Result, "skin_key": request.SkinID}
	// A star click is not an apply: its composer writes 2 or 3 into the result slot
	// (analysis/dumps/skin-noti/df39_sender_F0FE0.c: v12[1] = (a4 != 0) + 2), and the
	// command reply core skips its whole category switch for anything but 0, so the
	// client applies nothing and the server owns the answer. This branch has to come
	// before the family routing below, whose categories are the same numbers.
	if request.Favorite {
		if w.role.ID == 0 {
			return nil, fmt.Errorf("skin selection before character load")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return skinFavoriteFrames(ctx, w.store, w.role.ID, request, p, record, event)
	}
	if protocol.IsSkinSelectionDamageFontCategory(request.Category) {
		if w.skinCatalog == nil {
			return nil, fmt.Errorf("skin storage catalog is not loaded")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return damageFontSelectionFrame(ctx, w.store, w.role.ID, w.role.AccountID,
			w.skinCatalog, request.Category, request.SkinID, record, event)
	}
	// The 边框 and 觉醒插图 panels carry a whole selection in one click instead of a
	// single id, so they answer through their own path, which also has to echo the
	// request body untouched.
	if _, ok := skinFamilyForCategory(request.Category); ok {
		if w.skinCatalog == nil {
			return nil, fmt.Errorf("skin storage catalog is not loaded")
		}
		record["skin_keys"] = request.SkinIDs
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return skinFamilySelectionFrame(ctx, w.store, w.role.ID, w.role.AccountID,
			w.skinCatalog, request, p, record, event)
	}
	// The 武器外观 tab belongs to the replication path, not to this one. Its 应用 is
	// the same 88-byte frame under the weapon-shape category, and the client's own
	// composer zeroes the result slot for every category it builds
	// (analysis/dumps/skin-noti/df39_sender_F1090.c: memset(&v24[1], 0, 84) leaves
	// v24[1] = 0), so a category-4 frame with result 0 is "wear this skin" — which
	// only syncSkin can answer, because the weapon skin is stored on the character
	// and the actor has to be rebuilt. The live capture is the refusal this used to
	// produce: five frames, category 4, skin 27694, result 0
	// (skin_selection_unsupported_category).
	//
	// A non-zero result on this category is not an apply: the only other sender of
	// that slot writes 2 or 3 (df39_sender_F0FE0.c: v12[1] = (a4 != 0) + 2), which is
	// the star toggle, so it stays unhandled below rather than wearing the skin.
	if request.Category == protocol.SkinCargoWeaponShape && request.Result == 0 {
		return w.syncSkin(p, event)
	}
	record["kind"] = "skin_selection_unsupported_category"
	event(record)
	return nil, fmt.Errorf("skin selection category %d has no reversed meaning", request.Category)
}

// damageFontSelectionFrame persists one click and builds the frames that replay
// it. 解除 is a request for that tab's own built-in font by name, so the answer has
// to carry the id the click named; storing 0 keeps the entry path from re-applying
// the font the player just took off.
func damageFontSelectionFrame(ctx context.Context, store *database.Store, character, account int64,
	entries map[uint32]catalog.SkinStorageEntry, category, skinKey uint32, record map[string]any,
	event func(map[string]any)) ([]outboundPacket, error) {
	// The client resets the font to its built-in default for any id the owned page
	// does not hold, so echoing an unregistered id would unequip rather than apply.
	skins, e := store.ListSkins(ctx, account)
	if e != nil {
		return nil, e
	}
	owned := false
	for _, id := range damageFontSkinIDs(skins, entries) {
		if id == skinKey {
			owned = true
			break
		}
	}
	// The reset id only counts as one while the account does not hold it: a click on
	// an owned id numbered like the default is an ordinary 应用.
	if resetID, ok := protocol.SkinSelectionResetID(category); ok && !owned && skinKey == resetID {
		if e := store.SelectSkin(ctx, character, category, 0); e != nil {
			record["kind"] = "skin_selection_store_failed"
			record["reason"] = e.Error()
			event(record)
			return nil, e
		}
		record["kind"] = "skin_selection_unequipped"
		event(record)
		return damageFontSelectionFrames("skin_selection_damage_font_reset", category, resetID)
	}
	if !owned {
		record["kind"] = "skin_selection_refused"
		record["reason"] = "skin is not in the account damage-font page"
		event(record)
		return nil, fmt.Errorf("damage font skin %d is not owned", skinKey)
	}
	if e = store.SelectSkin(ctx, character, category, skinKey); e != nil {
		record["kind"] = "skin_selection_store_failed"
		record["reason"] = e.Error()
		event(record)
		return nil, e
	}
	record["kind"] = "skin_selection_applied"
	event(record)
	return damageFontSelectionFrames("skin_selection_damage_font", category, skinKey)
}

// damageFontSelectionFrames builds the answer to one panel click: the NOTI1546
// selection the tab reads back. The normal-damage tab's reset needs the CMD1565
// echo as well, because its NOTI1546 branch clears only the 生效中 marker and
// leaves the rendered font at its old id; the cumulative tab's branch resets it
// itself. The echo goes second so its refresh sees the id NOTI1546 left behind.
//
// The two frames use different envelope kinds on purpose: 1546 is a
// notification (kind 0), while the echo is a command reply (kind 1). The client
// only looks 1565 up in its CMD registry when the envelope byte is 1.
func damageFontSelectionFrames(kind string, category, id uint32) ([]outboundPacket, error) {
	sel, e := protocol.SkinSelectionDamageFont(category, id)
	if e != nil {
		return nil, e
	}
	out := []outboundPacket{{kind, 0, 1546, sel}}
	if category != protocol.SkinSelectionDamageFontNormal || id != protocol.SkinSelectionNormalDamageDefaultFont {
		return out, nil
	}
	echo, e := protocol.SelectSkinEcho(category, id)
	if e != nil {
		return nil, e
	}
	return append(out, outboundPacket{"skin_selection_normal_damage_reset_echo", 1, 1565, echo}), nil
}

// damageFontRestore re-pushes the owned skin pages and their applied selections after
// the client has rebuilt its actor for a dungeon. The damage number renderer resolves
// its font when it creates the text object, reading the applied id off the avatar
// (sub_1447EB510 with its id argument -1 takes *(avatar+232), which only
// sub_142581F20 writes), so a frame that arrived in town does not carry into the
// run. A read failure only drops these frames, as the worn-visual restore already
// does for its own.
func (w *worldSession) damageFontRestore() []outboundPacket {
	if w == nil || w.characters == nil || w.skinCatalog == nil || w.role.ID == 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var out []outboundPacket
	cargo, e := damageFontCargo(ctx, w.store, w.role.AccountID, w.skinCatalog)
	if e == nil {
		out = append(out, outboundPacket{"dungeon_skin_cargo_damage_font_restored", 0, 1545, cargo})
		for _, category := range damageFontSelectionCategories {
			sel, e := restoreDamageFontSelection(ctx, w.store, w.role.ID, w.role.AccountID,
				w.skinCatalog, category)
			if e != nil || sel == nil {
				continue
			}
			out = append(out, outboundPacket{"dungeon_skin_selection_damage_font_restored", 0, 1546, sel})
		}
	}
	return append(out, w.skinFamilyRestoreFrames(ctx)...)
}

// damageFontSelectionCategories are the two tabs of the damage-font panel, in the
// order their frames are replayed: the owned page has to arrive first.
var damageFontSelectionCategories = []uint32{
	protocol.SkinSelectionDamageFontNormal,
	protocol.SkinSelectionDamageFontCumulative,
}

// restoreDamageFontSelection reads the character's applied font for one tab back
// into the frame that selects it. A skin the account no longer holds is not
// replayed: the client takes an unknown id as a reset, which looks the same as
// sending nothing, so nothing is invented here either.
func restoreDamageFontSelection(ctx context.Context, store *database.Store, character, account int64,
	entries map[uint32]catalog.SkinStorageEntry, category uint32) ([]byte, error) {
	skinKey, e := store.SelectedSkin(ctx, character, category)
	if e != nil || skinKey == 0 {
		return nil, e
	}
	skins, e := store.ListSkins(ctx, account)
	if e != nil {
		return nil, e
	}
	for _, id := range damageFontSkinIDs(skins, entries) {
		if id == skinKey {
			return protocol.SkinSelectionDamageFont(category, skinKey)
		}
	}
	return nil, nil
}
