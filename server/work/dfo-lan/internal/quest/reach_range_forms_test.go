package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/storage"
	"testing"
)

func TestReachRangeSourceForms(t *testing.T) {
	c, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	x := BuildIndex(c)
	counts := map[int]int{}
	for id, d := range c.Quests {
		if d.Kind != "[reach the range]" {
			continue
		}
		counts[len(d.ObjectiveCells)]++
		e := x.Entries[id]
		if e == nil || !e.Implemented || e.Initial != 1 {
			t.Errorf("quest %d reach objective unavailable: %+v", id, e)
			continue
		}
		switch len(d.ObjectiveCells) {
		case 3:
			if e.Model != ReachNPC || e.NPCReach.NPC == 0 || e.NPCReach.W <= 0 || e.NPCReach.H <= 0 {
				t.Errorf("quest %d invalid NPC reach: %+v", id, e)
			}
		case 6:
			if e.Model != SingleReachRange || e.Range.Town == 0 || e.Range.W <= 0 || e.Range.H <= 0 {
				t.Errorf("quest %d invalid map rectangle: %+v", id, e)
			}
		default:
			t.Errorf("quest %d unexpected reach length %d", id, len(d.ObjectiveCells))
		}
	}
	if counts[3] != 20 || counts[6] != 89 || len(counts) != 2 {
		t.Fatalf("reach source distribution changed: %v", counts)
	}
	if got := x.Entries[3281]; got.Model != ReachNPC || got.NPCReach != (NPCReachObjective{NPC: 39, W: 250, H: 250}) {
		t.Fatalf("quest 3281 mismatch: %+v", got)
	}
	if got := x.Entries[3281]; !got.RewardUsable || !got.GrowUsable || got.MinimumLevel != 50 ||
		!prerequisitesMet(got.PrerequisiteGroups, map[uint32]string{3513: "completed"}) {
		t.Fatalf("quest 3281 still fails a non-objective availability gate: %+v", got)
	}
	if got := x.Entries[3252]; got.Model != ReachNPC || got.NPCReach != (NPCReachObjective{NPC: 303, W: 1000, H: 1000}) {
		t.Fatalf("quest 3252 persisted model/source mismatch: %+v", got)
	}
	if got := x.Entries[13748]; got.Model != SingleReachRange || got.Range.Y != -240 {
		t.Fatalf("quest 13748 negative origin rejected: %+v", got)
	}
}

func TestReachGeometryAndUnsupportedShapes(t *testing.T) {
	locate := func(npc uint32) ([2]uint16, bool) {
		if npc == 39 {
			return [2]uint16{821, 799}, true
		}
		return [2]uint16{}, false
	}
	r := NPCReachObjective{NPC: 39, W: 250, H: 100}
	for _, tc := range []struct {
		x, y uint16
		want bool
	}{
		{821, 799, true}, {946, 849, true}, {947, 799, false}, {821, 850, false},
	} {
		if got := nearNPCReach(r, storage.WorldPosition{Town: 43, Area: 0, X: tc.x, Y: tc.y}, locate); got != tc.want {
			t.Errorf("NPC rectangle (%d,%d): got %v want %v", tc.x, tc.y, got, tc.want)
		}
	}
	mapRect := RangeObjective{Town: 139, Area: 0, X: 0, Y: -240, W: 2000, H: 640}
	if !mapRect.Contains(storage.WorldPosition{Town: 139, Area: 0, X: 700, Y: 220}) ||
		mapRect.Contains(storage.WorldPosition{Town: 139, Area: 1, X: 700, Y: 220}) {
		t.Fatal("negative-origin map rectangle does not match source area")
	}
}

func TestNPCDistanceMultiplierExpandsOnlyNPCGeometry(t *testing.T) {
	locate := func(npc uint32) ([2]uint16, bool) {
		if npc == 100000374 {
			return [2]uint16{515, 114}, true
		}
		return [2]uint16{}, false
	}
	point := storage.WorldPosition{Town: 40, Area: 3, X: 630, Y: 160}
	reach := NPCReachObjective{NPC: 100000374, W: 200, H: 100}
	t.Setenv("DFO_QUEST_NPC_DISTANCE_MULTIPLIER", "")
	if nearNPCReach(reach, point, locate) {
		t.Fatal("default NPC range unexpectedly reaches beyond its source width")
	}
	t.Setenv("DFO_QUEST_NPC_DISTANCE_MULTIPLIER", "2")
	if !nearNPCReach(reach, point, locate) || !nearNPC(100000374, storage.WorldPosition{X: 850, Y: 114}, locate) {
		t.Fatal("double NPC distance did not expand range and dialogue proximity")
	}
	for _, invalid := range []string{"0", "-2", "NaN", "+Inf", "bad"} {
		t.Setenv("DFO_QUEST_NPC_DISTANCE_MULTIPLIER", invalid)
		if NPCDistanceMultiplier() != 1 || nearNPCReach(reach, point, locate) {
			t.Fatalf("invalid multiplier %q changed the source range", invalid)
		}
	}
	mapRect := RangeObjective{Town: 40, Area: 3, X: 0, Y: 0, W: 100, H: 100}
	t.Setenv("DFO_QUEST_NPC_DISTANCE_MULTIPLIER", "2")
	if mapRect.Contains(point) {
		t.Fatal("NPC multiplier changed the independent map rectangle")
	}
}
