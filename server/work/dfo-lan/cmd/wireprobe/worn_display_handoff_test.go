package main

import "testing"

func TestHandoffWornVisualsAfterEntry(t *testing.T) {
	p := entryPayloads{WornUpdate: []byte{3, 1, 0}}.packets()
	// The worn visual refresh is followed by the actor appearance frame, and
	// the character option block (NOTI2827) is appended after those when one
	// is configured, because this client crashes on town entry when 2827
	// arrives early.
	end := len(p)
	if len(p) > 0 && p[len(p)-1].ID == 2827 {
		end--
	}
	if end < 1 || p[end-1].Kind != 0 || p[end-1].ID != 2 {
		t.Fatal("actor appearance must be the final data frame")
	}
	complete := -1
	for i, q := range p[:end] {
		if q.ID == 124 {
			complete = i
		}
	}
	if complete < 0 || complete >= end-1 {
		t.Fatal("entry completion must precede the post-barrier refresh")
	}
	// Third pass (20260921): the id-14 slot-update frame for the worn set
	// must sit between the worn restore and the worn-window refresh — the
	// equip-change heal always emits this frame, entry previously never did,
	// and the arrow level-compare degenerates without it.
	slot := entryPayloads{WornSlots: []byte{3, 0}, WornUpdate: []byte{3, 1, 0}}.packets()
	slotIdx, winIdx, wornIdx := -1, -1, -1
	for i, q := range slot {
		switch q.Name {
		case "equipment_slots_updated_entry":
			slotIdx = i
		case "worn_equipment_window_refreshed_entry":
			winIdx = i
		case "worn_equipment_restored_after_barrier":
			wornIdx = i
		}
	}
	if slotIdx < 0 || wornIdx < 0 || winIdx < 0 || !(wornIdx < slotIdx && slotIdx < winIdx) {
		t.Fatal("worn slot-update frame must sit between the worn restore and the window refresh")
	}
	locked := entryPayloads{WornUpdate: []byte{3, 1, 0}, SkillLocks: []byte{1, 2, 3}}.packets()
	if last := locked[len(locked)-1]; last.Kind != 0 || last.ID != 2827 {
		t.Fatal("the character option block must be the final entry frame")
	}
	// Weapon-toast fix (20260923, attempt 1/3): the id-14 worn rows and the
	// worn-window refresh must also be emitted before the NOTI24 AREA_USERS
	// frame, so the 38250 "Weapon not equipped." refresh chain sees slot 12
	// installed. The after-barrier copies remain for the upgrade arrows.
	pre := entryPayloads{WornSlots: []byte{3, 0}, WornUpdate: []byte{3, 1, 0}}.packets()
	preWorn, preSlot, preWin, area := -1, -1, -1, -1
	for i, q := range pre {
		switch q.Name {
		case "worn_equipment_restored":
			preWorn = i
		case "equipment_slots_updated_entry_prelude":
			preSlot = i
		case "worn_equipment_window_refreshed_entry_prelude":
			preWin = i
		case "town_entry_probe_sent":
			area = i
		}
	}
	if preWorn < 0 || preSlot < 0 || preWin < 0 || area < 0 || !(preWorn < preSlot && preSlot < preWin && preWin < area) {
		t.Fatal("id-14 prelude frames must sit after the worn restore and before NOTI24 area-users")
	}
}
