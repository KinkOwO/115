package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"reflect"
	"testing"
)

func hellWaveFixture() (catalog.DungeonCatalog, *Session) {
	m := catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[special passive object]"}, {Type: 0, Value: 30528}, {Type: 0, Value: 681}, {Type: 0, Value: 303}, {Type: 0}, {Type: 0}, {Type: 6, Text: "[hellparty]"}, {Type: 0, Value: 125}, {Type: 0, Value: 25}, {Type: 0, Value: 1}, {Type: 0, Value: 44}, {Type: 0, Value: 20}, {Type: 0, Value: 2}, {Type: 6, Text: "[/hellparty]"}}}
	mob := catalog.HellPartyActor{Template: 1050}
	apc := catalog.HellPartyActor{Template: 10627, EntityType: 1}
	r := &catalog.HellPartyRules{Difficulties: []catalog.HellPartyDifficulty{{Key: "A", Columns: [5]uint32{8, 5, 100, 100, 0}}, {Key: "B", Columns: [5]uint32{6, 4}}}, Groups: map[uint16]catalog.HellPartyGroup{125: {ID: 125, Difficulty: "A", Actors: []catalog.HellPartyActor{mob}}, 44: {ID: 44, Difficulty: "A", Actors: []catalog.HellPartyActor{apc, apc}}}, Actors: map[catalog.HellPartyActor]catalog.HellPartyActorSource{mob: {HellMonster: true}, apc: {Level: 65}}}
	c := catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{7: m}, HellRules: r}
	s := &Session{Loaded: true, NextEntity: 4100, Room: catalog.DungeonRoom{Map: 6}, Definition: catalog.DungeonDefinition{BasisLevel: 62, HellParty: &catalog.DungeonHellParty{SealMap: 7}}, Dead: map[uint16]bool{}, Visited: map[uint32][]protocol.DungeonMonster{6: nil}}
	return c, s
}

func TestHellWaveOwnershipRevisitAndLastGroupDeath(t *testing.T) {
	c, s := hellWaveFixture()
	run, err := newHellPartyRun(c, s, func(n uint64) (uint64, error) { return n - 1, nil })
	if err != nil {
		t.Fatal(err)
	}
	s.HellParty = run
	if len(run.Rows) != 3 || run.Mode != 1 || s.NextEntity != 4103 || run.Rows[0].Level != 62 || run.Rows[1].Level != 65 || run.Rows[1].SourceIndex != 10000 || !run.Rows[1].APC {
		t.Fatalf("invalid source identities: %+v", run)
	}
	if _, err = s.ConfirmDeath(4100, 3, 3); err == nil {
		t.Fatal("accepted reserved Hell identity in another room")
	}
	next, err := s.enterRoom(c, catalog.DungeonRoom{Map: 7})
	if err != nil {
		t.Fatal(err)
	}
	next.Loaded = true
	if next.NextEntity != 4103 || len(next.Monsters) != 3 {
		t.Fatal("rerolled reserved roster")
	}
	if _, err = next.ConfirmDeath(4100, 4, 3); err == nil {
		t.Fatal("foreign killer accepted")
	}
	for _, id := range []uint32{4100, 4101, 4102} {
		if fresh, err := next.ConfirmDeath(id, 3, 3); err != nil || !fresh {
			t.Fatal(err)
		}
		actor, ok := next.HellPartyReward(uint16(id))
		if !ok || (id == 4102 && actor.RewardRolls != 8) || (id != 4102 && actor.RewardRolls != 0) {
			t.Fatalf("reward at wrong death: %d %+v", id, actor)
		}
	}
	if actor, _ := next.HellPartyReward(4101); actor.RewardRolls != 0 {
		t.Fatal("earlier death became a second reward trigger")
	}
	if !next.HellPartyCleared() {
		t.Fatal("selected waves did not clear")
	}
	revisit, err := next.enterRoom(c, catalog.DungeonRoom{Map: 7})
	if err != nil || !reflect.DeepEqual(revisit.Monsters, next.Monsters) || revisit.NextEntity != next.NextEntity {
		t.Fatalf("revisit changed entity roster: %v", err)
	}
	if fresh, err := next.ConfirmDeath(4102, 3, 3); err != nil || fresh {
		t.Fatal("replayed death counted")
	}
}

func TestHellSourceGapsDoNotCoerceModernActors(t *testing.T) {
	c, s := hellWaveFixture()
	g := c.HellRules.Groups[44]
	g.Actors[0].EntityType = 3
	c.HellRules.Groups[44] = g
	if _, err := newHellPartyRun(c, s, func(uint64) (uint64, error) { return 0, nil }); err == nil {
		t.Fatal("unknown type became a monster")
	}
	if s.NextEntity != 4100 {
		t.Fatal("failed plan consumed identities")
	}
}

func TestHellSparseOrdersAndModeBRemainSourceDriven(t *testing.T) {
	c, s := hellWaveFixture()
	m := c.Maps[7]
	m.Cells[9].Value = 3
	m.Cells[12].Value = 5
	c.Maps[7] = m
	for id, g := range c.HellRules.Groups {
		g.Difficulty = "B"
		c.HellRules.Groups[id] = g
	}
	c.HellRules.Difficulties[0].Columns[3] = 0
	c.HellRules.Difficulties[1].Columns[3] = 100
	run, err := newHellPartyRun(c, s, func(uint64) (uint64, error) { return 0, nil })
	if err != nil || run.Mode != 2 || run.Key != "B" || run.Rows[0].SpawnOrder != 3 || run.Rows[1].SpawnOrder != 5 || run.Actors[4102].RewardRolls != 6 {
		t.Fatalf("sparse source orders/B rules changed: %+v %v", run, err)
	}
}
