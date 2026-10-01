package inventory

import (
	"dfolan/internal/game/protocol"
	"fmt"
)

// Modelled reports whether a slot belongs to a range this server persists.
// The client's own slot space is wider (its sort table spans 380 slots), so
// anything outside these ranges is carried by the client alone and must not be
// touched by a server side rearrangement.
func (r BagRules) Modelled(slot uint16) bool {
	if r.Quick(slot) {
		return true
	}
	if r.EquipmentSlots != [2]uint16{} && slot >= r.EquipmentSlots[0] && slot <= r.EquipmentSlots[1] {
		return true
	}
	for _, v := range r.Slots {
		if v != [2]uint16{} && slot >= v[0] && slot <= v[1] {
			return true
		}
	}
	return false
}

// SortItems adopts the arrangement the client already applied locally (CMD20).
//
// The table maps each old slot to its new one: an item sitting in slot s ends up
// in slot perm[s]. The mapping is a bijection, so no two items can be sent to the
// same slot.
//
// The direction is what the player sees. With the captured request that moved
// 12..18 and 29, applying the table this way collapses a bag holding
// 9,10,11,13..18,29 into the hole-free 9..18; the opposite direction leaves 13
// empty and keeps a lone item out at 29, which is exactly the "one item jumped
// somewhere else" the player reported. Both directions agree on a self-inverse
// table, which is why the first capture (a four cycle) could not tell them apart.
//
// Both ordinary items and the equipment bag are rearranged: the client's table
// addresses one slot space that covers the equipment range (9..64), the throw
// range (65..120) and the material range (121..176) alike. Special equipment
// spaces are left alone - their list/slot space is not part of this evidence.
func SortItems(b Bag, rules BagRules, r protocol.SortItemRequest) (Bag, error) {
	if r.List != 0 {
		return b, fmt.Errorf("sort item list is not the ordinary inventory")
	}
	if len(r.Slots) == 0 {
		return b, fmt.Errorf("empty sort item table")
	}
	seen := make([]bool, len(r.Slots))
	for _, v := range r.Slots {
		if int(v) >= len(r.Slots) || seen[v] {
			return b, fmt.Errorf("sort item table is not a permutation")
		}
		seen[v] = true
	}
	for idx := range b.Items {
		if e := sortSlot(&b.Items[idx].Slot, r.Slots, rules); e != nil {
			return b, e
		}
	}
	for idx := range b.Equipment {
		if e := sortSlot(&b.Equipment[idx].Slot, r.Slots, rules); e != nil {
			return b, e
		}
	}
	return b, nil
}

// sortSlot maps one occupied slot through the client's arrangement. Slots this
// server does not model are carried by the client alone, and nothing crosses the
// modelled boundary in either direction.
func sortSlot(slot *uint16, perm []uint16, rules BagRules) error {
	old := int(*slot)
	if old >= len(perm) || !rules.Modelled(uint16(old)) {
		return nil
	}
	next := int(perm[old])
	if next == old {
		return nil
	}
	if !rules.Modelled(uint16(next)) {
		return fmt.Errorf("sort item would leave the modelled slots")
	}
	*slot = uint16(next)
	return nil
}
