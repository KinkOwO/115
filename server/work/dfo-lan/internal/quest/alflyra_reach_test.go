package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
	"testing"
)

func TestAlflyra3252SourceAndTarget(t *testing.T) {
	c, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	d := c.Quests[3252]
	npc, ok := AlflyraReachTarget(d)
	if !ok || npc != 303 {
		t.Fatalf("quest 3252 target changed: %d %t", npc, ok)
	}
	initial, model, err := InitialProgress(d)
	if err != nil || initial != 1 || model != AlflyraReachNPC {
		t.Fatalf("quest 3252 must begin pending: %d %s %v", initial, model, err)
	}
	e := BuildIndex(c).Entries[3252]
	if e == nil || !e.Implemented || e.NPC != 303 || e.Model != AlflyraReachNPC {
		t.Fatalf("quest 3252 absent from positional index: %+v", e)
	}
	locate := func(npc uint32) ([2]uint16, bool) {
		if npc == 303 {
			return [2]uint16{568, 140}, true
		}
		return [2]uint16{}, false
	}
	if nearNPC(npc, storage.WorldPosition{Town: 42, Area: 2, X: 0, Y: 0}, locate) ||
		!nearNPC(npc, storage.WorldPosition{Town: 42, Area: 2, X: 568, Y: 140}, locate) {
		t.Fatal("quest 3252 must advance only at its source NPC")
	}
	if nearNPCWithin(npc, storage.WorldPosition{Town: 42, Area: 2, X: 393, Y: 188}, locate, 80) ||
		!nearNPCWithin(npc, storage.WorldPosition{Town: 42, Area: 2, X: 568, Y: 140}, locate, 80) {
		t.Fatal("quest 3252 must not complete at giver NPC 304")
	}
	d.ObjectiveCells[1].Value = 999
	if _, _, err := InitialProgress(d); err == nil {
		t.Fatal("changed three-cell reach source must remain unimplemented")
	}
	d = c.Quests[3281]
	if _, _, err := InitialProgress(d); err == nil {
		t.Fatal("unverified three-cell reach quest must remain unimplemented")
	}
}
