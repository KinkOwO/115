package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestAwakeningQuestTargetCharacter(t *testing.T) {
	c, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	x := BuildIndex(c)
	for id, entry := range x.Entries {
		if !entry.TargetUsable {
			t.Fatalf("quest %d has unsupported target character rows", id)
		}
	}
	tests := []struct {
		id                 uint32
		job                string
		advancement, stage byte
		want               bool
	}{
		{21956, "[swordman]", 1, 1, true},  // Omniblade
		{21956, "[swordman]", 2, 1, false}, // Dark Lord branch
		{21956, "[fighter]", 1, 1, false},
		{21956, "[swordman]", 1, 0, false},
		{21960, "[swordman]", 2, 2, true}, // Dark Lord
		{21976, "[fighter]", 1, 1, true},  // Nen Empress
		{21976, "[swordman]", 1, 1, false},
		{3232, "[archer]", 0, 0, true}, // Unrestricted mainline
	}
	for _, tc := range tests {
		e := x.Entries[tc.id]
		if e == nil || !e.TargetUsable {
			t.Fatalf("quest %d has invalid target rows: %+v", tc.id, e)
		}
		if got := targetCharacterAllowed(e.TargetCharacters, e.NonTargetCharacters, tc.job, tc.advancement, tc.stage); got != tc.want {
			t.Errorf("quest %d target (%s, %d, %d): got %v, want %v", tc.id, tc.job, tc.advancement, tc.stage, got, tc.want)
		}
	}
}

// TestNonTargetCharacterGate: a [non target character] row excludes the matched
// profession outright while leaving every other profession on the quest. This
// is the reverse face of the [target character] gate and was ignored until the
// Silent City faction lead-ins offered two spellings of one quest to the same
// character.
func TestNonTargetCharacterGate(t *testing.T) {
	rows, ok := nonTargetCharacters([]pvf.Token{
		{Type: 3, Text: "[non target character]"},
		{Type: 6, Text: "[at swordman]"},
		{Type: 0, Value: -1},
		{Type: 0, Value: -1},
		{Type: 3, Text: "[/non target character]"},
	})
	if !ok || len(rows) != 1 {
		t.Fatalf("unexpected non target rows: %+v, %v", rows, ok)
	}
	if targetCharacterAllowed(nil, rows, "[at swordman]", 0, 0) {
		t.Fatal("[non target character] must exclude the named profession")
	}
	if !targetCharacterAllowed(nil, rows, "[swordman]", 0, 0) {
		t.Fatal("an unrelated profession must stay allowed")
	}
	if !targetCharacterAllowed(nil, rows, "[all]", 0, 0) {
		t.Fatal("an unrelated profession must stay allowed")
	}
	if targetCharacterAllowed(nil, rows, "[at swordman]", 5, 3) {
		t.Fatal("the wildcard rows exclude every advancement of the named job")
	}
}

func TestNonTargetCharacterMalformedIsUnusable(t *testing.T) {
	rows, ok := nonTargetCharacters([]pvf.Token{
		{Type: 3, Text: "[non target character]"},
		{Type: 6, Text: "[at swordman]"},
		{Type: 0, Value: 1},
		{Type: 3, Text: "[/non target character]"},
	})
	if ok || len(rows) != 0 {
		t.Fatalf("malformed non target rows accepted: %+v, %v", rows, ok)
	}
}

func TestTargetCharacterMalformedIsUnusable(t *testing.T) {
	rows, ok := targetCharacters([]pvf.Token{
		{Type: 3, Text: "[target character]"},
		{Type: 6, Text: "[swordman]"},
		{Type: 0, Value: 1},
		{Type: 3, Text: "[/target character]"},
	})
	if ok || len(rows) != 0 {
		t.Fatalf("malformed target rows accepted: %+v, %v", rows, ok)
	}
}
