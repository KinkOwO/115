package progression

import (
	"dfolan/internal/catalog"
	"testing"
)

// Quest completion pays gold from [gold reward table], which every earlier
// build ignored - live capture 20260912T011900 shows no quest crediting gold.
// The value must track the level table and stay zero where experience is zero.
func TestQuestGoldTracksLevelTable(t *testing.T) {
	c, e := catalog.LoadProgression("../../configs/progression.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	q, e := catalog.LoadQuests("../../configs/quests.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	paid, zero, higher := 0, 0, 0
	var prevLevel byte
	var prevGold uint32
	for id, d := range q.Quests {
		gold, err := QuestGold(c, d, byte(max32(int(d.MinimumLevel), 1)))
		if err != nil {
			continue
		}
		exp, _ := QuestExperience(c, d, byte(max32(int(d.MinimumLevel), 1)))
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
