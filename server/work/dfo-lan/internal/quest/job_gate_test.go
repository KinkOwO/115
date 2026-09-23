package quest

import (
	"dfolan/internal/catalog"
	"testing"
)

func TestAct2JobGate(t *testing.T) {
	c, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	x := BuildIndex(c)
	mainline := x.Entries[3232]
	if mainline == nil || len(mainline.Jobs) != 0 || len(mainline.Prerequisites) != 1 || mainline.Prerequisites[0] != 21001 || !mainline.Implemented || !mainline.RewardUsable {
		t.Fatalf("Act 2 mainline source changed: %+v", mainline)
	}
	if !jobAllowed(mainline.Jobs, "[thief]") || !jobAllowed(mainline.Jobs, "[archer]") {
		t.Fatal("quest 3232 omits [job] and must remain available to every profession")
	}
	branch := x.Entries[3237]
	if branch == nil || !jobAllowed(branch.Jobs, "[thief]") || jobAllowed(branch.Jobs, "[archer]") {
		t.Fatalf("quest 3237 must retain its thief-only job gate: %+v", branch)
	}
}
