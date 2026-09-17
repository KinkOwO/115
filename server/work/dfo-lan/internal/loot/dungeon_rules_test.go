package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"reflect"
	"testing"
)

func TestDungeonSourceGoldExclusion(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.odyssey-merged-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	a := []Award{{0, 100}, {6003, 1}}
	got := filterDungeonAwards(c.Dungeons[100004934], a)
	if !reflect.DeepEqual(got, []Award{{6003, 1}}) {
		t.Fatal("Odyssey source gold exclusion ignored", got)
	}
	if !reflect.DeepEqual(filterDungeonAwards(c.Dungeons[3], a), a) {
		t.Fatal("ordinary drop changed")
	}
	if len(a) != 2 || a[0].Template != 0 {
		t.Fatal("input mutated")
	}
	wrongType := catalog.DungeonDefinition{Script: catalog.ScriptRecord{Cells: []pvf.Token{{Type: 6, Text: "[exclude gold drop]"}}}}
	if !reflect.DeepEqual(filterDungeonAwards(wrongType, a), a) {
		t.Fatal("untyped tag accepted")
	}
}

func TestUnownedAndFriendlyDeathsHaveNoDropRoll(t *testing.T) {
	for _, friendly := range []bool{false, true} {
		d := &dungeon.Session{RunID: "run", Loaded: true, Dead: map[uint16]bool{1: true}, Unowned: map[uint16]bool{1: !friendly}, Monsters: []protocol.DungeonMonster{{Entity: 1, NonCombat: friendly}}}
		s := NewSession(catalog.LootCatalog{}, Tables{}, Rules{}, nil, "run", 1, 1, 1)
		rows, e := s.Death(d, 1)
		if e != nil || len(rows) != 0 || len(s.Objects) != 0 || len(s.seeds) != 0 {
			t.Fatal("noncombat/unowned death rolled", e)
		}
	}
}
