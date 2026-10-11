package inventory

import (
	"dfolan/internal/game/protocol"
	"sort"
)

// DeletedTemplate marks a row that removes the slot's object client-side.
// The 115 NOTI14 reader (sub_1452E9810) compares the incoming row template
// with the existing slot object's template: equal keeps the update branch;
// different deletes the old object and, only when the row template is
// 0xFFFFFFFF, skips constructing a replacement (sub_1452ead22's
// `cmp [rbp+0x4b2], -1` guard). A plain zero template would delete the old
// object and then rebuild an empty one, losing the slot's icon while also
// marking it "newly obtained" (the build path sets the notice flag).
const DeletedTemplate = 0xFFFFFFFF

// ChangedItemRows returns the incremental NOTI14 rows that take the client
// from before to after without flashing every existing item:
//   - gold/coin rows only when the value changed;
//   - stackable rows only for slots whose template/amount/expiry changed;
//   - equipment rows only for slots whose encoded 181-byte row changed;
//   - rows for slots that disappeared are delete rows (template 0xFFFFFFFF);
//   - rows are sorted by slot, matching Rows().
//
// Slots that are untouched are simply omitted: the client reader updates only
// the slots present in the packet, so omitted slots keep their objects and
// never fire the new-item glow.
func ChangedItemRows(before, after Bag) [][protocol.CurrentItemRecordSize]byte {
	var rows [][protocol.CurrentItemRecordSize]byte

	if before.Gold != after.Gold {
		rows = append(rows, protocol.OrdinaryItem(0, 0, after.Gold))
	}
	if before.Coin != after.Coin {
		rows = append(rows, protocol.OrdinaryItem(1, 1, after.Coin))
	}

	beforeItems := map[uint16]BagItem{}
	for _, it := range before.Items {
		beforeItems[it.Slot] = it
	}
	afterItems := map[uint16]BagItem{}
	for _, it := range after.Items {
		afterItems[it.Slot] = it
	}
	for slot, it := range afterItems {
		old, ok := beforeItems[slot]
		if !ok || old.Template != it.Template || old.Amount != it.Amount || old.ExpireTime != it.ExpireTime {
			rows = append(rows, protocol.OrdinaryItem(slot, it.Template, it.Amount, it.ExpireTime))
		}
	}
	for slot := range beforeItems {
		if _, ok := afterItems[slot]; !ok {
			rows = append(rows, deleteRow(slot))
		}
	}

	beforeEquip := map[uint16]BagEquipment{}
	for _, it := range before.Equipment {
		beforeEquip[it.Slot] = it
	}
	afterEquip := map[uint16]BagEquipment{}
	for _, it := range after.Equipment {
		afterEquip[it.Slot] = it
	}
	for slot, it := range afterEquip {
		old, ok := beforeEquip[slot]
		if !ok || EquipmentRow(old) != EquipmentRow(it) {
			rows = append(rows, EquipmentRow(it))
		}
	}
	for slot := range beforeEquip {
		if _, ok := afterEquip[slot]; !ok {
			rows = append(rows, deleteRow(slot))
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		return uint16(rows[i][0])|uint16(rows[i][1])<<8 < uint16(rows[j][0])|uint16(rows[j][1])<<8
	})
	return rows
}

func deleteRow(slot uint16) [protocol.CurrentItemRecordSize]byte {
	return protocol.EmptyOrdinaryItem(slot)
}
