package inventory

import "testing"

// Live capture 20260912T004320: CMD19 with both lists 0, moving the 6003
// stack out of slot 65 onto belt slot 3. The equipment path refused it and
// the client showed "target inventory is full", so the belt looked broken.
func TestQuickSlotBeltMoves(t *testing.T) {
	rules, e := LoadBagRules("../../configs/inventory.current37.json")
	if e != nil {
		t.Fatal(e)
	}
	if !rules.Quick(3) || rules.Quick(9) || rules.Quick(65) {
		t.Fatal("belt range does not cover the captured slot alone")
	}
	b := Bag{Items: []BagItem{{Slot: 65, Template: 6003, Amount: 15},
		{Slot: 66, Template: 6004, Amount: 5}}}

	onto, e := b.MoveStackable(rules, 65, 3, 6003)
	if e != nil {
		t.Fatal("captured belt move refused:", e)
	}
	if onto.Items[0].Slot != 3 || onto.Items[0].Amount != 15 {
		t.Fatal("stack did not reach the belt intact", onto.Items[0])
	}
	if onto.Items[1].Slot != 66 {
		t.Fatal("an unrelated stack moved")
	}
	back, e := onto.MoveStackable(rules, 3, 65, 6003)
	if e != nil || back.Items[0].Slot != 65 {
		t.Fatal("stack cannot leave the belt", e)
	}
	if _, e = onto.MoveStackable(rules, 3, 66, 6003); e != nil {
		t.Fatal("belt stack refused a swap into an occupied type slot:", e)
	}
	swapped, _ := onto.MoveStackable(rules, 3, 66, 6003)
	if swapped.Items[0].Slot != 66 || swapped.Items[1].Slot != 3 {
		t.Fatal("swap did not exchange both slots", swapped.Items)
	}

	if _, e = b.MoveStackable(rules, 65, 121, 6003); e == nil {
		t.Fatal("a throwable was parked in the material range")
	}
	if _, e = b.MoveStackable(rules, 65, 65, 6003); e == nil {
		t.Fatal("a move onto itself was accepted")
	}
	if _, e = b.MoveStackable(rules, 65, 3, 6004); e == nil {
		t.Fatal("a mismatched template was accepted")
	}
	if _, e = b.MoveStackable(rules, 70, 3, 0); e == nil {
		t.Fatal("an empty source slot was accepted")
	}
	held := Bag{Items: b.Items, Equipment: []BagEquipment{{Slot: 3, Template: 10018}}}
	if _, e = held.MoveStackable(rules, 65, 3, 6003); e == nil {
		t.Fatal("a stack was dropped onto equipment")
	}
}
