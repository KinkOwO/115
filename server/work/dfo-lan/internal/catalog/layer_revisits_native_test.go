package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestLayerRevisitWitnessedRecordSourceBounds(t *testing.T) {
	r := [18]byte{0, 0, 0, 0, 4, 5, 165, 0, 33, 1}
	if err := validateLayerRevisitRecord(r, 165, 165, 289, 289); err != nil {
		t.Fatal(err)
	}
	for i := range r {
		bad := r
		bad[i]++
		if err := validateLayerRevisitRecord(bad, 165, 165, 289, 289); err == nil {
			t.Fatalf("changed byte %d accepted", i)
		}
	}
	if err := validateLayerRevisitRecord(r, 160, 170, 280, 290); err != nil {
		t.Fatal("valid source range refused", err)
	}
}

func TestLayerRevisitApplyOwnsSourceAndRefusesForeignBase(t *testing.T) {
	position := [2]byte{1, 1}
	c := DungeonCatalog{Source: pvf.ArchiveSnapshot{Checksum: "source"}, Dungeons: map[uint32]DungeonDefinition{1: {ID: 1, Script: ScriptRecord{SHA256: "dgn"}, Mazes: []DungeonMaze{{Index: 0, Quest: 3, Rooms: []DungeonRoom{{X: 1, Y: 1, Map: 2}}, Layers: []DungeonLayer{{Position: position, Maps: []uint32{4}}}}}}}, Maps: map[uint32]ScriptRecord{2: {SHA256: "base"}, 4: {SHA256: "final"}, 5: {SHA256: "foreign"}}}
	var overlay LayerRevisitOverlay
	overlay.Source.Checksum = "source"
	overlay.Scenes = []DungeonLayerRevisit{{Source: "source", Dungeon: 1, Quest: 3, Position: position, Map: 4, ResumeMap: 2, ResumeMapSHA256: "base", Record: [18]byte{0, 0, 0, 0, 4, 5}, DungeonSHA256: "dgn", MapSHA256: "final", ActionSHA256: "act", CinematicSHA256: "cmt", CinematicPath: "scene.cmt"}}
	if err := ApplyLayerRevisits(&c, overlay); err != nil {
		t.Fatal(err)
	}
	overlay.Scenes[0].ResumeMap = 5
	overlay.Scenes[0].ResumeMapSHA256 = "foreign"
	if c.LayerRevisits[0].ResumeMap != 2 {
		t.Fatal("applied overlay shares source data")
	}
	if err := ApplyLayerRevisits(&c, overlay); err == nil {
		t.Fatal("base outside native grid accepted")
	}
	if c.LayerRevisits[0].ResumeMap != 2 {
		t.Fatal("failed apply replaced valid source")
	}
	overlay.Source.Checksum = "foreign"
	if err := ApplyLayerRevisits(&c, overlay); err == nil {
		t.Fatal("foreign archive accepted")
	}
}
