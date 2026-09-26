package catalog

import "testing"

func TestNativeNPCMoveAndEpisodeReturnIndex(t *testing.T) {
	w, err := LoadWorld("../../configs/world.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(w.NPCMoves) < 200 {
		t.Fatalf("too few native NPC move roles: %d", len(w.NPCMoves))
	}
	for _, tc := range []struct {
		from, to uint32
		quests   []uint32
	}{
		{618, 619, []uint32{8646, 8647, 8648, 8649, 8650}},
		{619, 618, nil},
		{353, 100000257, []uint32{6061}},
		{100000257, 353, nil},
	} {
		var found *NPCMove
		for i := range w.NPCMoves {
			if w.NPCMoves[i].NPCID == tc.from {
				found = &w.NPCMoves[i]
				break
			}
		}
		if found == nil || found.TargetNPC != tc.to {
			t.Fatalf("NPC %d target: %+v", tc.from, found)
		}
		if len(found.Quests) != len(tc.quests) {
			t.Fatalf("NPC %d quest guard: %+v", tc.from, found.Quests)
		}
		for i, q := range tc.quests {
			if found.Quests[i] != q {
				t.Fatalf("NPC %d quest guard: %+v", tc.from, found.Quests)
			}
		}
	}
	for town, expected := range map[uint32]NPCPlace{55: {38, 0}, 75: {22, 4}, 82: {22, 4}, 149: {6, 2}} {
		if w.EpisodeReturns[town] != expected {
			t.Fatalf("episode town %d return: %+v", town, w.EpisodeReturns[town])
		}
	}
}
