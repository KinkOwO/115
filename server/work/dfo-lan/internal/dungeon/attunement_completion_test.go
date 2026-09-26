package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
)

// The boundary-of-attunement mode never sends CMD117, so completionTarget stays
// zero and the two generic "no BOSS_CHECK needed" paths cannot fire - both
// require !hasFightableBoss(), and this mode's source boss is a real fightable
// rank-3 actor. Without the source-boss branch the run could never complete:
// that is the 2026-09-26 "clear room, GO appears, nothing happens" report.
func TestAttunementSourceBossDeathCompletesWithoutBossCheck(t *testing.T) {
	s := attunementSession()

	// Clearing the ordinary monsters must not complete the run: the source
	// script's clear condition is [hunt boss] on the pillar, not room clear.
	s.Dead[4100] = true
	s.tryComplete()
	if s.Completed() {
		t.Fatal("ordinary monsters alone must not complete a hunt-boss run")
	}
	if s.CompletionTarget() != 0 {
		t.Fatal("completion target reported before the run completed")
	}

	// The pillar's own death completes it, with no CMD117 anywhere.
	s.Dead[4118] = true
	s.tryComplete()
	if !s.Completed() {
		t.Fatal("source boss death must complete the run without CMD117")
	}
	// NOTI115 carries this identity; the wire encoder rejects 0 and 65535.
	if got := s.CompletionTarget(); got != 4118 {
		t.Fatalf("completion target = %d, want the pillar's own entity 4118", got)
	}
}

// The branch is gated on the source boss template, so a dungeon that merely
// happens to have a rank-3 actor is untouched.
func TestAttunementCompletionRequiresTheSourceBoss(t *testing.T) {
	s := attunementSession()
	s.Definition.AttunementBoss = 0 // not a recognised source
	s.Dead[4100] = true
	s.Dead[4118] = true
	s.tryComplete()
	if s.Completed() {
		t.Fatal("a rank-3 death must not complete a run with no source boss declared")
	}
}

// A different rank-3 template in the same room must not satisfy it.
func TestAttunementCompletionIgnoresOtherRankThree(t *testing.T) {
	s := attunementSession()
	s.Dead[4100] = true
	s.Dead[4200] = true // some other rank-3 actor
	s.tryComplete()
	if s.Completed() {
		t.Fatal("only the declared source boss may complete the run")
	}
}

func attunementSession() *Session {
	return &Session{
		Definition: catalog.DungeonDefinition{ID: 100005068, AttunementBoss: 109008634},
		Loaded:     true,
		Dead:       map[uint16]bool{},
		Monsters: []protocol.DungeonMonster{
			{Entity: 4100, Template: 109018067, Rank: 0, Level: 145, Team: 100},
			{Entity: 4200, Template: 109019402, Rank: 3, Level: 145, Team: 100},
			{Entity: 4118, Template: 109008634, Rank: 3, Level: 145, Team: 100},
		},
	}
}
