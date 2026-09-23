package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

// A layered story sequence reaches its end map without ever producing a
// BOSS_CHECK: the client raises command 117 only for a real boss, and a
// [displayhuntdummy] actor is spawned with NonCombat set. Completion therefore
// has to close on the condition the layer does have - the sequence reached its
// final map and every killable enemy is dead - or the run is stranded with a
// dark clear button.
func TestStoryLayerCompletesWithoutBossCheck(t *testing.T) {
	s := storyLayerSession(t, true)
	// The dummy boss carries NonCombat, so the client never reports its rank-3
	// identity and completionTarget must stay zero for this whole test.
	if s.hasKillableBoss() {
		t.Fatal("display dummy counted as a killable boss")
	}
	if !s.atLayerFinalMap() {
		t.Fatal("final layer map not recognised")
	}
	// Clearing the room is what completes it; no BossCheck is involved.
	killKillable(s)
	if !s.roomSettled() {
		t.Fatal("room not settled after every killable enemy died")
	}
	s.TryComplete()
	if !s.Completed() {
		t.Fatal("story layer did not complete on room clear")
	}
	// No BOSS_CHECK happened, so no requested identity exists. The reported
	// target is the display dummy standing in for the boss: the wire payload
	// needs a legal identity or the clear-enable batch is dropped.
	if s.completionTarget != 0 {
		t.Fatalf("story layer fabricated a requested target: %d", s.completionTarget)
	}
	if target := s.CompletionTarget(); target == 0 || target == 65535 {
		t.Fatalf("completion target unusable for the wire payload: %d", target)
	}
}

// The guard that keeps the fallback honest: a room with a real killable boss
// must still wait for its BOSS_CHECK. Without this the fallback would complete
// every ordinary boss room the moment its small monsters died.
func TestStoryLayerFallbackRefusesRealBossRoom(t *testing.T) {
	s := storyLayerSession(t, false)
	if !s.hasKillableBoss() {
		t.Fatal("real boss not recognised as killable")
	}
	killKillable(s)
	s.TryComplete()
	if s.Completed() {
		t.Fatal("real boss room completed without its boss check")
	}
}

// An ordinary room reached mid-route is not a layer entry at all. It must never
// complete on room clear, or the first small room would end the whole dungeon.
func TestOrdinaryRoomNeverCompletesWithoutBossCheck(t *testing.T) {
	s := storyLayerSession(t, true)
	// Move the session off the layered sequence entirely.
	s.Maze.Layers = nil
	killKillable(s)
	if s.atLayerFinalMap() {
		t.Fatal("room outside every layer reported a final map")
	}
	s.TryComplete()
	if s.Completed() {
		t.Fatal("ordinary room completed without a boss check")
	}
}

// A layered sequence that has not reached its last map must not complete even
// when its current map is empty: the client still has scenes to play.
func TestStoryLayerMidMapDoesNotComplete(t *testing.T) {
	s := storyLayerSession(t, true)
	s.Room.Map = 100008695 // first map of the sequence, not the last
	killKillable(s)
	s.TryComplete()
	if s.Completed() {
		t.Fatal("sequence completed before its final map")
	}
}

// A live killable enemy blocks completion even on the final map.
func TestStoryLayerWaitsForLiveEnemy(t *testing.T) {
	s := storyLayerSession(t, true)
	// Kill everything the client would fight, then add one more live enemy.
	killKillable(s)
	s.Monsters = append(s.Monsters, protocol.DungeonMonster{
		Entity: 0x2000, Rank: 0, Team: 100, NonCombat: false,
	})
	s.TryComplete()
	if s.Completed() {
		t.Fatal("completed with a live enemy in the room")
	}
}

// A room whose actors cannot be killed still has to clear, or the client never
// gets the door that leads to the next map of the sequence.
func TestStoryLayerRoomClearedOpensForUnkillableActors(t *testing.T) {
	s := storyLayerSession(t, true)
	s.Loaded = true
	if !s.RoomCleared() {
		t.Fatal("story layer room refused to clear")
	}
}

// The door stays shut while a killable enemy lives, even on a story layer.
func TestRoomClearedBlocksLiveEnemy(t *testing.T) {
	s := storyLayerSession(t, true)
	s.Loaded = true
	s.Monsters = append(s.Monsters, protocol.DungeonMonster{
		Entity: 0x2000, Rank: 0, Team: 100, NonCombat: false,
	})
	if s.RoomCleared() {
		t.Fatal("room cleared with a live killable enemy")
	}
}

// The room-clear fallback must not hijack dungeon 26 maze 3's terminal layer.
// That layer is entered with a live combat target and is only finished by the
// closing [CHANGE MAP] cinematic returning to the cached map, so a clear of the
// map itself is not the end of the run. Without this exception the fallback
// would complete the run the instant the target dies, before the scene plays.
// The full live-verified sequence is pinned separately by
// TestLotusClosingCinematicReusesFinalLayer.
func TestStoryLayerFallbackSkipsLotusTerminalLayer(t *testing.T) {
	s := &Session{
		Definition: catalog.DungeonDefinition{ID: 26},
		Maze: catalog.DungeonMaze{Index: 3, Layers: []catalog.DungeonLayer{
			{Position: [2]byte{3, 0}, Maps: []uint32{100008786, 100008697}},
		}},
		Room:   catalog.DungeonRoom{X: 3, Y: 0, Map: 100008697, Boss: true},
		Loaded: true,
		Monsters: []protocol.DungeonMonster{
			{Entity: 0x102d, Template: 70160, Team: 100},
			{Entity: 0x1030, Template: 75099, Rank: 3, Team: 100, NonCombat: true},
		},
		Dead:    map[uint16]bool{},
		Visited: map[uint32][]protocol.DungeonMonster{},
	}
	// Preconditions: the fallback would fire here if the map were not exempt.
	if s.hasKillableBoss() || !s.atLayerFinalMap() {
		t.Fatal("precondition: the fallback would not have fired anyway")
	}
	s.Dead[0x102d] = true
	if !s.roomSettled() {
		t.Fatal("room not settled")
	}
	s.TryComplete()
	if s.Completed() {
		t.Fatal("terminal layer completed on clear, before the closing scene")
	}
	// Only the validated closing cinematic may finish it.
	s.lotusClosingReached = true
	s.TryComplete()
	if !s.Completed() || s.CompletionTarget() != 0x1030 {
		t.Fatalf("closing cinematic did not finish the run: completed=%v target=%x", s.Completed(), s.CompletionTarget())
	}
}

// storyLayerSession builds the shape of the Castellan Chamber story layer
// (dungeon 15 / maze 6 / final map 100008683): a four-map layered sequence
// whose final map holds a [displayhuntdummy] boss plus cinematic actors, none
// of which the client ever enters combat with.
//
// withDummy selects a display dummy (story layer) or a real killable boss
// (ordinary boss room) so both sides of the guard can be exercised.
func storyLayerSession(t *testing.T, withDummy bool) *Session {
	t.Helper()
	boss := protocol.DungeonMonster{Entity: 0x1013, Rank: 3, Team: 100, NonCombat: withDummy}
	return &Session{
		Loaded: true,
		Definition: catalog.DungeonDefinition{
			ID:           15,
			MinimumLevel: 18,
			BasisLevel:   18,
		},
		Maze: catalog.DungeonMaze{
			Index: 6,
			Boss:  [2]byte{0, 0},
			Rooms: []catalog.DungeonRoom{
				{X: 0, Y: 1, Map: 57980},
				{X: 0, Y: 0, Map: 57981, Boss: true},
			},
			Layers: []catalog.DungeonLayer{
				{Position: [2]byte{0, 0}, Maps: []uint32{100008695, 100008694, 100008684, 100008683}},
			},
		},
		Room: catalog.DungeonRoom{X: 0, Y: 0, Map: 100008683, Boss: true},
		Monsters: []protocol.DungeonMonster{
			// Cinematic actors: spawned with team 0, so the parser marks them
			// NonCombat and the client never reports their death.
			{Entity: 0x1010, Rank: 0, Team: 0, NonCombat: true},
			{Entity: 0x1011, Rank: 0, Team: 0, NonCombat: true},
			{Entity: 0x1012, Rank: 0, Team: 0, NonCombat: true},
			boss,
			{Entity: 0x1015, Rank: 0, Team: 0, NonCombat: true},
			{Entity: 0x1016, Rank: 0, Team: 0, NonCombat: true},
		},
		Dead:    map[uint16]bool{},
		Visited: map[uint32][]protocol.DungeonMonster{},
	}
}

// killKillable reports every enemy the client would actually fight, leaving the
// cinematic actors alive exactly as the live client does.
func killKillable(s *Session) {
	for _, m := range s.Monsters {
		if !m.NonCombat {
			s.Dead[m.Entity] = true
		}
	}
}
