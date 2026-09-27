package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// makeSkin answers CMD1592 (ENUM_CMDPACKET_MAKE_SKIN), the blacksmith window's
// replication confirmation.
//
// Replicating consumes the weapon and one Linus mold - the client's own
// confirmation text says exactly that ("Replicating consumes the selected weapon
// and materials.", "Not enough Linus' Steel Molds or Molds.") - so the whole
// settlement happens in one transaction before any packet goes out. A refusal
// (no weapon in that slot, a weapon this class cannot use, no mold) answers with
// nothing at all, which is what the player sees as the confirmation not going
// through; buying a mold and confirming again then works, because a refused
// attempt is not recorded.
//
// The request names no item, so the target is whatever sits in the reported
// index. The index is a list-0 slot: the window field it comes from is the same
// one the sender uses to look the entry up, and the captured value 9 is exactly
// this build's first bag equipment slot (equipment_slots [9,64]).
//
// The skin goes into the weapon-shape container only. The first build filled all
// ten subtypes because the tab-to-subtype mapping was unknown; the live capture
// then pinned it - the page showing the replicated katana is the one whose
// CMD1565 reports subtype 4 - so the same entry no longer pollutes the other
// tabs.
func (w *worldSession) makeSkin(plaintext []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.characters == nil {
		return nil, fmt.Errorf("make skin before a character is loaded")
	}
	req, e := protocol.DecodeMakeSkin(plaintext)
	if e != nil {
		return nil, e
	}
	bag, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	template, container, ok := skinSlotItem(bag, req.Index)
	event(map[string]any{
		"kind": "make_skin_request", "character_id": w.role.ID,
		"op": req.Op, "mode": req.Mode, "flag": req.Flag, "index": req.Index,
		"container": container, "template": template, "resolved": ok,
	})
	if !ok {
		return nil, fmt.Errorf("make skin index %d holds no item", req.Index)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Keyed by the bag slot the window reported: replicating a different weapon
	// is a different event, while a repeated identical confirmation replays its
	// receipt instead of eating a second weapon.
	//
	// The -v2 suffix is a version bump, not a semantic change: CommitCharacterEvent
	// is idempotent by event_key alone and refuses to run the closure when the key
	// exists under a different model ("character event model mismatch"), so the rows
	// the pre-fix binary wrote under weapon-skin-make:<char>:<slot>:<template> made
	// every retry of those exact weapons fail forever (live 2026-09-27: char 2 slot
	// 19 and char 3 slot 15, both beamswords). A new key starts a clean series.
	key := fmt.Sprintf("weapon-skin-make-v2:%d:%d:%d", w.role.ID, req.Index, template)
	saved, spent, consumed, e := w.characters.ReplicateWeaponSkin(ctx, w.role, key, req.Index, req.Mode)
	if e != nil {
		// 拒绝（槽位不是武器 / 本职业用不了 / 没有模具）走这里：一个字节都不发。
		// 主循环把它记成 make_skin_refused，玩家换一件再来时事件不存在，可以重来。
		return nil, e
	}
	w.role = saved
	stored, se := inventory.ReadBag(saved.State)
	if se != nil {
		return nil, se
	}
	ids := stored.WeaponSkinStorage()
	event(map[string]any{
		"kind": "weapon_skin_stored", "character_id": w.role.ID, "skin": template,
		"stored": ids, "consumed": consumed, "duplicate": spent.Duplicate,
		"weapon_slot": spent.Slot, "mold": spent.Mold,
		"mold_slot": spent.MoldSlot, "mold_left": spent.MoldAmount,
	})
	plan := []outboundPacket{}
	if consumed {
		// 吃掉了一件装备和一枚模具：必须发权威全量快照（NOTI13）。客户端会先在原格
		// 乐观地画一下，增量 NOTI14 表达不了「那两格现在是空的」，只会留下幽灵叠。
		restore, re := protocol.InventoryRestore(stored.Rows(), stored.Expansion)
		if re != nil {
			return nil, re
		}
		plan = append(plan, outboundPacket{"skin_replicate_consumed", 0, 13, restore})
	}
	// Rebuild the whole tab rather than appending one row: NOTI1545 is
	// count-delimited, so this body is the container's full content. It is also
	// the same body entry re-pushes, which keeps the two paths identical.
	entries := make([]protocol.SkinCargoEntry, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, protocol.SkinCargoEntry{SkinID: id, Extra: 1})
	}
	cargo, e := protocol.SkinCargoInfo(byte(protocol.SkinCargoWeaponShape), entries, nil)
	if e != nil {
		return nil, e
	}
	recent, e := protocol.RecentAddSkinList([]protocol.RecentAddSkinEntry{{Kind: 0, SkinID: template}})
	if e != nil {
		return nil, e
	}
	return append(plan,
		outboundPacket{"skin_cargo_info", 0, 1545, cargo},
		outboundPacket{"skin_recent_add", 0, 1547, recent},
	), nil
}

// skinCargoRestore rebuilds the NOTI1545 the skin storage window needs at entry.
//
// The container only ever got a push alongside the replication confirmation, so
// a reconnect showed an empty storage even though the applied skin was still on
// the character. The saved list is replayed here; the entry packet order in
// entryPayloads.packets() places it after the actor/appearance block.
func skinCargoRestore(state json.RawMessage) ([]byte, error) {
	bag, e := inventory.ReadBag(state)
	if e != nil {
		return nil, e
	}
	ids := bag.WeaponSkinStorage()
	if len(ids) == 0 {
		return nil, nil
	}
	entries := make([]protocol.SkinCargoEntry, 0, len(ids))
	for _, id := range ids {
		entries = append(entries, protocol.SkinCargoEntry{SkinID: id, Extra: 1})
	}
	return protocol.SkinCargoInfo(byte(protocol.SkinCargoWeaponShape), entries, nil)
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
		if errors.Is(e, inventory.ErrWeaponSkinNotUsable) {
			// 本职业戴不上这件外观（仓库里修好复制校验之前留下的条目）：不写档、不换
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
	// The equipped-appearance block is the only channel that drives the world
	// model: slot 12 carries the skin template as both placeholder (the town
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
	}, nil
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
