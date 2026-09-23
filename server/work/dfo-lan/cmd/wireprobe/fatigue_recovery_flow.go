package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

func (w *worldSession) recoverFatiguePotion(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.fatigue == nil || w.vault == nil {
		return nil, fmt.Errorf("fatigue recovery before selection")
	}
	slot, e := protocol.DecodeFatigueAction(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, fp, e := w.fatigue.RecoverPotion(ctx, w.role, w.vault.Catalog, slot, time.Now())
	if e != nil {
		return nil, e
	}
	saved.WireID = w.role.WireID
	w.role = saved
	// Both are absolute authoritative updates; no speculative CMD507 ACK.
	fatigue, e := protocol.Fatigue(fp.Used, fp.Limit, fp.UsedMax)
	if e != nil {
		return nil, e
	}
	// Delta includes the spent slot, including an explicit empty row when depleted.
	bag, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	row := protocol.EmptyOrdinaryItem(slot)
	for _, item := range bag.Items {
		if item.Slot == slot {
			row = protocol.OrdinaryItem(slot, item.Template, item.Amount)
		}
	}
	update, e := protocol.InventoryUpdate([][protocol.CurrentItemRecordSize]byte{row})
	if e != nil {
		return nil, e
	}
	return []outboundPacket{{"fatigue_potion_inventory", 0, 14, update}, {"fatigue_potion_recovered", 0, 36, fatigue}}, nil
}
