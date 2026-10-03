package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/testfixture"
	"testing"
)

func TestCurrentQuestExperienceAndJobRewards(t *testing.T) {
	c, e := catalog.LoadProgression("../../configs/progression.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	q, e := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if e != nil {
		t.Fatal(e)
	}
	d := q.Quests[3145]
	for _, level := range []byte{1, 2, 8, 13} {
		gain, e := GrowthQuestExperience(c, d, level)
		if e != nil || gain != 1200 {
			t.Fatal(level, gain, e)
		}
	}
	for _, tc := range []struct {
		job, grow byte
		want      bool
	}{{0, 0, false}, {0, 1, true}, {0, 17, true}, {16, 0, false}, {16, 5, true}, {16, 7, false}} {
		got, e := GrowthSelectedItemRewards(d.RewardCells, tc.job, tc.grow)
		if e != nil || got != tc.want {
			t.Fatal(tc, got, e)
		}
	}
	gold := []pvf.Token{{Type: 0, Value: 0}, {Type: 0, Value: 100}}
	if yes, e := GrowthSelectedItemRewards(gold, 0, 0); e != nil || !yes {
		t.Fatal("currency must not vanish", e)
	}
	if _, e := GrowthSelectedItemRewards(gold[:1], 0, 0); e == nil {
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
	gain, e := GrowthQuestExperience(c, d, 1)
	if e != nil || gain != 1100 {
		t.Fatal("multi-digit difficulty not resolved", gain, e)
	}
}

// Quest completion pays gold from [gold reward table], which every earlier
// build ignored - live capture 20260912T011900 shows no quest crediting gold.
// The value must track the level table and stay zero where experience is zero.
func TestQuestGoldTracksLevelTable(t *testing.T) {
	c, e := catalog.LoadProgression("../../configs/progression.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	q, e := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if e != nil {
		t.Fatal(e)
	}
	paid, zero, higher := 0, 0, 0
	var prevLevel byte
	var prevGold uint32
	for id, d := range q.Quests {
		gold, err := GrowthQuestGold(c, d, byte(max32(int(d.MinimumLevel), 1)))
		if err != nil {
			continue
		}
		exp, _ := GrowthQuestExperience(c, d, byte(max32(int(d.MinimumLevel), 1)))
		if gold > 0 {
			paid++
			if exp == 0 {
				t.Fatalf("quest %d pays gold %d but no experience", id, gold)
			}
			// A higher-level quest of the same difficulty should not pay less.
			if d.MinimumLevel > uint32(prevLevel) && gold >= prevGold {
				higher++
			}
			prevLevel, prevGold = byte(d.MinimumLevel), gold
		} else {
			zero++
		}
	}
	t.Logf("quests paying gold: %d, paying none: %d", paid, zero)
	if paid == 0 {
		t.Fatal("no quest pays any gold - the table is not being read")
	}
	if higher == 0 {
		t.Fatal("gold never rises with level - the table index is wrong")
	}
}

func max32(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Live capture 20260912T004120 refused quest 2109 three times with
// "unsupported quest difficulty" and it could never be handed in. 347 of this
// build's quests carry no [difficulty] section, and another 81 spell their key
// in lower case while the table spells every key upper. Neither shape may cost
// the player the quest.
func TestQuestDifficultyCoverage(t *testing.T) {
	c, e := catalog.LoadProgression("../../configs/progression.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	q, e := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if e != nil {
		t.Fatal(e)
	}
	absent, folded, refused := 0, 0, 0
	var firstRefusal error
	for id, d := range q.Quests {
		difficulty := growthSection(d.Script.Cells, "[difficulty]")
		gain, err := GrowthQuestExperience(c, d, 20)
		if err != nil {
			// Other source shapes are still allowed to refuse; only the two
			// difficulty shapes must not.
			if len(difficulty) == 0 || len(difficulty) == 1 && difficulty[0].Type == 6 {
				if err.Error() == "unsupported quest difficulty" ||
					err.Error() == "quest difficulty absent from source" {
					refused++
					if firstRefusal == nil {
						firstRefusal = err
						t.Logf("quest %d refused: %v", id, err)
					}
				}
			}
			continue
		}
		if len(difficulty) == 0 {
			absent++
			if gain != 0 {
				t.Fatalf("quest %d declares no difficulty but gained %d", id, gain)
			}
			continue
		}
		text := difficulty[0].Text
		if text != "" && text >= "a" && text <= "z" {
			folded++
		}
	}
	if refused != 0 {
		t.Fatalf("%d quests still refused on difficulty (%v)", refused, firstRefusal)
	}
	if absent == 0 || folded == 0 {
		t.Fatalf("catalog no longer exercises both shapes: absent=%d folded=%d", absent, folded)
	}
	t.Logf("settled %d quests with no difficulty and %d with a lower-case key", absent, folded)
}

func TestSourceClearBaseAndReferenceRankFormula(t *testing.T) {
	c, e := catalog.LoadProgression("../../configs/progression.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	d, e := catalog.LoadDungeons(testfixture.DungeonPath(t, "dungeons.generated.json"))
	if e != nil {
		t.Fatal(e)
	}
	got, e := GrowthDungeonClear(c, d.Dungeons[3], 0, 50)
	// Current level3 base126, decimal rate1.3, rank50 crosses four thresholds.
	// The explicit reference formula floors base to163 and its15% score to24.
	if e != nil || got.Base != 163 || got.Score != 24 || got.Grade != 50 {
		t.Fatal(got, e)
	}
	c.DifficultyRates[0] = 2
	got, e = GrowthDungeonClear(c, d.Dungeons[3], 0, 50)
	if e != nil || got.Base != 252 || got.Score != 37 {
		t.Fatal("rate did not follow source", got, e)
	}
	if _, e = GrowthDungeonClear(c, d.Dungeons[3], 255, 50); e == nil {
		t.Fatal("unknown difficulty accepted")
	}
}
