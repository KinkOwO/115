package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
)

func IsPetContainerMove(b Bag, r protocol.ItemMoveRequest) bool {
	if r.SourceList != 7 && r.DestinationList != 7 {
		return false
	}
	for _, item := range b.PetItems {
		if r.SourceList == 7 && r.SourceSlot == item.Slot || r.DestinationList == 7 && r.DestinationSlot == item.Slot {
			return true
		}
	}
	for _, item := range b.Items {
		if r.SourceList == 0 && r.SourceSlot == item.Slot || r.DestinationList == 0 && r.DestinationSlot == item.Slot {
			return true
		}
	}
	return false
}

func (b Bag) MovePetStackRequest(c catalog.LootCatalog, rules BagRules, r protocol.ItemMoveRequest) (Bag, error) {
	fail := func(msg string) (Bag, error) { return b, fmt.Errorf("%s", msg) }
	if c.Source.Checksum != rules.Source || r.SourceList != 7 && r.DestinationList != 7 ||
		(r.SourceList != 0 && r.SourceList != 7) || (r.DestinationList != 0 && r.DestinationList != 7) ||
		r.Extra != 0 || r.Selection != 0xffffffff || r.Flags != [3]byte{} {
		return fail("invalid pet stack move request")
	}
	find := func(list byte, slot uint16) *BagItem {
		rows := b.Items
		if list == 7 {
			rows = b.PetItems
		}
		for _, row := range rows {
			if row.Slot == slot {
				copy := row
				return &copy
			}
		}
		return nil
	}
	a, z := find(r.SourceList, r.SourceSlot), find(r.DestinationList, r.DestinationSlot)
	matches := func(row *BagItem, template uint32) bool {
		if row == nil {
			return template == 0
		}
		return row.Template == template
	}
	if !matches(a, r.SourceItem) || !matches(z, r.DestinationItem) {
		return fail("stale pet stack move identity")
	}
	fromList, fromSlot, toList, toSlot := r.SourceList, r.SourceSlot, r.DestinationList, r.DestinationSlot
	if r.Count == 0 {
		a, z = z, a
		fromList, fromSlot, toList, toSlot = toList, toSlot, fromList, fromSlot
	}
	if a == nil || fromList == toList && fromSlot == toSlot {
		return fail("empty pet stack move source")
	}
	valid := func(row BagItem, list byte, slot uint16) (uint32, error) {
		def, ok := c.Items[row.Template]
		if !ok || def.Kind != "stackable" {
			return 0, fmt.Errorf("unknown pet stack template")
		}
		if list == 7 {
			if !IsPetConsumable(def.StackableType) || slot < PetConsumableFirst || slot > PetConsumableLast {
				return 0, fmt.Errorf("invalid pet consumable slot")
			}
			for _, equipment := range b.Special[7] {
				if equipment.Slot == slot {
					return 0, fmt.Errorf("pet equipment occupies slot")
				}
			}
		} else {
			home := stackableSlotRange(rules, def.StackableType)
			if slot < home[0] || slot > home[1] {
				return 0, fmt.Errorf("invalid bag stack slot")
			}
			for _, equipment := range b.Equipment {
				if equipment.Slot == slot {
					return 0, fmt.Errorf("bag equipment occupies slot")
				}
			}
		}
		limit := stackLimitFor(rules, def.StackableType, def.StackLimit)
		if row.Amount > limit {
			return 0, fmt.Errorf("pet stack exceeds limit")
		}
		return limit, nil
	}
	cap, err := valid(*a, toList, toSlot)
	if err != nil {
		return b, err
	}
	if _, err = valid(*a, fromList, fromSlot); err != nil {
		return b, err
	}
	amount := r.Count
	if amount == 0 {
		amount = a.Amount
	}
	if amount > a.Amount {
		return fail("pet stack move exceeds source")
	}
	if z != nil && z.Template != a.Template {
		if amount != a.Amount {
			return fail("partial pet stack swap")
		}
		if _, err = valid(*z, fromList, fromSlot); err != nil {
			return b, err
		}
	}
	b.Items = append([]BagItem(nil), b.Items...)
	b.PetItems = append([]BagItem(nil), b.PetItems...)
	put := func(list byte, slot uint16, row *BagItem) {
		rows := &b.Items
		if list == 7 {
			rows = &b.PetItems
		}
		for i := range *rows {
			if (*rows)[i].Slot == slot {
				*rows = append((*rows)[:i], (*rows)[i+1:]...)
				break
			}
		}
		if row != nil && row.Amount > 0 {
			copy := *row
			copy.Slot = slot
			*rows = append(*rows, copy)
		}
	}
	if z != nil && z.Template != a.Template {
		put(fromList, fromSlot, z)
		put(toList, toSlot, a)
		return b, nil
	}
	total := uint64(amount)
	if z != nil {
		if z.ExpireTime != a.ExpireTime {
			return fail("pet stack expiry mismatch")
		}
		total += uint64(z.Amount)
	}
	if total > uint64(cap) {
		return fail("pet stack merge exceeds limit")
	}
	remaining := *a
	remaining.Amount -= amount
	put(fromList, fromSlot, &remaining)
	result := *a
	result.Amount = uint32(total)
	put(toList, toSlot, &result)
	return b, nil
}
