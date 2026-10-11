package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"testing"
)

func asrahanSpawnFixture() (*catalog.AsrahanRaidRules, *Session) {
	r := &catalog.AsrahanRaidRules{Symbols: map[string]uint32{}, MonsterSource: &catalog.AsrahanMonsterSource{Path: "fixture", SHA256: "fixture", Monsters: map[string]catalog.AsrahanMonster{"boss": {Kind: "boss", Template: 41, X: 793, Y: 337}}}}
	fields := make([]pvf.Token, 17)
	for i := range fields {
		fields[i] = pvf.Token{Type: 0, Value: -1}
	}
	for i, slot := range []int{0, 2, 3, 4, 13} {
		name := string(rune('A' + i))
		r.Symbols[name] = uint32(i + 1)
		fields[slot] = pvf.Token{Type: 3, Text: name}
	}
	fields[7] = pvf.Token{Type: 0, Value: 41}
	r.Groups = []catalog.AsrahanSymbolGroup{{Header: []pvf.Token{{Type: 0, Value: 1}, {Type: 0, Value: 101}, {Type: 0, Value: 1}}, Fields: fields}}
	// Deliberately distinct from the monster COS kind, like native Terminus.
	r.Normal = catalog.AsrahanRaidPhase{Dungeons: map[uint32]catalog.AsrahanDungeon{101: {Kind: "another", InitialState: "open"}}}
	r.Hard = r.Normal
	s := &Session{RunID: "owned", Loaded: true, Definition: catalog.DungeonDefinition{ID: 101, BasisLevel: 110}, Room: catalog.DungeonRoom{X: 1, Y: 2, Map: 99}, Maze: catalog.DungeonMaze{Boss: [2]byte{1, 2}}, NextEntity: 4096, Dead: map[uint16]bool{}, completed: true}
	return r, s
}

func TestAsrahanLoadedSourceBossRegistersCombatAndClearOwnership(t *testing.T) {
	r, s := asrahanSpawnFixture()
	row, added, err := s.AddAsrahanBoss(r, false, 1)
	if err != nil || !added || row.Template != 41 || row.X != 793 || row.Y != 337 || s.Completed() || len(s.Monsters) != 1 {
		t.Fatal("source boss registration failed", row, added, err)
	}
	if _, err := protocol.UnassignedMonsterAdd115([]protocol.UnassignedMonster115{row}); err != nil {
		t.Fatal(err)
	}
	if _, added, err := s.AddAsrahanBoss(r, false, 1); err != nil || added || s.NextEntity != 4097 {
		t.Fatal("loading replay spawned duplicate boss", added, err)
	}
	if _, err := s.ConfirmDeath(uint32(row.Entity), 7, 7); err != nil {
		t.Fatal(err)
	}
	if !s.Dead[row.Entity] {
		t.Fatal("real source boss missing from death roster")
	}
}

func TestAsrahanRejectedSpawnDoesNotMutateRoom(t *testing.T) {
	for _, mutate := range []func(*catalog.AsrahanRaidRules, *Session){
		func(_ *catalog.AsrahanRaidRules, s *Session) { s.Loaded = false },
		func(_ *catalog.AsrahanRaidRules, s *Session) { s.RunID = "" },
		func(_ *catalog.AsrahanRaidRules, s *Session) { s.Definition.ID = 102 },
		func(_ *catalog.AsrahanRaidRules, s *Session) { s.Room.X = 0 },
		func(_ *catalog.AsrahanRaidRules, s *Session) { s.NextEntity = 65535 },
		func(r *catalog.AsrahanRaidRules, _ *Session) { r.Groups[0].Fields[7].Value = 42 },
		func(r *catalog.AsrahanRaidRules, _ *Session) {
			m := r.MonsterSource.Monsters["boss"]
			m.Template = 0
			r.MonsterSource.Monsters["boss"] = m
		},
	} {
		r, s := asrahanSpawnFixture()
		mutate(r, s)
		entity := s.NextEntity
		if _, added, err := s.AddAsrahanBoss(r, false, 1); err == nil || added {
			t.Fatal("foreign/unloaded/placeholder source obtained spawn")
		}
		if len(s.Monsters) != 0 || s.NextEntity != entity || !s.completed {
			t.Fatal("rejected spawn changed room")
		}
	}
}

func TestAsrahanMapNativeBossDoesNotUsePlaceholderOrRespawn(t *testing.T) {
	r, s := asrahanSpawnFixture()
	m := r.MonsterSource.Monsters["boss"]
	m.Template = 0
	r.MonsterSource.Monsters["boss"] = m
	s.Monsters = []protocol.DungeonMonster{{Entity: 4096, SourceIndex: 0, Template: 41, Rank: 3, Team: 100}}
	s.NextEntity = 4097
	if _, added, err := s.AddAsrahanBoss(r, false, 1); err != nil || added || len(s.Monsters) != 1 || s.NextEntity != 4097 || !s.ArenaBoss || s.Completed() {
		t.Fatal("native map actor was duplicated or rejected", added, err)
	}
	s.Dead[4096] = true
	if _, added, err := s.AddAsrahanBoss(r, false, 1); err == nil || added || len(s.Monsters) != 1 {
		t.Fatal("cleared source actor was respawned", added, err)
	}
}

func TestAsrahanSpawnDoesNotBorrowNormalPhaseDungeon(t *testing.T) {
	r, s := asrahanSpawnFixture()
	r.Hard.Dungeons = map[uint32]catalog.AsrahanDungeon{102: {Kind: "other", InitialState: "open"}}
	if _, added, err := s.AddAsrahanBoss(r, true, 1); err == nil || added || len(s.Monsters) != 0 || s.NextEntity != 4096 || !s.completed {
		t.Fatal("hard phase borrowed normal dungeon", added, err)
	}
}
