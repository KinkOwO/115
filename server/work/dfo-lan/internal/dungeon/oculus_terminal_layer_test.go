package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestOculusClosingCinematicReusesFinalLayer(t *testing.T) {
	const finalMap uint32 = 100000294
	monsters := []protocol.DungeonMonster{
		{Entity: 0x1021, Template: 109010976, Team: 100},
		{Entity: 0x1022, Template: 109010748, Team: 100},
	}
	objective := []protocol.DungeonMonster{{Entity: 0x1010, Team: 100}}
	c := catalog.DungeonCatalog{
		Maps: map[uint32]catalog.ScriptRecord{finalMap: {SHA256: "map"}},
		TerminalScenes: []catalog.DungeonTerminalScene{{
			Source: "source", Dungeon: 291100432, Maze: 0, Quest: 12147,
			Position: [2]byte{1, 2}, ObjectiveMap: 292106830, FinalMap: finalMap,
			XMin: 741, XMax: 741, YMin: 346, YMax: 346,
			DungeonSHA256: "dungeon", MapSHA256: "map",
		}},
	}
	c.Source.Checksum = "source"
	s := &Session{
		Definition: catalog.DungeonDefinition{ID: 291100432, Script: catalog.ScriptRecord{SHA256: "dungeon"}},
		Maze: catalog.DungeonMaze{Quest: 12147, Layers: []catalog.DungeonLayer{
			{Position: [2]byte{1, 2}, Maps: []uint32{finalMap}},
		}},
		Room:     catalog.DungeonRoom{X: 1, Y: 2, Map: finalMap, Boss: true},
		Loaded:   true,
		Monsters: monsters,
		Visited:  map[uint32][]protocol.DungeonMonster{292106830: objective, finalMap: monsters},
		Dead:     map[uint16]bool{0x1010: true},
	}
	r := protocol.DungeonRoomTransition{
		Dungeon: 291100432, Position: [2]byte{1, 2}, LayerChange: true,
		Record: [18]byte{0, 0, 0, 0, 4, 5, 0xe5, 0x02, 0x5a, 0x01},
	}
	if s.RoomCleared() {
		t.Fatal("source cinematic actors unexpectedly counted as a cleared room")
	}
	next, err := s.MoveScene(c, r)
	if err != nil || next.Room.Map != finalMap || next.Loaded || next.Completed() {
		t.Fatalf("closing scene must first revisit the cached final map: next=%+v err=%v", next, err)
	}
	next.Loaded = true
	next.TryComplete()
	if !next.Completed() || next.CompletionNeedsBossCheck() || next.CompletionTarget() != 0 {
		t.Fatalf("source scene close must settle without a non-existent boss: next=%+v", next)
	}
	bad := r
	bad.Record[6]++
	if _, err := s.MoveScene(c, bad); err == nil {
		t.Fatal("accepted a terminal transition absent from the source cinematic")
	}
	delete(s.Dead, 0x1010)
	if _, err := s.MoveScene(c, r); err == nil {
		t.Fatal("accepted terminal transition before the objective room was cleared")
	}
	s.Dead[0x1010] = true
	s.Maze.Quest++
	if _, err := s.MoveScene(c, r); err == nil {
		t.Fatal("accepted the closing scene for another quest maze")
	}
}

func TestTerminalSceneUsesCatalogInsteadOfQuestIDs(t *testing.T) {
	const finalMap uint32 = 9002
	const objectiveMap uint32 = 9001
	c := catalog.DungeonCatalog{
		Maps: map[uint32]catalog.ScriptRecord{finalMap: {SHA256: "another-map"}},
		TerminalScenes: []catalog.DungeonTerminalScene{{
			Source: "source", Dungeon: 800, Maze: 2, Quest: 777,
			Position: [2]byte{4, 3}, ObjectiveMap: objectiveMap, FinalMap: finalMap,
			XMin: 420, XMax: 422, YMin: 315, YMax: 316,
			DungeonSHA256: "another-dungeon", MapSHA256: "another-map",
		}},
	}
	c.Source.Checksum = "source"
	s := &Session{
		Definition: catalog.DungeonDefinition{ID: 800, Script: catalog.ScriptRecord{SHA256: "another-dungeon"}},
		Maze: catalog.DungeonMaze{Index: 2, Quest: 777, Layers: []catalog.DungeonLayer{{
			Position: [2]byte{4, 3}, Maps: []uint32{finalMap},
		}}},
		Room: catalog.DungeonRoom{X: 4, Y: 3, Map: finalMap}, Loaded: true,
		Monsters: []protocol.DungeonMonster{{Entity: 22, Team: 100}},
		Visited: map[uint32][]protocol.DungeonMonster{
			objectiveMap: {{Entity: 21, Team: 100}},
			finalMap:     {{Entity: 22, Team: 100}},
		},
		Dead: map[uint16]bool{21: true},
	}
	r := protocol.DungeonRoomTransition{Dungeon: 800, Position: [2]byte{4, 3}, LayerChange: true,
		Record: [18]byte{0, 0, 0, 0, 4, 5, 0xa5, 0x01, 0x3b, 0x01}}
	next, err := s.MoveScene(c, r)
	if err != nil {
		t.Fatal(err)
	}
	next.Loaded = true
	next.TryComplete()
	if !next.Completed() || next.CompletionNeedsBossCheck() {
		t.Fatal("catalog-backed terminal scene did not finish without a boss")
	}
	c.Source.Checksum = "other-source"
	if _, err := s.MoveScene(c, r); err == nil {
		t.Fatal("terminal scene from a different PVF source was accepted")
	}
}
