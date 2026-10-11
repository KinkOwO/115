package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"reflect"
	"testing"
)

func TestDeclaredDungeonDropsSurviveRandomExclusion(t *testing.T) {
	for _, source := range []string{"declared", "rate-table", "no-group", "zero-rate"} {
		t.Run(source, func(t *testing.T) {
			c := catalog.LootCatalog{MaximumGrade: 200, Items: map[uint32]catalog.LootItem{
				100: {ID: 100, Kind: "stackable", Grade: 10, Weight: 1},
				101: {ID: 101, Kind: "stackable", Grade: 10, Weight: 1},
			}, DropGroups: []catalog.DropGroup{{ID: 9, Explicit: []catalog.DropWeight{{Template: 101, Weight: 1}}}}}
			cells := []pvf.Token{{Type: 3, Text: "[exclude monster random drop]"}}
			if source == "declared" {
				cells = append(cells, []pvf.Token{{Type: 3, Text: "[difficulty dropitem group list]"}, {Type: 3, Text: "[group info]"}, {Type: 3, Text: "[normal group index]"}, {Value: 1}, {Value: 9}, {Type: 3, Text: "[/group info]"}, {Type: 3, Text: "[/difficulty dropitem group list]"}}...)
			}
			if source == "rate-table" || source == "zero-rate" {
				rate := uint32(groupRateBase)
				if source == "zero-rate" {
					rate = 0
				}
				c.DungeonDropInfo = map[uint32][]catalog.DungeonDropRateEntry{2: {{DropGroup: 9, DungeonType: "dgn_normal", RateList: map[string][]uint32{"normal": {rate}}}}}
			}
			tables := Tables{Probability: []float64{1, 200, 10000, 10000, 0, 0, 0}, Gold: []float64{10, 100, 0}, Grade: []float64{10, 1, 1}, Rank: make([]float64, 20), Rarity: make([]float64, 36)}
			for i := range tables.Rank {
				tables.Rank[i] = 1
			}
			for i := range tables.Rarity {
				tables.Rarity[i] = 1000000
			}
			rules := Rules{Denominator: 10000, DifficultyBonus: []float64{1}, SupportedKinds: []string{"gold", "stackable"}}
			run := &dungeon.Session{RunID: "exclusion", Loaded: true, NextEntity: 10, Difficulty: 1, Room: catalog.DungeonRoom{Map: 1}, Definition: catalog.DungeonDefinition{ID: 2, Script: catalog.ScriptRecord{Cells: cells}}, Monsters: []protocol.DungeonMonster{{Entity: 1, Template: 9, Level: 10}}, Dead: map[uint16]bool{1: true}}
			s := NewSession(c, tables, rules, nil, run.RunID, 1, 1, 3)
			s.seeds[1] = 42
			rows, err := s.Death(run, 1)
			if err != nil {
				t.Fatal(err)
			}
			gold, items := 0, 0
			for _, drop := range s.Objects {
				if drop.Award.Template == 0 {
					gold++
				} else {
					items++
					if drop.Award.Template != 101 {
						t.Fatal("generic random item bypassed exclusion", drop)
					}
				}
			}
			want := 1
			if source == "no-group" || source == "zero-rate" {
				want = 0
			}
			if gold != 1 || items != want {
				t.Fatalf("gold=%d items=%d want 1/%d", gold, items, want)
			}
			seed := s.seeds[1]
			replay, err := s.Death(run, 1)
			if err != nil || !reflect.DeepEqual(rows, replay) || s.seeds[1] != seed {
				t.Fatal("death replay rerolled declared reward", err)
			}
		})
	}
}
