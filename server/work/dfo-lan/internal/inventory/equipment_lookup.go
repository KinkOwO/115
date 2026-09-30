package inventory

// Upgrade commands search the requested container first, then the other one.
// Keep their slot/template matching separate from inheritance's Group filter
// and reinforcement's strict container lookup.
func (b Bag) locateUpgradeEquipment(space byte, slot uint16, template uint32) (byte, []BagEquipment, int) {
	for range 2 {
		items := b.Equipment
		if space == 3 {
			items = b.Worn
		}
		for i, gear := range items {
			if gear.Slot == slot && gear.Template == template {
				return space, items, i
			}
		}
		if space == 3 {
			space = 0
		} else {
			space = 3
		}
	}
	return space, nil, -1
}
