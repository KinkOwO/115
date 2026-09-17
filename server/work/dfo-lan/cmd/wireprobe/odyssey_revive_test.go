package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"encoding/binary"
	"testing"
)

func TestOdysseyReviveGuards(t *testing.T) {
	role, _ := odysseyRewardFixture(t)
	w := &worldSession{role: role, activeDungeon: &dungeon.Session{RunID: "run", Loaded: true, Definition: catalog.DungeonDefinition{Odyssey: true}, Room: catalog.DungeonRoom{Map: 1}}, dungeons: &catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{1: {}}}, pilotDeath: &odysseyDeath{Run: "run", Sequence: 1, Dead: true}}
	p := make([]byte, 8)
	binary.LittleEndian.PutUint16(p, role.WireID)
	if e := w.pilotReviveAllowed(p); e != nil {
		t.Fatal(e)
	}
	for _, id := range []uint16{40, 41, 160} {
		seen := map[uint16]int{}
		for n := 0; n < 32; n++ {
			if !retainRequestBody(id, seen) {
				t.Fatal("request stopped after sample limit", id, n)
			}
		}
	}
	w.pilotDeath.Dead = false
	if e := w.pilotReviveAllowed(p); e == nil {
		t.Fatal("living revival")
	}
	w.pilotDeath.Dead = true
	w.pilotDeath.Run = "old"
	if e := w.pilotReviveAllowed(p); e == nil {
		t.Fatal("foreign run")
	}
	w.pilotDeath.Run = "run"
	p[0]++
	if e := w.pilotReviveAllowed(p); e == nil {
		t.Fatal("foreign actor")
	}
	p[0]--
	w.dungeons.Maps[1] = catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[cannot use coin map]"}}}
	if e := w.pilotReviveAllowed(p); e == nil {
		t.Fatal("source no-coin map")
	}
	w.dungeons.Maps[1] = catalog.ScriptRecord{}
	w.activeDungeon.Loaded = false
	if e := w.pilotReviveAllowed(p); e == nil {
		t.Fatal("unloaded revival")
	}
	t.Log("no-coin maps, living actor, foreign actor/run and unloaded scenes rejected; commands retained after32 requests")
}
