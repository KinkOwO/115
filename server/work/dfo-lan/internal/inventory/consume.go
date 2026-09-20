package inventory

import (
	"dfolan/internal/catalog"
	"fmt"
)

// Consume removes one unit of a stackable from an exact bag slot.
//
// The slot, the identity and the amount are all checked against what the bag
// actually holds: a request cannot name a slot it does not own, an item that
// is not there, or spend a stack it has already emptied. An emptied slot is
// removed rather than left at zero, which is how every other bag path here
// represents "absent".
func (b Bag) Consume(c catalog.LootCatalog, slot uint16, template uint32) (Bag, uint32, error) {
	if slot == 0 || template == 0 {
		return b, 0, fmt.Errorf("invalid consume request")
	}
	if slot == 1 && template == 1 {
		if b.Coin == 0 {
			return b, 0, fmt.Errorf("coin stack is already empty")
		}
		b.Coin--
		return b, b.Coin, nil
	}
	item, known := c.Items[template]
	if !known {
		for _, row := range b.Items {
			if row.Slot == slot && row.Template == template {
				known = true
				item = catalog.LootItem{ID: template, Kind: "stackable"}
				break
			}
		}
	}
	if !known {
		return b, 0, fmt.Errorf("item is absent from the imported source")
	}
	if item.Kind != "stackable" {
		return b, 0, fmt.Errorf("only a source stackable can be consumed")
	}
	rows := append([]BagItem(nil), b.Items...)
	for i, row := range rows {
		if row.Slot != slot {
			continue
		}
		if row.Template != template {
			return b, 0, fmt.Errorf("slot holds a different item")
		}
		if row.Amount == 0 {
			return b, 0, fmt.Errorf("slot stack is already empty")
		}
		remaining := row.Amount - 1
		if remaining == 0 {
			rows = append(rows[:i:i], rows[i+1:]...)
		} else {
			rows[i].Amount = remaining
		}
		b.Items = rows
		return b, remaining, nil
	}
	return b, 0, fmt.Errorf("no such owned bag slot")
}
