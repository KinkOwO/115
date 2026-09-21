package inventory

import (
	"dfolan/internal/game/protocol"
	"reflect"
	"sort"
	"testing"
)

// sortRules mirrors the shipped bag ranges: quick belt 0..8, equipment bag
// 9..64, throw 65..120, material 121..176.
func sortRules() BagRules {
	return BagRules{
		QuickSlots:     [2]uint16{0, 8},
		EquipmentSlots: [2]uint16{9, 64},
		Slots:          map[string][2]uint16{"[throw]": {65, 120}, "[material]": {121, 176}},
	}
}

func permutation(n int, swaps ...[2]int) []uint16 {
	p := make([]uint16, n)
	for i := range p {
		p[i] = uint16(i)
	}
	for _, s := range swaps {
		p[s[0]], p[s[1]] = p[s[1]], p[s[0]]
	}
	return p
}

// Adopting the arrangement moves both items of a swap and leaves everything
// else where it is.
func TestSortItemsAppliesThePermutation(t *testing.T) {
	b := Bag{Items: []BagItem{
		{Slot: 18, Template: 111, Amount: 1},
		{Slot: 21, Template: 222, Amount: 2},
		{Slot: 30, Template: 333, Amount: 3},
	}}
	got, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 0, Slots: permutation(380, [2]int{18, 21})})
	if e != nil {
		t.Fatal(e)
	}
	want := map[uint16]uint32{18: 222, 21: 111, 30: 333}
	for _, i := range got.Items {
		if want[i.Slot] != i.Template {
			t.Fatalf("slot %d holds template %d, want %d", i.Slot, i.Template, want[i.Slot])
		}
	}
	if len(got.Items) != 3 {
		t.Fatalf("item count changed: %d", len(got.Items))
	}
}

// Slots the server does not model are carried by the client alone: the table
// covers 380 slots but this server owns 0..176.
func TestSortItemsLeavesUnmodelledSlotsAlone(t *testing.T) {
	b := Bag{Items: []BagItem{{Slot: 200, Template: 999, Amount: 1}}}
	got, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 0, Slots: permutation(380, [2]int{200, 201})})
	if e != nil {
		t.Fatal(e)
	}
	if got.Items[0].Slot != 200 {
		t.Fatalf("unmodelled slot moved to %d", got.Items[0].Slot)
	}
}

// A rearrangement must never push a modelled item into(or out of)the space the
// server does not own.
func TestSortItemsRefusesCrossingTheModelledBoundary(t *testing.T) {
	b := Bag{Items: []BagItem{{Slot: 30, Template: 111, Amount: 1}}}
	if _, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 0, Slots: permutation(380, [2]int{30, 200})}); e == nil {
		t.Fatal("item moved outside the modelled slots")
	}
	other := Bag{Items: []BagItem{{Slot: 200, Template: 111, Amount: 1}}}
	if _, e := SortItems(other, sortRules(), protocol.SortItemRequest{List: 0, Slots: permutation(380, [2]int{30, 200})}); e != nil {
		t.Fatalf("unmodelled source slot must be skipped, got %v", e)
	}
}

func TestSortItemsRejectsForeignListAndTables(t *testing.T) {
	b := Bag{Items: []BagItem{{Slot: 18, Template: 111, Amount: 1}}}
	if _, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 2, Slots: permutation(380)}); e == nil {
		t.Fatal("foreign list accepted")
	}
	if _, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 0}); e == nil {
		t.Fatal("empty table accepted")
	}
	short := permutation(20) // slot 18 exists, but the table stops short of it
	if _, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 0, Slots: short}); e != nil {
		t.Fatalf("short table over modelled slot rejected: %v", e)
	}
	bad := []uint16{0, 0}
	if _, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 0, Slots: bad}); e == nil {
		t.Fatal("non permutation accepted")
	}
}

// The captured request that moved slots 11..22 was rearranging equipment: only
// Bag.Equipment held those slots, so the equipment bag must be sorted too. An
// item in slot s moves to perm[s].
func TestSortItemsRearrangesTheEquipmentBag(t *testing.T) {
	// Non-identity entries of the permutation captured on 2026-09-21 15:13.
	perm := permutation(380)
	for _, s := range [][2]int{
		{11, 19}, {12, 11}, {13, 12}, {14, 13}, {15, 14}, {16, 15},
		{17, 21}, {18, 22}, {19, 16}, {21, 17}, {22, 18},
	} {
		perm[s[0]] = uint16(s[1])
	}
	b := Bag{Equipment: []BagEquipment{
		{Slot: 11, Template: 111},
		{Slot: 15, Template: 222},
		{Slot: 17, Template: 333},
		{Slot: 22, Template: 444},
	}}
	got, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 0, Slots: perm})
	if e != nil {
		t.Fatal(e)
	}
	want := map[uint16]uint32{19: 111, 14: 222, 21: 333, 18: 444}
	for _, i := range got.Equipment {
		if want[i.Slot] != i.Template {
			t.Fatalf("equipment slot %d holds template %d, want %d", i.Slot, i.Template, want[i.Slot])
		}
	}
	if len(got.Equipment) != 4 {
		t.Fatalf("equipment count changed: %d", len(got.Equipment))
	}
}

// The captured request that moved 12..18 and 29 must collapse the bag: it held
// 9,10,11,13..18,29 before, and adopting the arrangement leaves the hole-free
// 9..18. Applying the table in the opposite direction instead leaves 13 empty and
// keeps a lone item out at 29 - the "one item jumped somewhere else" the player
// saw, so this test also pins the direction.
func TestSortItemsCollapsesTheBagWithCapturedPermutation(t *testing.T) {
	perm := permutation(380)
	for _, s := range [][2]int{{12, 29}, {13, 12}, {14, 13}, {15, 14}, {16, 15}, {17, 16}, {18, 17}, {29, 18}} {
		perm[s[0]] = uint16(s[1])
	}
	var b Bag
	for _, slot := range []uint16{9, 10, 11, 13, 14, 15, 16, 17, 18, 29} {
		b.Equipment = append(b.Equipment, BagEquipment{Slot: slot, Template: uint32(slot)})
	}
	got, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 0, Slots: perm})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Equipment) != 10 {
		t.Fatalf("equipment count changed: %d", len(got.Equipment))
	}
	slots := make([]uint16, 0, len(got.Equipment))
	for _, i := range got.Equipment {
		slots = append(slots, i.Slot)
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
	want := []uint16{9, 10, 11, 12, 13, 14, 15, 16, 17, 18}
	if !reflect.DeepEqual(slots, want) {
		t.Fatalf("arranged slots = %v, want %v", slots, want)
	}
}
