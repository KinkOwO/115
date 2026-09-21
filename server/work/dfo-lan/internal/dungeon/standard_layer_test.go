package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestStandardDungeonCastellanChamberLayerTransitions(t *testing.T) {
	c, err := catalog.LoadDungeons("testdata/dungeon15_maze4.json")
	if err != nil {
		t.Fatalf("failed to load fixture: %v", err)
	}

	// Select Dungeon 15 with Quest 3185 (Maze 4: Castellan's Chamber, Quest: skycastle_18)
	accepted := map[uint16]bool{3185: true}
	s, err := Select(c, protocol.DungeonSelection{
		ID:         15,
		Difficulty: 2,
		Party:      65535,
		Quest:      3185,
	}, 19, accepted)
	if err != nil {
		t.Fatalf("failed to select dungeon 15 maze 4: %v", err)
	}

	if s.Maze.Index != 4 {
		t.Fatalf("expected maze 4, got %d", s.Maze.Index)
	}
	if len(s.Maze.Layers) != 1 || s.Maze.Layers[0].Position != [2]byte{1, 0} {
		t.Fatalf("expected 1 layer at (1, 0), got %+v", s.Maze.Layers)
	}

	// Enter boss room (1, 0), base map 100008324
	bossRoom := catalog.DungeonRoom{X: 1, Y: 0, Map: 100008324, Boss: true}
	s, err = s.enterRoom(c, bossRoom)
	if err != nil {
		t.Fatalf("failed to enter boss base room: %v", err)
	}
	s.Loaded = true

	if s.Room.Map != 100008324 {
		t.Fatalf("expected base boss room map 100008324, got %d", s.Room.Map)
	}

	// 1. First layer transition: cutscene 14646 ends and sends CMD 45 with LayerChange=true
	r := protocol.DungeonRoomTransition{
		Dungeon:     15,
		Position:    [2]byte{1, 0},
		LayerChange: true,
		Record:      [18]byte{0, 0, 0, 0, 4, 5, 0x80, 0x04, 0x2f, 0x01}, // landing coordinates (1152, 303)
	}

	next, err := s.MoveScene(c, r)
	if err != nil {
		t.Fatalf("failed first layer transition: %v", err)
	}
	if next.Room.Map != 57962 {
		t.Fatalf("expected next map 57962 (Lord of Light boss room), got %d", next.Room.Map)
	}

	// Verify Lord of Light boss is spawned in map 57962
	var lordOfLight uint16
	for _, m := range next.Monsters {
		if m.Template == 109015501 && m.Rank == 3 {
			lordOfLight = m.Entity
			break
		}
	}
	if lordOfLight == 0 {
		t.Fatalf("Lord of Light boss 109015501 missing from layer map 57962")
	}

	next.Loaded = true

	// 2. Kill boss Lord of Light
	if applied, err := next.ConfirmDeath(uint32(lordOfLight), 11, 11); err != nil || !applied {
		t.Fatalf("failed to kill Lord of Light: %v", err)
	}

	// Client sends BossCheck (CMD 117) for Lord of Light right after killing him in 57962.
	// In standard layered dungeons, BossCheck MUST be accepted to record completionTarget,
	// but dungeon completion is deferred until reaching the final map of the layer sequence.
	if err := next.BossCheck(protocol.BossCheckRequest{Actor: 11, Target: lordOfLight}, 11); err != nil {
		t.Fatalf("BossCheck on Lord of Light should be accepted, got: %v", err)
	}
	if next.Completed() {
		t.Fatalf("dungeon should not complete yet until reaching final outro layer 100008595")
	}

	// 3. Second layer transition: boss death cutscene 14896 ends and sends CMD 45 with LayerChange=true
	r2 := protocol.DungeonRoomTransition{
		Dungeon:     15,
		Position:    [2]byte{1, 0},
		LayerChange: true,
		Record:      [18]byte{0, 0, 0, 0, 4, 5, 0x66, 0x02, 0xf9, 0x00}, // landing coordinates
	}
	next2, err := next.MoveScene(c, r2)
	if err != nil {
		t.Fatalf("failed second layer transition: %v", err)
	}
	if next2.Room.Map != 100008595 {
		t.Fatalf("expected next map 100008595 (final dummy boss room), got %d", next2.Room.Map)
	}

	// Verify dummy boss 75099 is present in map 100008595
	var dummyBoss uint16
	for _, m := range next2.Monsters {
		if m.Template == 75099 && m.Rank == 3 {
			dummyBoss = m.Entity
			break
		}
	}
	if dummyBoss == 0 {
		t.Fatalf("dummy boss 75099 missing from final layer map 100008595")
	}

	next2.Loaded = true

	// 4. Dummy boss destroyed by cutscene 14897, ConfirmDeath marks it dead
	if applied, err := next2.ConfirmDeath(uint32(dummyBoss), 65535, 11); err != nil || !applied {
		t.Fatalf("failed to confirm dummy boss death: %v", err)
	}

	// Now that final map is reached and all bosses are dead, dungeon MUST be completed!
	if !next2.Completed() {
		t.Fatalf("expected dungeon to be completed after dummy boss death on final map")
	}

	// 5. Verify ClearedMaps includes map 57962 (Quest 3185 target)
	cleared := next2.ClearedMaps()
	foundQuestMap := false
	for _, id := range cleared {
		if id == 57962 {
			foundQuestMap = true
			break
		}
	}
	if !foundQuestMap {
		t.Fatalf("expected cleared maps to include 57962 for Quest 3185, got %v", cleared)
	}
}
