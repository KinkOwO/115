package inventory

import (
	"fmt"
	"math"
	"strings"
)

func normalizeStackableType(s string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(s, "`", "")))
}

// stackableSlotRange maps a stackable type string to its bag slot range.
// If not specified or absent from rules, defaults to the throw/consumables range [65, 120].
func stackableSlotRange(r BagRules, stackableType string) [2]uint16 {
	norm := normalizeStackableType(stackableType)
	if rng, ok := r.Slots[norm]; ok && rng != [2]uint16{} {
		return rng
	}
	switch {
	case strings.HasPrefix(norm, "[material]") && strings.HasSuffix(norm, "4"):
		return [2]uint16{345, 359}
	case strings.HasPrefix(norm, "[material]"):
		return [2]uint16{121, 176}
	case strings.HasPrefix(norm, "[quest]"):
		return [2]uint16{177, 232}
	case strings.HasPrefix(norm, "[material expert job]"):
		return [2]uint16{233, 288}
	case strings.HasPrefix(norm, "[avatar emblem]"):
		return [2]uint16{289, 344}
	default:
		if rng, ok := r.Slots["[throw]"]; ok && rng != [2]uint16{} {
			return rng
		}
		return [2]uint16{65, 120}
	}
}

// Buy adds an item purchased from an NPC shop into the bag.
// NPC shop items (templates 1-3175) are catalog-independent: if stackableType is
// unspecified or not in catalog, they default to the throw range [65, 120] (consumables).
func (b Bag) Buy(r BagRules, template, count, cost uint32, stackableType ...string) (Bag, uint16, error) {
	if template == 0 || count == 0 {
		return b, 0, fmt.Errorf("invalid buy parameters")
	}
	if b.Gold < cost {
		return b, 0, fmt.Errorf("insufficient gold: need %d, have %d", cost, b.Gold)
	}

	st := ""
	if len(stackableType) > 0 {
		st = stackableType[0]
	}
	slots := stackableSlotRange(r, st)
	limit := r.MissingStackLimit
	if limit == 0 {
		limit = 1000
	}
	if count > limit {
		return b, 0, fmt.Errorf("buy count exceeds stack limit")
	}

	occupied := map[uint16]bool{}
	for _, eq := range b.Equipment {
		occupied[eq.Slot] = true
	}

	// 1. Try stacking into an existing slot of the same template within the target range
	b.Items = append([]BagItem(nil), b.Items...)
	for i, it := range b.Items {
		occupied[it.Slot] = true
		if it.Template == template && it.Slot >= slots[0] && it.Slot <= slots[1] {
			if uint64(it.Amount)+uint64(count) <= uint64(limit) {
				b.Items[i].Amount += count
				b.Gold -= cost
				return b, it.Slot, nil
			}
		}
	}

	// 2. Allocate the first unoccupied slot in the category range
	for n := uint32(slots[0]); n <= uint32(slots[1]); n++ {
		slot := uint16(n)
		if !occupied[slot] {
			b.Items = append(b.Items, BagItem{Slot: slot, Template: template, Amount: count})
			b.Gold -= cost
			return b, slot, nil
		}
	}

	return b, 0, fmt.Errorf("bag category is full")
}

// Sell sells an item from the bag by its slot and inventory list type.
// Dispatch rules:
// - equipment_slots [9,64] -> b.Equipment (unworn equipment in bag)
// - throw [65,120] / material [121,176] -> b.Items (stackables)
// - b.Worn (equipped gear) is rejected
// Slot overlap resolution: equipment slots 12-25 overlap with worn slots 12-25.
// Equipment range checks b.Equipment first; only if absent from b.Equipment does it check Worn.
func (b Bag) Sell(r BagRules, list byte, slot uint16, unitPrice uint32) (Bag, uint32, uint32, error) {
	if list != 0 {
		return b, 0, 0, fmt.Errorf("unsupported inventory list %d", list)
	}
	if slot == 0 {
		return b, 0, 0, fmt.Errorf("invalid sell slot")
	}

	goldGained := unitPrice
	if goldGained == 0 {
		goldGained = 1
	}
	if uint64(b.Gold)+uint64(goldGained) > math.MaxUint32 {
		return b, 0, 0, fmt.Errorf("gold overflow")
	}

	eqSlots := r.EquipmentSlots
	if eqSlots == [2]uint16{} {
		eqSlots = [2]uint16{9, 64}
	}

	// 1. If slot is within the equipment range [9, 64], check b.Equipment first.
	if slot >= eqSlots[0] && slot <= eqSlots[1] {
		for i, eq := range b.Equipment {
			if eq.Slot == slot {
				template := eq.Template
				b.Equipment = append([]BagEquipment(nil), b.Equipment...)
				b.Equipment = append(b.Equipment[:i], b.Equipment[i+1:]...)
				b.Gold += goldGained
				return b, template, goldGained, nil
			}
		}
		// If not in b.Equipment, check if worn at slot 12-25
		for _, w := range b.Worn {
			if w.Slot == slot {
				return b, 0, 0, fmt.Errorf("cannot sell worn equipment at slot %d", slot)
			}
		}
	}

	// 2. Check stackable items in b.Items
	for i, it := range b.Items {
		if it.Slot == slot {
			template := it.Template
			b.Items = append([]BagItem(nil), b.Items...)
			if it.Amount <= 1 {
				b.Items = append(b.Items[:i], b.Items[i+1:]...)
			} else {
				b.Items[i].Amount--
			}
			b.Gold += goldGained
			return b, template, goldGained, nil
		}
	}

	// 3. Fallback check for worn slots
	for _, w := range b.Worn {
		if w.Slot == slot {
			return b, 0, 0, fmt.Errorf("cannot sell worn equipment at slot %d", slot)
		}
	}

	return b, 0, 0, fmt.Errorf("no such owned bag slot %d", slot)
}
