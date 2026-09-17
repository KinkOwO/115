package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"fmt"
)

// Count-zero bag operations use the same empty-first layout as warehouse
// reposition. Validate both endpoints so a stale client icon cannot mint items.
func (b Bag) MoveStackRequest(c catalog.LootCatalog, rules BagRules, r protocol.ItemMoveRequest) (Bag, error) {
	fail := func(msg string) (Bag, error) { return b, fmt.Errorf("%s", msg) }
	if c.Source.Checksum != rules.Source || r.SourceList != 0 || r.DestinationList != 0 || r.SourceSlot == r.DestinationSlot || r.Extra != 0 || r.Selection != 0xffffffff || r.Flags != [3]byte{} {
		return fail("invalid bag move request")
	}
	find := func(slot uint16) *BagItem {
		for _, v := range b.Items {
			if v.Slot == slot {
				x := v
				return &x
			}
		}
		return nil
	}
	a, z := find(r.SourceSlot), find(r.DestinationSlot)
	matches := func(v *BagItem, id uint32) bool {
		if v == nil {
			return id == 0
		}
		return id == v.Template
	}
	if !matches(a, r.SourceItem) || !matches(z, r.DestinationItem) {
		return fail("stale bag move identity")
	}
	from, to := r.SourceSlot, r.DestinationSlot
	if r.Count == 0 {
		a, z = z, a
		from, to = to, from
	}
	if a == nil {
		return fail("empty bag move source")
	}
	limit := func(v BagItem, slot uint16) (uint32, error) {
		item, ok := c.Items[v.Template]
		if !ok || item.Kind != "stackable" {
			return 0, fmt.Errorf("unknown stack definition")
		}
		home, ok := rules.Slots[item.StackableType]
		if !ok {
			switch item.StackableType {
			case "[etc]", "[waste]", "[hp]", "[mp]", "[hp mp]", "[expert town potion]":
				home = [2]uint16{65, 120}
			default:
				return 0, fmt.Errorf("unknown stack category")
			}
		}
		if slot == 0 || !(slot >= home[0] && slot <= home[1] || item.StackableType != "[material]" && rules.Quick(slot)) {
			return 0, fmt.Errorf("invalid stack destination")
		}
		for _, gear := range b.Equipment {
			if gear.Slot == slot {
				return 0, fmt.Errorf("equipment occupies stack destination")
			}
		}
		n := item.StackLimit
		if n == 0 {
			n = rules.MissingStackLimit
		}
		if n == 0 || v.Amount > n {
			return 0, fmt.Errorf("invalid stack amount")
		}
		return n, nil
	}
	cap, e := limit(*a, to)
	if e != nil {
		return b, e
	}
	if _, e = limit(*a, from); e != nil {
		return b, e
	}
	amount := r.Count
	if amount == 0 {
		amount = a.Amount
	}
	if amount > a.Amount {
		return fail("stack move exceeds source")
	}
	b.Items = append([]BagItem{}, b.Items...)
	put := func(slot uint16, id, n uint32) {
		kept := make([]BagItem, 0, len(b.Items)+1)
		for _, x := range b.Items {
			if x.Slot != slot {
				kept = append(kept, x)
			}
		}
		if n > 0 {
			kept = append(kept, BagItem{slot, id, n})
		}
		b.Items = kept
	}
	if z != nil && z.Template != a.Template {
		if amount != a.Amount {
			return fail("partial stack swap")
		}
		if _, e = limit(*z, from); e != nil {
			return b, e
		}
		put(from, z.Template, z.Amount)
		put(to, a.Template, a.Amount)
	} else {
		total := uint64(amount)
		if z != nil {
			total += uint64(z.Amount)
		}
		if total > uint64(cap) {
			return fail("stack merge exceeds limit")
		}
		put(from, a.Template, a.Amount-amount)
		put(to, a.Template, uint32(total))
	}
	return b, nil
}
