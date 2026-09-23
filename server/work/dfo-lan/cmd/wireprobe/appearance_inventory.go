package main

import (
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
)

// A full mode0 actor rebuild resets the client's worn/avatar containers.
// Restore authoritative instances after it, including options and sockets.
func appearanceInventory(state json.RawMessage) ([]outboundPacket, error) {
	bag, e := inventory.ReadBag(state)
	if e != nil {
		return nil, e
	}
	var plan []outboundPacket
	for _, space := range []byte{1, 7, 3} {
		rows := bag.Special[space]
		if space == 3 {
			// Coexisting clone/look avatars share a body slot; the wire
			// payload carries one row per slot.
			rows = bag.WornBaseItems()
		}
		p, e := inventory.EquipmentPayload(space, rows, true)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"appearance_inventory_restored", 0, 13, p})
	}
	p, e := inventory.WornSpaceUpdate(state)
	if e != nil {
		return nil, e
	}
	plan = append(plan, outboundPacket{"appearance_worn_restored", 0, 14, p})
	return plan, nil
}

func appearanceRestore(service *character.Service, role storage.Character) ([]outboundPacket, error) {
	plan, e := appearanceInventory(role.State)
	if e != nil {
		return nil, e
	}
	addition, e := service.EntryAddition(role)
	if e != nil {
		return nil, e
	}
	skills, e := service.EntrySkills(role)
	if e != nil {
		return nil, e
	}
	variation, e := service.VariationRestore(role)
	if e != nil {
		return nil, e
	}
	plan = append([]outboundPacket{{"appearance_attributes_restored", 0, 2, addition}}, plan...)
	return append(plan,
		outboundPacket{"appearance_skills_restored", 0, 19, skills},
		outboundPacket{"appearance_variations_restored", 1, 29, variation}), nil
}
