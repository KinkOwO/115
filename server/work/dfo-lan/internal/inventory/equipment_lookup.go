package inventory

import "slices"

// findEquipment prefers the requested space, then checks the other space.
// Callers that require an exact container must keep their stricter lookup.
func (b Bag) findEquipment(space byte, slot uint16, template uint32) (byte, []BagEquipment, int) {
	other := byte(3)
	if space == 3 {
		other = 0
	}
	for _, candidate := range []byte{space, other} {
		items := b.Equipment
		if candidate == 3 {
			items = b.Worn
		}
		index := slices.IndexFunc(items, func(gear BagEquipment) bool {
			return gear.Slot == slot && gear.Template == template
		})
		if index >= 0 {
			return candidate, items, index
		}
	}
	return other, nil, -1
}
