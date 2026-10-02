package loot

import (
	"reflect"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

func TestOrdinaryRateTableOnlyReplacesItsDeclaredClasses(t *testing.T) {
	c := catalog.LootCatalog{Items: map[uint32]catalog.LootItem{100: {Kind: "stackable"}}}
	pool := []inventory.EquipmentDrop{{ID: 200, Rarity: 0}, {ID: 201, Rarity: 1}, {ID: 202, Rarity: 2}, {ID: 203, Rarity: 3}, {ID: 206, Rarity: 6}}
	base := []Award{{0, 42}, {100, 1}, {200, 1}, {201, 1}, {202, 1}, {203, 1}, {206, 1}}
	entries := []catalog.DungeonDropRateEntry{{Grade: "unique"}, {Grade: "legendary"}}
	keep := base[:5]
	for _, from := range [][]Award{nil, {{303, 1}}} {
		got := replaceOrdinaryRateAwards(base, from, entries, c, pool)
		want := append(append([]Award(nil), keep...), from...)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unique/legendary table erased other classes or retained owned misses: %v want %v", got, want)
		}
	}
	entries = []catalog.DungeonDropRateEntry{{Grade: "stackable"}, {Grade: "rare"}}
	got := replaceOrdinaryRateAwards(base, nil, entries, c, pool)
	want := []Award{{0, 42}, {200, 1}, {201, 1}, {203, 1}, {206, 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stackable/rare table failed to own only its classes: %v", got)
	}
}

func TestOrdinaryUniqueMapMissDoesNotEraseStackableDeath(t *testing.T) {
	c := catalog.LootCatalog{ClearReward: &catalog.ClearRewardTable{}, Items: map[uint32]catalog.LootItem{100: {ID: 100, Kind: "stackable", Grade: 15, Weight: 1}}, DungeonDropInfo: map[uint32][]catalog.DungeonDropRateEntry{11: {{Grade: "unique", DropGroup: 11005, RateList: map[string][]uint32{"normal": {0}}}, {Grade: "legendary", DropGroup: 11006, RateList: map[string][]uint32{"normal": {0}}}}}}
	tables := Tables{Probability: []float64{1, 200, 0, 10000, 0, 0, 0}, Gold: []float64{15, 100, 0}, Grade: []float64{15, 7, 3}, Rank: make([]float64, 20), Rarity: make([]float64, 36), Difficulty: make([]float64, 25)}
	for _, values := range [][]float64{tables.Rank, tables.Difficulty} {
		for i := range values {
			values[i] = 1
		}
	}
	for i := range tables.Rarity {
		tables.Rarity[i] = 1000000
	}
	rules := Rules{Denominator: 10000, DifficultyBonus: []float64{1, 1, 1, 1, 1}, SupportedKinds: []string{"stackable"}}
	run := &dungeon.Session{RunID: "live-zero-map", Loaded: true, NextEntity: 10, Room: catalog.DungeonRoom{Map: 58605}, Definition: catalog.DungeonDefinition{ID: 11}, Monsters: []protocol.DungeonMonster{{Entity: 1, Level: 15}}, Dead: map[uint16]bool{1: true}}
	s := NewSession(c, tables, rules, nil, run.RunID, 1, 1, 3)
	s.seeds[58605] = 42
	rows, err := s.Death(run, 1)
	if err != nil || len(rows) != 1 || len(s.Objects) != 1 {
		t.Fatalf("unrelated map rarity removed generic stackable: rows=%v err=%v", rows, err)
	}
	for _, drop := range s.Objects {
		if drop.Award != (Award{100, 1}) {
			t.Fatalf("wrong stackable award: %+v", drop)
		}
	}
	replayed, err := s.Death(run, 1)
	if err != nil || !reflect.DeepEqual(rows, replayed) || len(s.Objects) != 1 {
		t.Fatalf("death retry duplicated or lost scoped loot: %v %v", replayed, err)
	}
}
