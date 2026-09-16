package progression

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestCurrentQuestExperienceAndJobRewards(t *testing.T) {
	c, e := catalog.LoadProgression("../../configs/progression.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	q, e := catalog.LoadQuests("../../configs/quests.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	d := q.Quests[3145]
	for _, level := range []byte{1, 2, 8, 13} {
		gain, e := QuestExperience(c, d, level)
		if e != nil || gain != 1200 {
			t.Fatal(level, gain, e)
		}
	}
	for _, tc := range []struct {
		job, grow byte
		want      bool
	}{{0, 0, false}, {0, 1, true}, {0, 17, true}, {16, 0, false}, {16, 5, true}, {16, 7, false}} {
		got, e := SelectedItemRewards(d.RewardCells, tc.job, tc.grow)
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
	gold := []pvf.Token{{Type: 0, Value: 0}, {Type: 0, Value: 100}}
	if yes, e := SelectedItemRewards(gold, 0, 0); e != nil || !yes {
		t.Fatal("currency must not vanish", e)
	}
	if _, e := SelectedItemRewards(gold[:1], 0, 0); e == nil {
		t.Fatal("truncated reward accepted")
	}
	// The current numeric difficulty table includes multi-character keys.
	d.Script.Cells = append([]pvf.Token(nil), d.Script.Cells...)
	for i := range d.Script.Cells {
		if d.Script.Cells[i].Type == 3 && d.Script.Cells[i].Text == "[difficulty]" {
			d.Script.Cells[i+1].Text = "20"
			break
		}
	}
	gain, e := QuestExperience(c, d, 1)
	if e != nil || gain != 1100 {
		t.Fatal("multi-digit difficulty not resolved", gain, e)
	}
}
