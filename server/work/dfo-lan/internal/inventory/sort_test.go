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

// 徽章区（289..360）整理回归：连续无洞时必须幂等。旧实现把 289..320 推成
// 321..352，下一次又弹回来，玩家每点一次整理整段跳 32 格（2026-10-07 实测）。
func TestSortItemsKeepsContiguousEmblemsInPlace(t *testing.T) {
	b := Bag{}
	for s := uint16(289); s <= 320; s++ {
		b.Items = append(b.Items, BagItem{Slot: s, Template: 2500000 + uint32(s), Amount: 1})
	}
	got, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 0, Slots: permutation(380)})
	if e != nil {
		t.Fatal(e)
	}
	for _, i := range got.Items {
		if i.Slot < 289 || i.Slot > 320 {
			t.Fatalf("emblem pushed out of 289..320: slot %d", i.Slot)
		}
	}
}

// 徽章被旧实现平移走后（321..352），幂等压缩应把它拉回 289..320。
func TestSortItemsCompactsDisplacedEmblems(t *testing.T) {
	b := Bag{}
	for s := uint16(321); s <= 352; s++ {
		b.Items = append(b.Items, BagItem{Slot: s, Template: 2500000 + uint32(s), Amount: 1})
	}
	got, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 0, Slots: permutation(380)})
	if e != nil {
		t.Fatal(e)
	}
	slots := make([]uint16, 0, len(got.Items))
	for _, i := range got.Items {
		slots = append(slots, i.Slot)
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i] < slots[j] })
	want := make([]uint16, 0, 32)
	for s := uint16(289); s <= 320; s++ {
		want = append(want, s)
	}
	if !reflect.DeepEqual(slots, want) {
		t.Fatalf("displaced emblems = %v, want 289..320", slots)
	}
}

// 客户端对徽章区发出的排列必须被采纳（真机帧 list=0，old[321..352] -> new 289..320
// 的一个乱序映射）。徽章区不在 BagRules 里，旧实现会整段跳过。
func TestSortItemsAdoptsEmblemPermutation(t *testing.T) {
	// 真机 2026-10-07 22:54 帧：perm[321+i] 的值（i=0..31）。
	targets := []uint16{
		296, 289, 298, 311, 301, 320, 319, 297, 290, 305, 312, 306,
		309, 302, 318, 300, 310, 303, 295, 308, 313, 315, 304, 291,
		317, 314, 292, 299, 294, 293, 307, 316,
	}
	perm := permutation(380)
	b := Bag{}
	for i := 0; i < 32; i++ {
		old := uint16(321 + i)
		b.Items = append(b.Items, BagItem{Slot: old, Template: 5000000 + uint32(old), Amount: 1})
		perm[old] = targets[i]
		// 真机帧里 289..320 同时指向 321..352（保持全表双射）。
		perm[289+i] = uint16(321 + i)
	}
	got, e := SortItems(b, sortRules(), protocol.SortItemRequest{List: 0, Slots: perm})
	if e != nil {
		t.Fatal(e)
	}
	if len(got.Items) != 32 {
		t.Fatalf("emblem count changed: %d", len(got.Items))
	}
	// 每个徽章必须留在徽章区内（位置由客户端排列决定，压缩后再连续）。
	seen := map[uint32]bool{}
	for _, it := range got.Items {
		if it.Slot < 289 || it.Slot > 320 {
			t.Fatalf("emblem tpl %d adopted to %d, outside emblem zone", it.Template, it.Slot)
		}
		seen[it.Template] = true
	}
	for i := 0; i < 32; i++ {
		tpl := 5000000 + uint32(321+i)
		if !seen[tpl] {
			t.Fatalf("emblem tpl %d lost", tpl)
		}
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
