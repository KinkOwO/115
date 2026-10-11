package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

func graded(grade string) catalog.QuestDefinition {
	return catalog.QuestDefinition{Script: catalog.ScriptRecord{Cells: []pvf.Token{
		{Type: 3, Text: "[grade]"}, {Type: 6, Text: grade},
	}}}
}

// The button may only touch the story chain and the guidance chain. A sweep that
// reached the side and daily quests once cleared 65 records, so the filter is the
// whole safety story and is pinned here.
func TestMainlineIsTheStoryAndGuideGrades(t *testing.T) {
	for _, c := range []struct {
		grade string
		want  bool
	}{{"[epic]", true}, {"[guide]", true}, {"[side]", false}, {"[daily]", false}, {"[dungeon and life]", false}, {"[title]", false}, {"", false}} {
		if got := Mainline(graded(c.grade)); got != c.want {
			t.Fatalf("grade %q: mainline=%v want %v", c.grade, got, c.want)
		}
		if got := MainlineGrades().Allows(c.grade); got != c.want {
			t.Fatalf("grade %q: filter allows=%v want %v", c.grade, got, c.want)
		}
	}
}

// The source marks a quest that pays when it is force-cleared with an empty
// section, and marks nothing otherwise. Five of 2844 rows carry it; the other
// 2839 settle on no experience, no gold and no items.
func TestRewardableOnForceClearReadsAnEmptyMarker(t *testing.T) {
	marked := catalog.QuestDefinition{Script: catalog.ScriptRecord{Cells: []pvf.Token{
		{Type: 3, Text: "[grade]"}, {Type: 6, Text: "[epic]"},
		{Type: 3, Text: "[rewardable on force clear]"},
		{Type: 3, Text: "[reward int data]"}, {Type: 0, Value: 10164787}, {Type: 0, Value: 1},
	}}}
	plain := catalog.QuestDefinition{Script: catalog.ScriptRecord{Cells: []pvf.Token{
		{Type: 3, Text: "[grade]"}, {Type: 6, Text: "[epic]"},
		{Type: 3, Text: "[reward int data]"}, {Type: 0, Value: 10164787}, {Type: 0, Value: 1},
	}}}
	if !RewardableOnForceClear(marked) {
		t.Fatal("the marker was not read")
	}
	if RewardableOnForceClear(plain) {
		t.Fatal("an unmarked quest was treated as rewardable")
	}
	if RewardableOnForceClear(catalog.QuestDefinition{}) {
		t.Fatal("an empty definition was treated as rewardable")
	}
}
