package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

// disjointItem answers CMD26 (ENUM_CMDPACKET_DISJOINT_ITEM).
// It deletes requested equipment, awards clear cube fragments into the bag,
// returns the ACK with deleted slots and rewards, followed by NOTI14 to refresh the inventory.
func (w *worldSession) disjointItem(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, fmt.Errorf("disjoint service unavailable")
	}
	r, e := protocol.DecodeDisjointItem(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, _, e := w.loot.Disjoint(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	b, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	ack, e := protocol.DisjointItemSuccess(protocol.DisjointItemResult{
		DeletedSlots: receipt.DeletedSlots,
		List:         0,
		ToolSlot:     receipt.ToolSlot,
		Rewards:      receipt.Rewards,
	})
	if e != nil {
		return nil, e
	}
	update, e := protocol.InventoryUpdate(b.Rows())
	if e != nil {
		return nil, e
	}
	w.role = saved
	return []outboundPacket{
		{"disjoint_item_ack", 1, 26, ack},
		{"disjoint_item_inventory_updated", 0, 14, update},
	}, nil
}
