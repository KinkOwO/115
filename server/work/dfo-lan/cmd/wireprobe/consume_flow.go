package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

// useStackable answers CMD44. The client applies the recovery effect itself,
// so the server owns the durable decrement, the acknowledgement echoing the
// consumed slot, and the authoritative bag refresh that follows it.
func (w *worldSession) useStackable(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, fmt.Errorf("item use before character selection")
	}
	r, e := protocol.DecodeUseStackable(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, _, e := w.loot.Consume(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	ack, e := protocol.UseStackableSuccess(r)
	if e != nil {
		return nil, e
	}
	b, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	update, e := protocol.InventoryUpdate(b.Rows())
	if e != nil {
		return nil, e
	}
	w.role = saved
	// The absolute committed bag follows the acknowledgement, so a retried
	// hotkey press cannot leave the client's own count drifting.
	return []outboundPacket{
		{"item_use_ack", 1, 44, ack},
		{"item_use_inventory_updated", 0, 14, update},
	}, nil
}
