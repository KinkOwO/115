package progression

import (
	"dfolan/internal/catalog"
	"testing"
)

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
	q, e := catalog.LoadQuests("../../configs/quests.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	absent, folded, refused := 0, 0, 0
	var firstRefusal error
	for id, d := range q.Quests {
		difficulty := section(d.Script.Cells, "[difficulty]")
		gain, err := QuestExperience(c, d, 20)
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
