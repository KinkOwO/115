package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
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
	{family: catalog.SkinFamilyInstantEmoticon, page: protocol.SkinCargoInstantEmoticonPage, category: protocol.SkinCategoryInstantEmoticon},
	{family: catalog.SkinFamilySpray, page: protocol.SkinCargoSprayPage, category: protocol.SkinCategorySpray},
	{family: catalog.SkinFamilyAirshipEffect, page: protocol.SkinCargoAirshipEffectPage, category: protocol.SkinCategoryAirshipEffect},
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

// skinFamilyPage is the owned page a family's skins are pushed onto. The client keys
// three structures with that one number — owned page, selection category, and the
// family class it compares a 最近获得 row against — so the page byte doubles as the
// NOTI1547 kind (analysis/dumps/CLIENT-MECHANICS.md 14.1 and 20.2). The damage font is
// not in skinFamilyTable (its two tabs share page 2 under categories 2 and 6), so it is
// answered here rather than by the table.
func skinFamilyPage(family catalog.SkinFamily) (byte, bool) {
	if family == catalog.SkinFamilyDamageFont {
		return protocol.SkinCargoDamageFontPage, true
	}
	frame, ok := skinFamilyForEntry(family)
	return frame.page, ok
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
func skinFamilySkinIDs(skins []database.AccountSkin, entries map[uint32]catalog.SkinStorageEntry,
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
func skinFamilyCargo(ctx context.Context, store *database.Store, account, character int64,
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

// skinFamilyFramePayload dispatches one family's accepted selection to its own frame
// shape, because the five categories do not share one body layout:
//
//   - 0 边框 and 1 觉醒插图 are sets — every id in one vector, order irrelevant;
//   - 3 表情 is positional — the reader consumes exactly four words with no count and its
//     only consumer forwards that vector to the chat channel, so the slot index is part
//     of the state and a zero word has to be sent for an empty cell;
//   - 7 涂鸦 and 8 飞空艇特效 hold one id, and their reader appends it as a one-element
//     vector after the ownership check.
//
// A nil return for 3 / 7 / 8 means "no selection to state", which the callers treat as
// "send nothing": the 表情 frame with four zero words would be an explicit empty bar, and
// the 7 / 8 branch that receives an unowned id stores nothing at all, so an absent frame
// and a cleared frame are the same client state.
func skinFamilyFramePayload(category uint32, ids, second, slots []uint32,
	byID map[uint32]catalog.SkinStorageEntry) ([]byte, error) {
	switch category {
	case protocol.SkinCategoryInstantEmoticon:
		for _, id := range slots {
			if id != 0 {
				return protocol.SkinSelectionInstantEmoticon(slots)
			}
		}
		return nil, nil
	case protocol.SkinCategorySpray, protocol.SkinCategoryAirshipEffect:
		if len(ids) == 0 {
			return nil, nil
		}
		return protocol.SkinSelectionSingle(category, ids[0])
	}
	return skinFamilySelectionPayload(category, ids, second, byID)
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
func skinFamilyOwnedSet(skins []database.AccountSkin, entries map[uint32]catalog.SkinStorageEntry,
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
func skinFamilySelectionFrame(ctx context.Context, store *database.Store, character, account int64,
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
	var accepted, second, slots []uint32
	rejected := make([]uint32, 0, len(request.SkinIDs))
	var missing []uint32
	switch request.Category {
	case protocol.SkinCategorySkillCutscene:
		awakening, secondAwakening := skinCutscenePickedLists(request, skinByID(entries))
		accepted, missing = skinKeepOwned(awakening, owned)
		rejected = append(rejected, missing...)
		second, missing = skinKeepOwned(secondAwakening, owned)
		rejected = append(rejected, missing...)
	case protocol.SkinCategoryInstantEmoticon:
		// Position is kept: an unowned cell becomes a zero word rather than being dropped,
		// because dropping it would move every later cell into a different quick-bar slot.
		slots = make([]uint32, protocol.SkinSelectionInstantEmoticonSlots)
		for i, id := range request.EmoticonSlots {
			if i >= len(slots) {
				break
			}
			if id == 0 {
				continue
			}
			if !owned[id] {
				rejected = append(rejected, id)
				continue
			}
			slots[i] = id
		}
	default:
		accepted, missing = skinKeepOwned(request.SkinIDs, owned)
		rejected = append(rejected, missing...)
	}
	if len(rejected) > 0 {
		record["rejected"] = rejected
	}
	selection, e := skinFamilyFramePayload(request.Category, accepted, second, slots, skinByID(entries))
	if e != nil {
		record["kind"] = "skin_selection_frame_error"
		record["reason"] = e.Error()
		event(record)
		return nil, e
	}
	// Both pools share the category's storage rows; the frame splits them again on read.
	stored := append(append([]uint32{}, accepted...), second...)
	if request.Category == protocol.SkinCategoryInstantEmoticon {
		// The 表情 bar keeps its cell index, so it has its own positional table.
		e = store.SetSkinSelectionSlots(ctx, character, request.Category, slots)
		stored = stored[:0]
		for _, id := range slots {
			if id != 0 {
				stored = append(stored, id)
			}
		}
	} else {
		e = store.SetSkinSelectionList(ctx, character, request.Category, stored)
	}
	if e != nil {
		record["kind"] = "skin_selection_store_failed"
		record["reason"] = e.Error()
		event(record)
		return nil, e
	}
	if len(stored) == 0 {
		record["kind"] = "skin_selection_cleared"
	} else {
		record["kind"] = "skin_selection_applied"
		if slots != nil {
			// The 表情 bar's state is its four cells, not a flat id list.
			record["selected"] = slots
		} else {
			record["selected"] = accepted
			record["second_awakening"] = second
		}
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
func restoreSkinFamilySelection(ctx context.Context, store *database.Store, character, account int64,
	entries map[uint32]catalog.SkinStorageEntry, frame skinFamilyFrame) ([]byte, error) {
	byID := skinByID(entries)
	if frame.category == protocol.SkinCategoryInstantEmoticon {
		// Read back by cell, because the frame is positional; the width is the number of
		// words the client's reader consumes.
		slots, e := store.SkinSelectionSlots(ctx, character, frame.category,
			protocol.SkinSelectionInstantEmoticonSlots)
		if e != nil {
			return nil, e
		}
		owned, e := skinFamilyOwnedFilter(ctx, store, account, entries, frame)
		if e != nil {
			return nil, e
		}
		for i, id := range slots {
			if id != 0 && !owned[id] {
				slots[i] = 0
			}
		}
		return skinFamilyFramePayload(frame.category, nil, nil, slots, byID)
	}
	keys, e := store.SkinSelectionList(ctx, character, frame.category)
	if e != nil || len(keys) == 0 {
		return nil, e
	}
	owned, e := skinFamilyOwnedFilter(ctx, store, account, entries, frame)
	if e != nil {
		return nil, e
	}
	ids := make([]uint32, 0, len(keys))
	for _, key := range keys {
		if owned[key] {
			ids = append(ids, key)
		}
	}
	var second []uint32
	if frame.category == protocol.SkinCategorySkillCutscene {
		ids, second = skinCutsceneStoredLists(ids, byID)
	}
	if len(ids) == 0 && len(second) == 0 {
		return nil, nil
	}
	return skinFamilyFramePayload(frame.category, ids, second, nil, byID)
}

// skinFamilyOwnedFilter is the owned set as a map, fetched once per restore.
func skinFamilyOwnedFilter(ctx context.Context, store *database.Store, account int64,
	entries map[uint32]catalog.SkinStorageEntry, frame skinFamilyFrame) (map[uint32]bool, error) {
	skins, e := store.ListSkins(ctx, account)
	if e != nil {
		return nil, e
	}
	return skinFamilyOwnedSet(skins, entries, frame), nil
}

// restoreSkinFamilies fills the entry plan's page and selection payloads for every family
// in the table. Like the damage-font block beside it, a read failure only drops these
// frames: town entry must not depend on the skin storage the way the profile decoration
// state does.
//
// The two original families keep their named fields, because their page ordering relative
// to the profile-decoration push is pinned by a test. The three newer ones append to
// SkinFamilyRestores instead, which adds no field-per-family to the shared entry plan.
func (p *entryPayloads) restoreSkinFamilies(ctx context.Context, store *database.Store,
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
			continue
		case protocol.SkinCategorySkillCutscene:
			p.SkinCargoSkillCutscene, p.SkinSelectionSkillCutscene = cargo, selection
			continue
		}
		if push {
			p.SkinFamilyRestores = append(p.SkinFamilyRestores,
				outboundPacket{"skin_cargo_family_restored", 0, 1545, cargo})
		}
		if selection != nil {
			p.SkinFamilyRestores = append(p.SkinFamilyRestores,
				outboundPacket{"skin_selection_family_restored", 0, 1546, selection})
		}
	}
	if favorites, e := restoreSkinFavorites(ctx, store, character); e != nil {
		event(map[string]any{"kind": "skin_favorite_restore_error",
			"character_id": character, "reason": e.Error()})
	} else if favorites != nil {
		p.SkinFamilyRestores = append(p.SkinFamilyRestores,
			outboundPacket{"skin_favorites_restored", 0, 2641, favorites})
	}
}

// restoreSkinFavorites encodes the character's stored stars as the absolute NOTI2641
// frame. It returns nil when nothing is starred, which is the state the client already
// has on a fresh login — its favourite list starts empty, and only this frame can fill it.
func restoreSkinFavorites(ctx context.Context, store *database.Store, character int64) ([]byte, error) {
	pages, e := store.SkinFavorites(ctx, character, protocol.SkinFavoritePages)
	if e != nil || pages == nil {
		return nil, e
	}
	empty := true
	for _, ids := range pages {
		if len(ids) > 0 {
			empty = false
			break
		}
	}
	if empty {
		return nil, nil
	}
	return protocol.SkinFavorites(pages)
}

// skinFavoriteFrames answers one 星星 toggle.
//
// The toggle cannot be answered by the command echo alone: the CMD1565 reply core runs its
// whole category switch only for result 0, so a star click leaves the client's favourite
// list untouched, and that list's only writer is NOTI2641 (sub_1444E8DF0 has one caller,
// sub_1444ED1B0 — analysis/ida-work/df42.log section 5). The frame is absolute, so the
// answer is stored state re-encoded whole, including the all-empty case: after the last
// star on a page is removed, only a frame that says so can take the row away. The two
// frames are ordered by skinFavoriteAnswer, which is why the list goes out before the echo.
//
// The group a star belongs to is the skin's registry page, which for every family here is
// also its selection category — the one proven exception being the damage font, whose two
// tabs (2 and 6) both enumerate owned page 2.
func skinFavoriteFrames(ctx context.Context, store *database.Store, character int64,
	request protocol.SelectSkinRequest, body []byte, record map[string]any,
	event func(map[string]any)) ([]outboundPacket, error) {
	page := request.Category
	if page == protocol.SkinSelectionDamageFontCumulative {
		page = protocol.SkinCargoDamageFontPage
	}
	echo, e := protocol.SelectSkinEchoRaw(body)
	if e != nil {
		return nil, e
	}
	if page >= protocol.SkinFavoritePages || request.SkinID == 0 {
		record["kind"] = "skin_favorite_refused"
		record["reason"] = "no favourite group for this category or no skin named"
		event(record)
		return []outboundPacket{{"skin_selection_echo_only", 1, 1565, echo}}, nil
	}
	starred := request.Result == protocol.SkinSelectResultFavoriteAdd
	ok, e := store.SetSkinFavorite(ctx, character, page, request.SkinID, starred,
		protocol.SkinFavoriteCapPerGroup)
	if e != nil {
		record["kind"] = "skin_favorite_store_failed"
		record["reason"] = e.Error()
		event(record)
		return nil, e
	}
	// The client refuses an eleventh star with message 101037008 of its own; the server
	// drops the add the same way rather than pushing a list the panel would not show.
	record["kind"] = "skin_favorite_refused"
	if ok {
		if starred {
			record["kind"] = "skin_favorite_added"
		} else {
			record["kind"] = "skin_favorite_removed"
		}
	}
	event(record)
	favorites, e := restoreSkinFavorites(ctx, store, character)
	if e != nil {
		return nil, e
	}
	// The stored state is empty only when the last star was just removed, and that is
	// exactly the case where the client still holds the old node and needs the frame.
	if favorites == nil {
		favorites, e = protocol.SkinFavorites(make([][]uint32, protocol.SkinFavoritePages))
		if e != nil {
			return nil, e
		}
	}
	return skinFavoriteAnswer(echo, favorites), nil
}

// skinFavoriteAnswer orders the two frames a star click is answered with: the absolute 收藏
// list first, the command echo last.
//
// The echo is what repaints the window: for a non-zero result `sub_1444EE820` skips its
// whole category switch and only tail-calls `sub_1444F1EE0(mgr, category)`, which rebuilds
// the page from the manager's tables (analysis/dumps/skin-noti/rd_cmd1565_select_skin_else_1444ee820.c,
// df13_cargo_render_tail_1444f1ee0.c). So an echo that arrives *before* NOTI2641 repaints
// from the 收藏 table as it stood before the click, and the star and the 概要 rows then sit
// stale until something else refreshes the page (live 2026-09-28: 收藏已入库，但星星不点亮、
// 概要也不随分类应用变化). Same rule the normal-damage reset path already states: the echo
// goes second so its refresh sees what the other frame just wrote.
func skinFavoriteAnswer(echo, favorites []byte) []outboundPacket {
	return []outboundPacket{
		{"skin_favorites_restored", 0, 2641, favorites},
		{"skin_selection_echo_only", 1, 1565, echo},
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
		cargo, push, e := skinFamilyCargo(ctx, w.store, w.role.AccountID, w.role.ID,
			w.skinCatalog, frame)
		if e != nil || !push {
			continue
		}
		out = append(out, outboundPacket{"dungeon_skin_cargo_family_restored", 0, 1545, cargo})
		selection, e := restoreSkinFamilySelection(ctx, w.store, w.role.ID, w.role.AccountID,
			w.skinCatalog, frame)
		if e != nil || selection == nil {
			continue
		}
		out = append(out, outboundPacket{"dungeon_skin_selection_family_restored", 0, 1546, selection})
	}
	return out
}
