package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"fmt"
)

// skinFamilyFrame is the server-side description of one 皮肤仓库 family that the
// panel can select: which owned page it draws from and which selection category its
// 应用 / 解除 clicks carry.
//
// Page and category are the same number here because the client keys three different
// structures with one index: sub_1444EBD90 reads the owned page at mgr+136+16*page,
// the selection vectors hang off mgr+296 under the category, and each panel's grid
// refill keeps only the ids whose static registry record has that number as its
// family class (df22_ui_1441E3F40.c for 边框, df22_ui_1441E4180.c for 觉醒插图).
//
// The damage font is not in this table: its two tabs share page 2 under categories 2
// and 6, and its whole chain is already live in skin_selection_flow.go. Nothing here
// touches those frames.
type skinFamilyFrame struct {
	family   catalog.SkinFamily
	page     byte
	category uint32
}

var skinFamilyTable = []skinFamilyFrame{
	{family: catalog.SkinFamilyPartyFrame, page: protocol.SkinCargoPartyFramePage, category: protocol.SkinCategoryPartyFrame},
	{family: catalog.SkinFamilySkillCutscene, page: protocol.SkinCargoSkillCutscenePage, category: protocol.SkinCategorySkillCutscene},
}

func skinFamilyForCategory(category uint32) (skinFamilyFrame, bool) {
	for _, frame := range skinFamilyTable {
		if frame.category == category {
			return frame, true
		}
	}
	return skinFamilyFrame{}, false
}

// skinFamilyForEntry is the same table read from the consumed item's side: a skin
// whose family has no page the client renders returns false, so its registration
// stays durable-only.
func skinFamilyForEntry(family catalog.SkinFamily) (skinFamilyFrame, bool) {
	for _, frame := range skinFamilyTable {
		if frame.family == family {
			return frame, true
		}
	}
	return skinFamilyFrame{}, false
}

// skinFamilyBuiltins are the rows the client ships with and its own grid singles out
// of the owned page as the family's default cells. A page frame rebuilds the whole
// page (df13_noti1545_body_1444eff40.c clears the tree before it inserts), so a push
// that omitted them would delete them from the panel.
func skinFamilyBuiltins(family catalog.SkinFamily) []uint32 {
	switch family {
	case catalog.SkinFamilyPartyFrame:
		out := make([]uint32, 0, len(protocol.SkinCargoPartyFrameBuiltins)+1)
		out = append(out, protocol.SkinCargoPartyFrameBuiltins...)
		return append(out, protocol.SkinCargoPartyFrameDefaultList)
	case catalog.SkinFamilySkillCutscene:
		out := make([]uint32, 0, len(protocol.SkinCargoSkillCutsceneBuiltins))
		return append(out, protocol.SkinCargoSkillCutsceneBuiltins...)
	}
	return nil
}

// skinByID indexes the catalog by the skin id the client keys its pages with; the
// catalog map itself is keyed by item template, and several templates can register
// the same skin.
func skinByID(entries map[uint32]catalog.SkinStorageEntry) map[uint32]catalog.SkinStorageEntry {
	byID := make(map[uint32]catalog.SkinStorageEntry, len(entries))
	for _, entry := range entries {
		byID[entry.SkinKey()] = entry
	}
	return byID
}

// skinFamilySkinIDs picks the account's registered skins of one family, deduplicated.
func skinFamilySkinIDs(skins []storage.AccountSkin, entries map[uint32]catalog.SkinStorageEntry,
	family catalog.SkinFamily) []uint32 {
	ids := make([]uint32, 0, len(skins))
	seen := make(map[uint32]bool, len(skins))
	for _, skin := range skins {
		entry, ok := entries[skin.SourceTemplate]
		if !ok || entry.Family() != family || seen[skin.SkinKey] {
			continue
		}
		seen[skin.SkinKey] = true
		ids = append(ids, skin.SkinKey)
	}
	return ids
}

// skinFamilyCargo builds one family's absolute owned page.
//
// push is false while the account holds no skin of that family, and the caller then
// sends nothing at all: the 边框 page belongs to the profile-decoration feature as
// well, whose own push already carries its three built-in rows, and replacing it with
// a page that only repeats defaults would be a regression rather than an improvement.
//
// The page a 边框 push carries also has to keep whatever the character's profile state
// already owns. That state is part of the player's save and its owned list can hold
// more than the built-ins, and because NOTI1545 rebuilds the page it names, a page
// frame that omitted one of those rows would delete it from the panel.
func skinFamilyCargo(ctx context.Context, store *storage.Store, account, character int64,
	entries map[uint32]catalog.SkinStorageEntry, frame skinFamilyFrame) (payload []byte, push bool, e error) {
	skins, e := store.ListSkins(ctx, account)
	if e != nil {
		return nil, false, e
	}
	owned := skinFamilySkinIDs(skins, entries, frame.family)
	if len(owned) == 0 {
		return nil, false, nil
	}
	if frame.family == catalog.SkinFamilyPartyFrame {
		profile, e := store.RestoreProfileSkins(ctx, account, character)
		if e != nil {
			return nil, false, e
		}
		for _, item := range profile.Owned {
			owned = append(owned, item.ID)
		}
	}
	payload, e = protocol.SkinCargoPage(frame.page, skinFamilyPageIDs(frame.family, owned))
	return payload, e == nil, e
}

// skinFamilyPageIDs is the whole page: the client's own default rows first, then every
// skin the account registered, with duplicates dropped because several item templates
// can register the same skin.
func skinFamilyPageIDs(family catalog.SkinFamily, owned []uint32) []uint32 {
	all := append(append([]uint32{}, skinFamilyBuiltins(family)...), owned...)
	ids := make([]uint32, 0, len(all))
	seen := make(map[uint32]bool, len(all))
	for _, id := range all {
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	return ids
}

// skinFamilySelectionPayload encodes the NOTI1546 frame that re-applies one family's
// stored selection.
//
// The 边框 frame has two channels with different destinations, so the stored ids are
// partitioned by the skin's own PVF label: the raid party list frames go into the
// trailing list that refills the client's acquired set, and the other three partitions
// share the frame's three single-value slots. Which of those three slots one id came
// from does not matter — sub_1444EECA0 case 0 appends them in read order into one
// vector and each consumer filters that vector by the registry subtype it renders.
//
// The 觉醒插图 frame is the opposite: its two lists go to different destinations, so
// second is the 二觉 pool, which becomes mgr+1152 and only that family honours it.
func skinFamilySelectionPayload(category uint32, ids, second []uint32,
	byID map[uint32]catalog.SkinStorageEntry) ([]byte, error) {
	if category != protocol.SkinCategoryPartyFrame {
		return protocol.SkinSelectionSkillCutscene(ids, second)
	}
	singles := make([]uint32, 0, 3)
	raid := make([]uint32, 0, len(ids))
	for _, id := range ids {
		entry, ok := byID[id]
		if ok && entry.IsRaidPartyListFrame() {
			raid = append(raid, id)
			continue
		}
		singles = append(singles, id)
	}
	return protocol.SkinSelectionPartyFrame(singles, raid)
}

// skinCutscenePickedLists reads one 觉醒插图 应用 / 解除 request as the two lists the
// client itself built it from: slots 10..19 are mgr+1128 (the panel's 一觉 rows) and
// slots 0..9 are mgr+1176 (its 二觉 rows) — sub_1444F1410 lays them out that way before
// handing the vector to the composer
// (analysis/dumps/skin-noti/df36_fn_f1410_0x1444f1410.c:103-131), and each list takes at
// most ten rows (sub_1444E9240 / sub_1444E9290 both gate on size < 0xA).
//
// Each list is the set of rows the player ticked in that tab; sub_1444F1B60 / sub_1444F1BE0
// add the tab's own default id only when the assignment left the list empty, so 30000 and
// 100000 mean "nothing chosen" just while they stand alone in their list. Beside real rows
// they are that tab's default row ticked on purpose, and since NOTI1546 hands the second
// list straight back to mgr+1176, dropping one there un-ticks it the next time the panel
// opens — 实机 2026-09-28 flushes one applied skin as one bare id (17:11:02) and a ticked
// default as default+pick (17:12:19).
//
// The defaults never cross tabs, because 100000 is subtype 3 and the 一觉 tab cannot offer
// it: one found in the 一觉 list is a row an earlier round stored there and goes, while a
// non-default second-awakening row is re-routed, which also heals those stored rows.
func skinCutscenePickedLists(request protocol.SelectSkinRequest,
	byID map[uint32]catalog.SkinStorageEntry) (awakening, secondAwakening []uint32) {
	awake := make([]uint32, 0, len(request.Awakening))
	second := make([]uint32, 0, len(request.SecondAwakening))
	for _, id := range request.SecondAwakening {
		if id == protocol.SkinSelectionCutsceneDefault {
			continue
		}
		second = append(second, id)
	}
	rerouted := make([]uint32, 0, 2)
	for _, id := range request.Awakening {
		if id == protocol.SkinSelectionSecondAwakeningDefault {
			continue
		}
		if entry, ok := byID[id]; ok && entry.IsSecondAwakeningCutscene() {
			rerouted = append(rerouted, id)
			continue
		}
		awake = append(awake, id)
	}
	// A row that has to be moved over replaces the 二觉 tab's marker instead of joining its
	// picks beside it: the player never ticked it there.
	if len(rerouted) > 0 && len(second) == 1 &&
		second[0] == protocol.SkinSelectionSecondAwakeningDefault {
		second = rerouted
	} else {
		second = append(second, rerouted...)
	}
	return skinCutsceneKeepChosen(awake, protocol.SkinSelectionCutsceneDefault),
		skinCutsceneKeepChosen(second, protocol.SkinSelectionSecondAwakeningDefault)
}

// skinCutsceneKeepChosen collapses the two shapes "nothing chosen" can arrive as — an empty
// list, or one holding only its own tab's default id. The render pools express that state by
// themselves, since sub_1444EA8A0 and sub_1444EBAD0 both fall back to the default id when
// their pool is empty.
func skinCutsceneKeepChosen(ids []uint32, def uint32) []uint32 {
	if len(ids) == 0 || len(ids) == 1 && ids[0] == def {
		return nil
	}
	return ids
}

// skinCutsceneStoredLists rebuilds the same two pools from one stored id list. Rows keep no
// slot position, so each tab's own default is matched by value and every other row is
// classified by its PVF sub type label (catalog.IsSecondAwakeningCutscene); a default left
// alone in its list is the marker a 解除 click put there, not a pick.
func skinCutsceneStoredLists(ids []uint32,
	byID map[uint32]catalog.SkinStorageEntry) (awakening, secondAwakening []uint32) {
	awake := make([]uint32, 0, len(ids))
	second := make([]uint32, 0, len(ids))
	for _, id := range ids {
		if id == protocol.SkinSelectionCutsceneDefault {
			awake = append(awake, id)
			continue
		}
		if id == protocol.SkinSelectionSecondAwakeningDefault {
			second = append(second, id)
			continue
		}
		if entry, ok := byID[id]; ok && entry.IsSecondAwakeningCutscene() {
			second = append(second, id)
			continue
		}
		awake = append(awake, id)
	}
	return skinCutsceneKeepChosen(awake, protocol.SkinSelectionCutsceneDefault),
		skinCutsceneKeepChosen(second, protocol.SkinSelectionSecondAwakeningDefault)
}

// skinKeepOwned partitions one id list against the ids the client accepts for the family,
// so an id the account does not hold is reported instead of being pushed into a pool whose
// reader would drop it.
func skinKeepOwned(ids []uint32, owned map[uint32]bool) (kept, missing []uint32) {
	kept = make([]uint32, 0, len(ids))
	for _, id := range ids {
		if owned[id] {
			kept = append(kept, id)
			continue
		}
		missing = append(missing, id)
	}
	return kept, missing
}

// skinFamilyOwnedSet is what the client will accept for one family: the skins the
// account registered plus the family's own default rows, which are exactly the ids a
// 解除 click names.
func skinFamilyOwnedSet(skins []storage.AccountSkin, entries map[uint32]catalog.SkinStorageEntry,
	frame skinFamilyFrame) map[uint32]bool {
	owned := make(map[uint32]bool, len(skins)+4)
	for _, id := range skinFamilyBuiltins(frame.family) {
		owned[id] = true
	}
	for _, id := range skinFamilySkinIDs(skins, entries, frame.family) {
		owned[id] = true
	}
	return owned
}

// skinFamilySelectionFrame answers one 边框 or 觉醒插图 应用 / 解除 click with the two
// frames that make it stick.
//
// The order is the opposite of the damage-font pair, and the reason is in the echo
// reader itself: sub_1444EE820 rebuilds the category's vector from the body's slots,
// and its case 1 deliberately refuses to store a second-awakening id (it writes every
// slot whose registry subtype is not 3). So the echo has to come first — it performs
// the removal the client itself asked for — and NOTI1546 then replaces that vector
// with the validated selection, which is the frame the panel reads back for 生效中.
//
// The request body goes back verbatim rather than rebuilt from the accepted ids: the
// same reader also has a result==4 branch that erases one id from the acquired set,
// and no server-side id list can express that intent.
func skinFamilySelectionFrame(ctx context.Context, store *storage.Store, character, account int64,
	entries map[uint32]catalog.SkinStorageEntry, request protocol.SelectSkinRequest, body []byte,
	record map[string]any, event func(map[string]any)) ([]outboundPacket, error) {
	frame, ok := skinFamilyForCategory(request.Category)
	if !ok {
		record["kind"] = "skin_selection_unsupported_category"
		event(record)
		return nil, fmt.Errorf("skin selection category %d has no reversed page", request.Category)
	}
	echo, e := protocol.SelectSkinEchoRaw(body)
	if e != nil {
		return nil, e
	}
	if request.Result == 4 && request.Category == protocol.SkinCategoryPartyFrame {
		record["kind"] = "skin_selection_acquired_set_erase"
		event(record)
		return []outboundPacket{{"skin_selection_party_frame_erase", 1, 1565, echo}}, nil
	}
	skins, e := store.ListSkins(ctx, account)
	if e != nil {
		return nil, e
	}
	owned := skinFamilyOwnedSet(skins, entries, frame)
	var accepted, second []uint32
	rejected := make([]uint32, 0, len(request.SkinIDs))
	var missing []uint32
	if request.Category == protocol.SkinCategorySkillCutscene {
		awakening, secondAwakening := skinCutscenePickedLists(request, skinByID(entries))
		accepted, missing = skinKeepOwned(awakening, owned)
		rejected = append(rejected, missing...)
		second, missing = skinKeepOwned(secondAwakening, owned)
		rejected = append(rejected, missing...)
	} else {
		accepted, missing = skinKeepOwned(request.SkinIDs, owned)
		rejected = append(rejected, missing...)
	}
	if len(rejected) > 0 {
		record["rejected"] = rejected
	}
	selection, e := skinFamilySelectionPayload(request.Category, accepted, second, skinByID(entries))
	if e != nil {
		record["kind"] = "skin_selection_frame_error"
		record["reason"] = e.Error()
		event(record)
		return nil, e
	}
	// Both pools share the category's storage rows; the frame splits them again on read.
	stored := append(append([]uint32{}, accepted...), second...)
	if e = store.SetSkinSelectionList(ctx, character, request.Category, stored); e != nil {
		record["kind"] = "skin_selection_store_failed"
		record["reason"] = e.Error()
		event(record)
		return nil, e
	}
	if len(stored) == 0 {
		record["kind"] = "skin_selection_cleared"
	} else {
		record["kind"] = "skin_selection_applied"
		record["selected"] = accepted
		record["second_awakening"] = second
	}
	event(record)
	return []outboundPacket{
		{"skin_selection_family_echo", 1, 1565, echo},
		{"skin_selection_family_restored", 0, 1546, selection},
	}, nil
}

// restoreSkinFamilySelection reads one family's stored selection back into the frame
// that re-applies it, dropping ids the account no longer holds — the client's own
// reader filters against the owned page, so an unregistered id would silently vanish
// from the vector anyway and the server invents nothing to cover that up.
func restoreSkinFamilySelection(ctx context.Context, store *storage.Store, character, account int64,
	entries map[uint32]catalog.SkinStorageEntry, frame skinFamilyFrame) ([]byte, error) {
	keys, e := store.SkinSelectionList(ctx, character, frame.category)
	if e != nil || len(keys) == 0 {
		return nil, e
	}
	skins, e := store.ListSkins(ctx, account)
	if e != nil {
		return nil, e
	}
	owned := skinFamilyOwnedSet(skins, entries, frame)
	ids := make([]uint32, 0, len(keys))
	for _, key := range keys {
		if owned[key] {
			ids = append(ids, key)
		}
	}
	var second []uint32
	if frame.category == protocol.SkinCategorySkillCutscene {
		ids, second = skinCutsceneStoredLists(ids, skinByID(entries))
	}
	if len(ids) == 0 && len(second) == 0 {
		return nil, nil
	}
	return skinFamilySelectionPayload(frame.category, ids, second, skinByID(entries))
}

// restoreSkinFamilies fills the entry plan's page and selection payloads for both list
// families. Like the damage-font block beside it, a read failure only drops these
// frames: town entry must not depend on the skin storage the way the profile
// decoration state does.
func (p *entryPayloads) restoreSkinFamilies(ctx context.Context, store *storage.Store,
	account, character int64, entries map[uint32]catalog.SkinStorageEntry, event func(map[string]any)) {
	for _, frame := range skinFamilyTable {
		cargo, push, e := skinFamilyCargo(ctx, store, account, character, entries, frame)
		if e != nil {
			event(map[string]any{"kind": "skin_cargo_family_restore_error",
				"character_id": character, "category": frame.category, "reason": e.Error()})
			continue
		}
		var selection []byte
		if push {
			selection, e = restoreSkinFamilySelection(ctx, store, character, account, entries, frame)
			if e != nil {
				event(map[string]any{"kind": "skin_selection_family_restore_error",
					"character_id": character, "category": frame.category, "reason": e.Error()})
				continue
			}
		}
		switch frame.category {
		case protocol.SkinCategoryPartyFrame:
			p.SkinCargoPartyFrame, p.SkinSelectionPartyFrame = cargo, selection
		default:
			p.SkinCargoSkillCutscene, p.SkinSelectionSkillCutscene = cargo, selection
		}
	}
}

// skinFamilyRestoreFrames re-pushes both list families' pages and applied selections
// after the client has rebuilt its actor for a dungeon. The 觉醒插图 renderer reads its
// id when it creates the cutscene object and the border consumers read mgr+296, so a
// frame that arrived in town does not carry into the run. A read failure drops the
// frames, as the damage-font restore does for its own.
func (w *worldSession) skinFamilyRestoreFrames(ctx context.Context) []outboundPacket {
	var out []outboundPacket
	for _, frame := range skinFamilyTable {
		cargo, push, e := skinFamilyCargo(ctx, w.characters.Store, w.role.AccountID, w.role.ID,
			w.skinCatalog, frame)
		if e != nil || !push {
			continue
		}
		out = append(out, outboundPacket{"dungeon_skin_cargo_family_restored", 0, 1545, cargo})
		selection, e := restoreSkinFamilySelection(ctx, w.characters.Store, w.role.ID, w.role.AccountID,
			w.skinCatalog, frame)
		if e != nil || selection == nil {
			continue
		}
		out = append(out, outboundPacket{"dungeon_skin_selection_family_restored", 0, 1546, selection})
	}
	return out
}
