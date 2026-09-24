package character

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"testing"
)

// slotTotalFixture builds a learning catalog whose profession-0 rows are all
// active, so applySkillSlotSwaps only trips on the passive rule when a test
// wants it to. The ids are the real ones captured in the 2026-09-23 session.
func slotTotalFixture(ids ...uint16) *Service {
	index := map[byte]map[uint16]LearningDefinition{0: {}}
	for _, id := range ids {
		index[0][id] = LearningDefinition{Job: 0, ID: id, Path: "test", SHA256: "",
			Fields: map[string][]pvf.Token{"[type]": {{Type: 1, Text: "[active]"}}}}
	}
	return &Service{Learning: &LearningCatalog{index: index}}
}

func rowsOf(assign map[uint16]uint16) []protocol.LearnedSkill {
	var rows []protocol.LearnedSkill
	for slot, id := range assign {
		rows = append(rows, protocol.LearnedSkill{ID: id, Level: 1, Slot: slot})
	}
	return rows
}

func slotMap(rows []protocol.LearnedSkill) map[uint16]uint16 {
	out := map[uint16]uint16{}
	for _, r := range rows {
		out[r.Slot] = r.ID
	}
	return out
}

// The two auto-arrange requests captured at 08:35:04.697 and 08:35:17.797
// start from different pre-states yet must converge on the same shortcut bar:
// 0-8 = 77,82,85,87,88,90,92,93,97 with slots 9-13 vacated.
func TestSkillSlotTotalSwapsConverge(t *testing.T) {
	preA := map[uint16]uint16{0: 3, 1: 75, 2: 76, 3: 97, 4: 93, 5: 81, 6: 92, 7: 90, 8: 85,
		9: 82, 10: 88, 11: 87, 12: 86, 13: 83, 22: 77, 27: 84, 32: 89, 34: 91, 37: 94, 38: 95, 39: 96}
	pairsA := []protocol.SkillSlotSwap{{Source: 22, Target: 0}, {Source: 9, Target: 1}, {Source: 8, Target: 2}, {Source: 11, Target: 3}, {Source: 10, Target: 4}, {Source: 7, Target: 5},
		{Source: 10, Target: 7}, {Source: 11, Target: 8}, {Source: 9, Target: 26}, {Source: 10, Target: 28}, {Source: 11, Target: 29}, {Source: 12, Target: 30}, {Source: 13, Target: 31}}
	preB := map[uint16]uint16{0: 97, 1: 93, 2: 81, 3: 92, 4: 90, 5: 85, 6: 82, 7: 88, 8: 87,
		9: 86, 10: 83, 11: 7, 12: 77, 13: 96}
	pairsB := []protocol.SkillSlotSwap{{Source: 12, Target: 0}, {Source: 6, Target: 1}, {Source: 5, Target: 2}, {Source: 8, Target: 3}, {Source: 7, Target: 4}, {Source: 7, Target: 5}, {Source: 8, Target: 6},
		{Source: 8, Target: 7}, {Source: 12, Target: 8}, {Source: 9, Target: 29}, {Source: 10, Target: 30}, {Source: 11, Target: 31}, {Source: 12, Target: 32}, {Source: 13, Target: 33}}

	want := map[uint16]uint16{0: 77, 1: 82, 2: 85, 3: 87, 4: 88, 5: 90, 6: 92, 7: 93, 8: 97}
	for i := uint16(9); i < 14; i++ {
		want[i] = 0
	}
	for name, tc := range map[string]struct {
		pre   map[uint16]uint16
		pairs []protocol.SkillSlotSwap
	}{"frame1": {preA, pairsA}, "frame2": {preB, pairsB}} {
		s := slotTotalFixture(3, 7, 75, 76, 77, 81, 82, 83, 84, 85, 86, 87, 88, 89, 90, 91, 92, 93, 94, 95, 96, 97)
		rows := rowsOf(tc.pre)
		if e := s.applySkillSlotSwaps(0, rows, tc.pairs); e != nil {
			t.Fatalf("%s: %v", name, e)
		}
		got := slotMap(rows)
		for slot, id := range want {
			if got[slot] != id {
				t.Fatalf("%s: slot %d = %d, want %d (full: %v)", name, slot, got[slot], id, got)
			}
		}
	}
}

func TestSkillSlotTotalPassiveGuard(t *testing.T) {
	index := map[byte]map[uint16]LearningDefinition{0: {
		81: {Job: 0, ID: 81, Fields: map[string][]pvf.Token{"[type]": {{Type: 1, Text: "[active]"}}}},
		82: {Job: 0, ID: 82, Fields: map[string][]pvf.Token{"[type]": {{Type: 1, Text: "[passive]"}}}},
	}}
	s := &Service{Learning: &LearningCatalog{index: index}}
	rows := rowsOf(map[uint16]uint16{0: 81, 20: 82})
	// Swapping bar slot 0 with palette 20 would land the passive on the bar.
	if e := s.applySkillSlotSwaps(0, rows, []protocol.SkillSlotSwap{{Source: 20, Target: 0}}); e == nil {
		t.Fatal("passive skill allowed onto the quick bar")
	}
	// Palette-to-palette movement of the passive is fine.
	rows = rowsOf(map[uint16]uint16{20: 82, 21: 81})
	if e := s.applySkillSlotSwaps(0, rows, []protocol.SkillSlotSwap{{Source: 21, Target: 20}}); e != nil {
		t.Fatal(e)
	}
	if m := slotMap(rows); m[20] != 81 || m[21] != 82 {
		t.Fatalf("palette swap misplaced: %v", m)
	}
	// An empty source cannot be swapped.
	rows = rowsOf(map[uint16]uint16{0: 81})
	if e := s.applySkillSlotSwaps(0, rows, []protocol.SkillSlotSwap{{Source: 30, Target: 0}}); e == nil {
		t.Fatal("empty source accepted")
	}
}
