package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestSourceDisabledDropsNeedNoTablesAndReplayPaysNothing(t *testing.T) {
	d := &dungeon.Session{RunID: "raid", Loaded: true,
		Definition: catalog.DungeonDefinition{Script: catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[disable drop item]"}}}},
		Monsters:   []protocol.DungeonMonster{{Entity: 4096, Template: 109014482, Level: 115, Rank: 3}}, Dead: map[uint16]bool{4096: true}}
	s := NewSession(catalog.LootCatalog{}, Tables{}, Rules{}, nil, "raid", 1, 2, 2)
	for i := 0; i < 2; i++ {
		drops, err := s.Death(d, 4096)
		if err != nil || len(drops) != 0 || len(s.seeds) != 0 || len(s.Objects) != 0 {
			t.Fatal(drops, err)
		}
	}
}

func TestSceneActorDeathsDoNotUseMonsterRewardTables(t *testing.T) {
	for _, m := range []protocol.DungeonMonster{
		{Entity: 4096, Template: 55424, Rank: 5, APC: true, Level: 0, Team: 100},
		{Entity: 4096, Template: 109019135, Rank: 0, Level: 0, Team: 100},
	} {
		d := &dungeon.Session{RunID: "scene", Loaded: true, Monsters: []protocol.DungeonMonster{m}, Dead: map[uint16]bool{4096: true}}
		s := NewSession(catalog.LootCatalog{}, Tables{}, Rules{}, nil, "scene", 1, 1, 14)
		for i := 0; i < 2; i++ {
			drops, e := s.Death(d, 4096)
			if e != nil || len(drops) != 0 || len(s.seeds) != 0 || len(s.Objects) != 0 {
				t.Fatal(drops, e)
			}
		}
	}
}
