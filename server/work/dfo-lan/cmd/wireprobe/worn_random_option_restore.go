package main

import "dfolan/internal/inventory"

// The native NOTI14 list-3 reader (1452E9810) applies the temporary
// random-option manager to the actor's existing item at 1452EA0xx, before
// creating a missing item at 1452EA8xx. The ordinary item-data projection
// 14576D8B0 omits record[60:76]. Replay only identified ordinary gear after
// reconstruction so 1451A5560 can apply its options to an existing object.
func appendDungeonWornRandomOptions(plan []outboundPacket, w *worldSession) ([]outboundPacket, error) {
	if w == nil || w.activeDungeon == nil || len(w.role.State) == 0 {
		return plan, nil
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, err
	}
	var items []inventory.BagEquipment
	for _, item := range bag.WornBaseItems() {
		if item.Slot < 12 || item.Slot > 25 {
			continue
		}
		row := inventory.EquipmentRow(item)
		if row[inventory.RandomOptionCountOffset] == 0 {
			continue
		}
		items = append(items, item)
	}
	if len(items) == 0 {
		return plan, nil
	}
	body, err := inventory.EquipmentPayload(3, items, false)
	if err != nil {
		return nil, err
	}
	return append(plan, outboundPacket{"dungeon_worn_random_options_restored", 0, 14, body}), nil
}
