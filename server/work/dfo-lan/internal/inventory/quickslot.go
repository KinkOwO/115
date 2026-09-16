package inventory

import "fmt"

// MoveStackable moves one stack between bag slots, which is how the client
// fills the quick-use belt: live capture 20260912T004320 shows CMD19 asking
// to put the 6003 stack in slot 65 into slot 3, with both list fields 0.
//
// Only stacks move here. Equipment keeps its own path in wear.go, and a
// destination holding equipment is refused rather than silently swapped with
// a stack, because the two are stored in different arrays and a swap across
// them would change what each slot range means.
//
// The destination must be a quick slot or inside the range the source item's
// own type occupies, so an item can reach the belt and return to where it
// belongs, but cannot be parked in another type's range.
func (b Bag) MoveStackable(rules BagRules, from, to uint16, template uint32) (Bag, error) {
	if from == to {
		return b, fmt.Errorf("identical stack locations")
	}
	source := -1
	target := -1
	for i, v := range b.Items {
		if v.Slot == from {
			source = i
		}
		if v.Slot == to {
			target = i
		}
	}
	if source < 0 {
		return b, fmt.Errorf("source slot holds no stack")
	}
	if template != 0 && b.Items[source].Template != template {
		return b, fmt.Errorf("stack identity does not match the request")
	}
	if b.Items[source].Amount == 0 {
		return b, fmt.Errorf("source stack is empty")
	}
	for _, v := range b.Equipment {
		if v.Slot == to {
			return b, fmt.Errorf("destination slot holds equipment")
		}
	}
	if !rules.Quick(to) && !returnsHome(rules, from, to) {
		return b, fmt.Errorf("destination slot is outside the belt and the item's own range")
	}
	moved := append([]BagItem(nil), b.Items...)
	moved[source].Slot = to
	if target >= 0 {
		moved[target].Slot = from
	}
	b.Items = moved
	return b, nil
}


// returnsHome reports whether a move that is not onto the belt stays inside
// the type range the stack already sits in - or, for a stack coming back off
// the belt, lands in one of the type ranges at all. The belt is outside every
// type range, so a stack on it has no range of its own to compare against;
// what keeps that case honest is that the stack has to have been placed there
// from a legal slot in the first place.
func returnsHome(rules BagRules, from, to uint16) bool {
	for _, v := range rules.Slots {
		if from >= v[0] && from <= v[1] {
			return to >= v[0] && to <= v[1]
		}
	}
	if !rules.Quick(from) {
		return false
	}
	for _, v := range rules.Slots {
		if to >= v[0] && to <= v[1] {
			return true
		}
	}
	return false
}
