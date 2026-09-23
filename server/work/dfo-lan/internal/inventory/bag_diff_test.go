package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"testing"
)

func TestChangedItemRowsOnlyChangedSlots(t *testing.T) {
	before := Bag{Version: "ordinary-bag-v1", Gold: 100, Coin: 5, Items: []BagItem{
		{Slot: 2, Template: 6001, Amount: 5},   // untouched
		{Slot: 3, Template: 6003, Amount: 5},   // amount changes
		{Slot: 65, Template: 9001, Amount: 1},  // removed
	}, Equipment: []BagEquipment{
		{Slot: 9, Template: 101, Durability: 10, Record: []byte{9}},        // untouched
		{Slot: 11, Template: 102, Durability: 20, Record: []byte{11}},      // durability changes
		{Slot: 13, Template: 103, Durability: 30, Record: []byte{13}},      // removed
	}}

	after := Bag{Version: "ordinary-bag-v1", Gold: 250, Coin: 5, Items: []BagItem{
		{Slot: 2, Template: 6001, Amount: 5},   // untouched -> omitted
		{Slot: 3, Template: 6003, Amount: 8},   // amount changed
		{Slot: 66, Template: 9002, Amount: 1},  // added
	}, Equipment: []BagEquipment{
		{Slot: 9, Template: 101, Durability: 10, Record: []byte{9}},   // untouched -> omitted
		{Slot: 11, Template: 102, Durability: 25, Record: []byte{11}}, // changed
		{Slot: 12, Template: 104, Durability: 1, Record: []byte{12}},  // added
	}}

	rows := ChangedItemRows(before, after)

	got := map[uint16][2]uint32{}
	for _, r := range rows {
		slot := binary.LittleEndian.Uint16(r[:])
		tpl := binary.LittleEndian.Uint32(r[2:])
		got[slot] = [2]uint32{tpl, binary.LittleEndian.Uint32(r[6:])}
	}

	want := map[uint16][2]uint32{
		0:  {0, 250},    // gold changed
		3:  {6003, 8},   // amount changed
		11: {102, 0},    // equipment durability changed (template stays, no amount)
		12: {104, 0},    // added equipment (no amount on equipment rows)
		13: {DeletedTemplate, 0}, // removed equipment
		65: {DeletedTemplate, 0}, // removed stackable
		66: {9002, 1},   // added stackable
	}

	if len(rows) != len(want) {
		t.Fatalf("row count = %d, want %d: %v", len(rows), len(want), got)
	}
	for slot, w := range want {
		g, ok := got[slot]
		if !ok {
			t.Fatalf("missing slot %d row: %v", slot, got)
		}
		if g[0] != w[0] || g[1] != w[1] {
			t.Fatalf("slot %d row = %v, want %v", slot, g, w)
		}
	}
	// untouched slot 2 and 9 must be absent entirely.
	if _, ok := got[2]; ok {
		t.Fatalf("untouched slot 2 must be omitted, got %v", got[2])
	}
	if _, ok := got[9]; ok {
		t.Fatalf("untouched slot 9 must be omitted, got %v", got[9])
	}
	// slot 11 must carry the new durability in its row.
	var slot11 [protocol.CurrentItemRecordSize]byte
	for _, r := range rows {
		if binary.LittleEndian.Uint16(r[:]) == 11 {
			slot11 = r
		}
	}
	if d := binary.LittleEndian.Uint16(slot11[11:]); d != 25 {
		t.Fatalf("slot 11 durability = %d, want 25", d)
	}
	// slots must be sorted ascending.
	for i := 1; i < len(rows); i++ {
		a := binary.LittleEndian.Uint16(rows[i-1][:])
		b := binary.LittleEndian.Uint16(rows[i][:])
		if a >= b {
			t.Fatalf("rows not sorted at %d (%d >= %d)", i, a, b)
		}
	}
}

func TestChangedItemRowsEmptyDelta(t *testing.T) {
	before := Bag{Version: "ordinary-bag-v1", Gold: 100, Items: []BagItem{{Slot: 2, Template: 6001, Amount: 5}}}
	after := before
	if rows := ChangedItemRows(before, after); len(rows) != 0 {
		t.Fatalf("identical bags must yield no rows, got %d", len(rows))
	}
}

func TestChangedItemRowsExpiryChangeEmitted(t *testing.T) {
	before := Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 2, Template: 6001, Amount: 5}}}
	after := Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 2, Template: 6001, Amount: 5, ExpireTime: 1700000000}}}
	rows := ChangedItemRows(before, after)
	if len(rows) != 1 {
		t.Fatalf("expiry change must emit a row, got %d", len(rows))
	}
	if exp := binary.LittleEndian.Uint32(rows[0][56:]); exp != 1700000000 {
		t.Fatalf("expiry = %d, want 1700000000", exp)
	}
}

func TestChangedItemRowsEquipmentRecordChangeEmitted(t *testing.T) {
	before := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: 101, Record: make([]byte, protocol.CurrentItemRecordSize)}}}
	after := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: 101, Record: make([]byte, protocol.CurrentItemRecordSize)}}}
	after.Equipment[0].Durability = 7
	rows := ChangedItemRows(before, after)
	if len(rows) != 1 {
		t.Fatalf("durability change must emit a row, got %d", len(rows))
	}
	if slot := binary.LittleEndian.Uint16(rows[0][:]); slot != 9 {
		t.Fatalf("row slot = %d, want 9", slot)
	}
	if d := binary.LittleEndian.Uint16(rows[0][11:]); d != 7 {
		t.Fatalf("durability = %d, want 7", d)
	}
}
