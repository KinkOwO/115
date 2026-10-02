package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

func TestSourceDropPlanOwnershipAndRetry(t *testing.T) {
	c, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	rules, e := LoadRules("../../configs/drop.compat90.json")
	if e != nil {
		t.Fatal(e)
	}
	tables, e := Parse(c)
	if e != nil {
		t.Fatal(e)
	}
	if tables.Gold[6] != 3 || tables.Gold[7] != 34 || tables.Gold[8] != 15 || tables.Probability[2] != 986 || tables.Rank[8] != 0.2 {
		t.Fatal("current source table shape/value drift")
	}
	dc, e := catalog.LoadDungeons("../../configs/dungeons.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	run, e := dungeon.Select(dc, protocol.DungeonSelection{ID: 3, Quest: 3145, Party: 65535}, 1, map[uint16]bool{3145: true})
	if e != nil {
		t.Fatal(e)
	}
	// This test covers the generic source roll's ownership and retry contract.
	// A declared map pool can legitimately suppress its item result; that
	// boundary is exercised separately by TestOrdinaryMapMissOwnsItemResultAndDeathRetry.
	run.Definition.Script.Cells = nil
	delete(c.DungeonDropInfo, run.Definition.ID)
	s := NewSession(c, tables, rules, nil, run.RunID, 10, 20, 3)
	entity := run.Monsters[0].Entity
	if _, e = s.Death(run, entity); e == nil {
		t.Fatal("unloaded/live monster awarded")
	}
	run.Loaded = true
	run.ConfirmDeath(uint32(entity), 3, 3)
	// Choose a deterministic seed that exercises current PVF gold and a real
	// stackable item. RNG remains a private room stream in production.
	var seed uint32
	for ; seed < 100000; seed++ {
		out, err := Roll(c, tables, rules, nil, seed, run.Monsters[0].Level, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		gold, stack := false, false
		for _, a := range out.Awards {
			gold = gold || a.Template == 0
			stack = stack || a.Template != 0
		}
		if gold && stack {
			break
		}
	}
	if seed == 100000 {
		t.Fatal("source pools never selected")
	}
	s.seeds[run.Room.Map] = seed
	p, e := s.Death(run, entity)
	if e != nil || len(p) < 2 {
		t.Fatal(p, e)
	}
	beforeSeed := s.seeds[run.Room.Map]
	again, e := s.Death(run, entity)
	a, _ := json.Marshal(p)
	b, _ := json.Marshal(again)
	if e != nil || string(a) != string(b) || s.seeds[run.Room.Map] != beforeSeed {
		t.Fatal("death rerolled")
	}
	object := p[0].Object
	if _, e = s.Owned(run, 11, 20, 3, object); e == nil {
		t.Fatal("cross account pickup")
	}
	if _, e = s.Owned(run, 10, 20, 4, object); e == nil {
		t.Fatal("cross actor pickup")
	}
	otherRoom := *run
	otherRoom.Room.Map++
	if _, e = s.Owned(&otherRoom, 10, 20, 3, object); e == nil {
		t.Fatal("cross room pickup")
	}
	t.Logf("source plan seed=%d objects=%d", seed, len(p))
}
