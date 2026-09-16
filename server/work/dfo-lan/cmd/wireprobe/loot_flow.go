package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

func (w *worldSession) pickup(p []byte) ([]outboundPacket, error) {
	if w.loot == nil || w.activeDungeon == nil {
		return nil, fmt.Errorf("pickup service/run unavailable")
	}
	r, e := protocol.DecodePickup(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, e := w.loot.Pickup(ctx, w.role, w.drops, w.activeDungeon, r)
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
	plan := []outboundPacket{{"pickup_ack", 1, 43, []byte{1}}}
	// NOTI39's parser changes branch after the scene object has been removed.
	// Replaying a gold tail there would corrupt the ordinary fallback cursor.
	if applied {
		body, e := protocol.PickupConfirmed(r.Object, w.role.WireID, receipt.Destination, receipt.Award.Template == 0)
		if receipt.Award.Template == 0 {
			body, e = protocol.GoldPickupConfirmed(r.Object, w.role.WireID, receipt.Award.Amount)
		}
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"pickup_scene_removed", 0, 39, body})
	}
	// NOTI39 applies the displayed delta first; the absolute committed balance
	// follows it, avoiding double-counting and repairing retried pickups.
	plan = append(plan, outboundPacket{"pickup_inventory_updated", 0, 14, update})
	w.role = saved
	return plan, nil
}
