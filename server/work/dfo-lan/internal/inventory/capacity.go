package inventory

import "fmt"

// OrdinarySlotUnlocked applies the native bag's expansion tier to ordinary
// equipment, consumables, materials, quest and profession tabs. Each tab owns
// 56 physical slots but exposes 40 + 8*tier (DFO.exe 1458D33D0,
// 1458D8A70 and 145AD84A0). Belt and separate inventories use their own limits.
// Existing rows in locked slots remain readable/removable for save compatibility.
func (b Bag) OrdinarySlotUnlocked(slot uint16) bool {
	if slot < 9 || slot > 288 {
		return true
	}
	return b.Expansion <= 2 && (slot-9)%56 < 40+8*uint16(b.Expansion)
}

// EquipmentAwardSlots reserves all requested destinations before the caller
// appends any instances. Both catalog awards and crafted booster grants use it.
func (b Bag) EquipmentAwardSlots(slots [2]uint16, count uint32) ([]uint16, error) {
	if slots[0] == 0 || slots[0] > slots[1] || count == 0 || count > uint32(slots[1])-uint32(slots[0])+1 {
		return nil, fmt.Errorf("invalid equipment award")
	}
	occupied := map[uint16]bool{}
	for _, row := range b.Items {
		occupied[row.Slot] = true
	}
	for _, row := range b.Equipment {
		occupied[row.Slot] = true
	}
	var available []uint16
	for n := uint32(slots[0]); n <= uint32(slots[1]) && uint32(len(available)) < count; n++ {
		slot := uint16(n)
		if b.OrdinarySlotUnlocked(slot) && !occupied[slot] {
			available = append(available, slot)
		}
	}
	if uint32(len(available)) != count {
		return nil, fmt.Errorf("equipment bag is full")
	}
	return available, nil
}
