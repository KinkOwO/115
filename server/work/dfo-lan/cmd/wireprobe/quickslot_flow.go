package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

// moveStack answers the CMD19 moves that carry a stack rather than a piece of
// equipment: dragging a consumable onto the quick-use belt and back. It
// reports whether it recognised the request, so the equipment path keeps
// every move it already handles.
func (w *worldSession) moveStack(rules inventory.BagRules,
	r protocol.ItemMoveRequest) ([]outboundPacket, bool, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, false, nil
	}
	if r.SourceList != 0 || r.DestinationList != 0 {
		return nil, false, nil
	}
	b, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, false, nil
	}
	// The dragged stack sits in the slot the client names as the destination;
	// the source fields carry the belt slot it is going to.
	carries := false
	for _, v := range b.Items {
		if v.Slot == r.DestinationSlot {
			carries = true
		}
	}
	if !carries {
		return nil, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, _, e := w.loot.MoveStack(ctx, w.role, rules, r)
	if e != nil {
		return nil, true, e
	}
	updated, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, true, e
	}
	update, e := protocol.InventoryUpdate(updated.Rows())
	if e != nil {
		return nil, true, e
	}
	w.role = saved
	if len(update) == 0 {
		return nil, true, fmt.Errorf("empty inventory update")
	}
	return []outboundPacket{
		{"stack_move_committed", 1, 19, protocol.ItemMoveSuccess(r, 1)},
		{"stack_move_inventory_updated", 0, 14, update},
	}, true, nil
}
