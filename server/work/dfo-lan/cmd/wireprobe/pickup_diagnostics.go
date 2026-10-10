package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/hex"
)

// Record the request and the refusal that was actually written. This does not
// retry a drop, change the bag or assert that the client consumed the reply.
func (client *gameConnection) recordPickupState(kind string, request, reply []byte) {
	w := client.worldState
	if w == nil {
		return
	}
	event := map[string]any{"kind": kind, "id": uint16(43), "character_id": w.role.ID}
	if r, err := protocol.DecodePickup(request); err == nil {
		event["object"] = r.Object
	}
	if reply != nil {
		event["plain_hex"] = hex.EncodeToString(reply)
	}
	if w.activeDungeon != nil {
		event["run"] = w.activeDungeon.RunID
		event["map"] = w.activeDungeon.Room.Map
	}
	if bag, err := inventory.ReadBag(w.role.State); err != nil {
		event["bag_error"] = err.Error()
	} else if w.loot != nil {
		event["expansion"] = bag.Expansion
		occupied := map[uint16]bool{}
		for _, row := range bag.Items {
			occupied[row.Slot] = true
		}
		for _, row := range bag.Equipment {
			occupied[row.Slot] = true
		}
		free := 0
		slots := w.loot.BagRules.EquipmentSlots
		if slots[0] > 0 && slots[0] <= slots[1] {
			for n := uint32(slots[0]); n <= uint32(slots[1]); n++ {
				if bag.OrdinarySlotUnlocked(uint16(n)) && !occupied[uint16(n)] {
					free++
				}
			}
		}
		event["free_equipment_slots"] = free
	}
	client.event(event)
}
