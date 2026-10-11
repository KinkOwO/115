package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"
	"log"
)

// oathSelectionPackets handles the current client's two C2S2382 forms.
// Player clicks mutate the option ledger; scene bootstrap only reads it.
func (w *worldSession) oathSelectionPackets(ctx context.Context, body []byte) ([]outboundPacket, error) {
	if w == nil || w.characters == nil || w.store == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("oath selection requires a selected character")
	}
	r, err := protocol.DecodeOathSystemRequest(body)
	if err != nil {
		return nil, err
	}
	if r.ExplicitSelection && w.activeDungeon != nil {
		return nil, fmt.Errorf("oath option can only be changed in town")
	}
	var current struct {
		Level  int
		ItemID uint32
		Option int
	}
	if r.ExplicitSelection {
		// The client submits the group's representative item (14057d140 ->
		// 1401b7000 -> 14057d100), which can have a different rarity from
		// the worn core. Persist against the actual core, retaining the
		// database's atomic ownership/level/slot check.
		equipped, err := w.store.EquippedOathSelection(ctx, w.account, w.role.ID)
		if err != nil {
			return nil, err
		}
		if !w.sameOathSelectionGroup(equipped.ItemID, r.ItemID) {
			return nil, fmt.Errorf("requested oath representative %d does not match equipped core %d", r.ItemID, equipped.ItemID)
		}
		selected, err := w.store.SelectEquippedOathOption(ctx, w.account, w.role.ID, equipped.ItemID, int(r.Option))
		if err != nil {
			return nil, err
		}
		current.Level, current.ItemID, current.Option = selected.Level, selected.ItemID, selected.Option
	} else {
		selected, err := w.store.EquippedOathSelection(ctx, w.account, w.role.ID)
		if err != nil {
			return nil, err
		}
		// Bootstrap carries cached UI data. It is read-only and must restore
		// the authoritative selection even after a core change.
		current.Level, current.ItemID, current.Option = selected.Level, selected.ItemID, selected.Option
	}
	info, err := w.oathInfo(ctx, current.Level, current.Option)
	if err != nil {
		return nil, err
	}
	ack := outboundPacket{"oath_option_selection_accepted", 1, 2382, []byte{1}}
	snapshot := outboundPacket{"oath_system_info_refreshed", 0, 2839, info}
	if r.ExplicitSelection {
		plan := []outboundPacket{ack}
		plan = append(plan, w.oathPointPackets()...)
		return append(plan, snapshot), nil
	}
	return []outboundPacket{snapshot, ack}, nil
}

// sameOathSelectionGroup uses the already prepared native oath point table.
// Both items must be cores: crystals also have rows in that table.
func (w *worldSession) sameOathSelectionGroup(equipped, requested uint32) bool {
	if equipped == 0 || requested == 0 {
		return false
	}
	if equipped == requested {
		return true
	}
	if w.items == nil || w.items.Equipment == nil || w.items.Points == nil {
		return false
	}
	for _, id := range []uint32{equipped, requested} {
		d, err := w.items.Equipment.Definition(id)
		if err != nil || len(d.Fields["[equipment type]"]) == 0 || d.Fields["[equipment type]"][0].Text != "[oath]" {
			return false
		}
	}
	group := func(id uint32) (int32, bool) {
		var value int32
		found := false
		for _, row := range w.items.Points.Oath.Rules {
			if row.Template != id || row.PartSetIndex < 0 {
				continue
			}
			if found && value != row.PartSetIndex {
				return 0, false
			}
			value, found = row.PartSetIndex, true
		}
		return value, found
	}
	a, okA := group(equipped)
	b, okB := group(requested)
	return okA && okB && a == b
}

// oathDisplayRefresh must follow item updates and NOTI2634. The native
// NOTI2839 handler (1405757f0) recalculates the oath and invokes 14053a280
// on the open inventory window; sending it before points redraws old data.
func (w *worldSession) oathDisplayRefresh(ctx context.Context, name string) []outboundPacket {
	if w == nil || w.store == nil || w.role.ID == 0 {
		return nil
	}
	selection, err := w.store.EquippedOathSelection(ctx, w.account, w.role.ID)
	if err != nil {
		log.Printf("oath display refresh: %v", err)
		return nil
	}
	info, err := w.oathInfo(ctx, selection.Level, selection.Option)
	if err != nil {
		log.Printf("oath display refresh: %v", err)
		return nil
	}
	return []outboundPacket{{name, 0, 2839, info}}
}

func moveTouchesOath(r protocol.ItemMoveRequest) bool {
	return (r.SourceList == 3 && r.SourceSlot >= 36 && r.SourceSlot <= 47) ||
		(r.DestinationList == 3 && r.DestinationSlot >= 36 && r.DestinationSlot <= 47)
}

func (w *worldSession) refreshOathAfterEquipment(ctx context.Context, plan []outboundPacket, r protocol.ItemMoveRequest) []outboundPacket {
	if len(plan) == 0 || !moveTouchesOath(r) {
		return plan
	}
	// Clone/worn restoration can rebuild the actor after the move handler's
	// point update. Move that update to the end instead of sending it twice.
	ordered := make([]outboundPacket, 0, len(plan)+2)
	for _, packet := range plan {
		if packet.Name != "oath_part_set_point" {
			ordered = append(ordered, packet)
		}
	}
	ordered = append(ordered, w.oathPointPackets()...)
	return append(ordered, w.oathDisplayRefresh(ctx, "oath_system_info_after_wear")...)
}

// dungeonOathSelectionPacket re-reads the authoritative selection after the
// dungeon actor and its worn items have been rebuilt. Scene bootstrap requests
// are not guaranteed to arrive, so the loading sequence must publish it too.
func (w *worldSession) dungeonOathSelectionPacket(ctx context.Context) (outboundPacket, error) {
	selected, err := w.store.EquippedOathSelection(ctx, w.account, w.role.ID)
	if err != nil {
		return outboundPacket{}, err
	}
	info, err := w.oathInfo(ctx, selected.Level, selected.Option)
	if err != nil {
		return outboundPacket{}, err
	}
	return outboundPacket{"dungeon_oath_selection_restored", 0, 2839, info}, nil
}
