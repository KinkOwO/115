package catalog

import "testing"

func TestCurrentGrandFloresQuestPrerequisites(t *testing.T) {
	c, e := LoadQuests("../../configs/quests.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	for id, pre := range map[uint32]uint32{4873: 3145, 3146: 4873, 3147: 3146, 3148: 3147, 3149: 3148, 3150: 3149} {
		d := c.Quests[id]
		if len(d.Prerequisites) != 1 || d.Prerequisites[0] != pre {
			t.Fatalf("quest%d source prerequisite missing: %v", id, d.Prerequisites)
		}
	}
	if c.Quests[4873].MinimumLevel != 5 || c.Quests[4873].Kind != "[meet npc]" {
		t.Fatal("next quest source level/type changed")
	}
}
