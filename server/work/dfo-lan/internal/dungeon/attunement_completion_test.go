package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
)

// A dungeon whose client never sends CMD117 keeps completionTarget at zero, so the
// two generic "no BOSS_CHECK needed" paths cannot fire - both require
// !hasFightableBoss(), and these runs' source boss is a real fightable rank-3
// actor. Without the source-boss branch such a run could never complete. That is
// the 2026-09-26 report for both the boundary-of-attunement pillar (100005067/68)
// and the final-attuner scales (100005014).
func TestSourceBossDeathCompletesWithoutBossCheck(t *testing.T) {
	s := attunementSession()

	// Clearing part of the room must not complete the run: the source script's
	// clear condition is [hunt boss] on its own boss, not room clear.
	s.Dead[4100] = true
	s.tryComplete()
	if s.Completed() {
		t.Fatal("ordinary monsters alone must not complete a hunt-boss run")
	}
	if s.CompletionTarget() != 0 {
		t.Fatal("completion target reported before the run completed")
	}

	// Everything but the declared boss is dead and the boss itself is not: still
	// nothing to settle, because the condition names the boss.
	s.Dead[4200] = true
	s.tryComplete()
	if s.Completed() {
		t.Fatal("a cleared room with the source boss alive must not complete")
	}

	// The declared boss's own death completes it, with no CMD117 anywhere.
	s.Dead[4118] = true
	s.tryComplete()
	if !s.Completed() {
		t.Fatal("source boss death must complete the run without CMD117")
	}
	// NOTI115 carries this identity; the wire encoder rejects 0 and 65535.
	if got := s.CompletionTarget(); got != 4118 {
		t.Fatalf("completion target = %d, want the source boss's own entity 4118", got)
	}
}

// The branch is gated on the source boss template, so a dungeon that merely
// happens to have a rank-3 actor is untouched.
func TestSourceBossCompletionRequiresTheSourceBoss(t *testing.T) {
	s := attunementSession()
	s.Definition.SourceBoss = 0 // not a recognised source
	s.Dead[4100] = true
	s.Dead[4200] = true
	s.Dead[4118] = true
	s.tryComplete()
	if s.Completed() {
		t.Fatal("a rank-3 death must not complete a run with no source boss declared")
	}
}

// A different rank-3 template in the same room must not satisfy it.
func TestSourceBossCompletionIgnoresOtherRankThree(t *testing.T) {
	s := attunementSession()
	s.Dead[4100] = true
	s.Dead[4200] = true // some other rank-3 actor
	s.tryComplete()
	if s.Completed() {
		t.Fatal("only the declared source boss may complete the run")
	}
}

// A run outside the script's own boss room must not settle, however dead the room
// is: the branch is a boss-room fallback, not a global "anything died" rule.
func TestSourceBossCompletionRequiresTheSourceBossRoom(t *testing.T) {
	s := attunementSession()
	s.Room = catalog.DungeonRoom{X: 0, Y: 0, Map: 100001016}
	s.Dead[4100] = true
	s.Dead[4200] = true
	s.Dead[4118] = true
	s.tryComplete()
	if s.Completed() {
		t.Fatal("a room that is not the source boss room must not settle")
	}
}

// attunementSession stands in the source boss room of a hunt-boss dungeon: the
// room whose coordinates the maze declares as its [boss map], with a boss-flagged
// room entry behind it.
func attunementSession() *Session {
	const mapID = uint32(100001016)
	return &Session{
		Definition: catalog.DungeonDefinition{ID: 100005068, SourceBoss: 109008634},
		Loaded:     true,
		Dead:       map[uint16]bool{},
		Room:       catalog.DungeonRoom{X: 0, Y: 2, Map: mapID, Boss: true},
		Maze: catalog.DungeonMaze{
			Index: 0,
			Boss:  [2]byte{0, 2},
			Rooms: []catalog.DungeonRoom{{X: 0, Y: 2, Map: mapID, Boss: true}},
		},
		Monsters: []protocol.DungeonMonster{
			{Entity: 4100, Template: 109018067, Rank: 0, Level: 145, Team: 100},
			{Entity: 4200, Template: 109019402, Rank: 3, Level: 145, Team: 100},
			{Entity: 4118, Template: 109008634, Rank: 3, Level: 145, Team: 100},
		},
	}
}
