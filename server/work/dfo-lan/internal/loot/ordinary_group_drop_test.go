package loot

import (
	"reflect"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
)

func TestOrdinaryDeclaredGroupsAdvanceSharedSeed(t *testing.T) {
	c := catalog.LootCatalog{DropGroups: []catalog.DropGroup{{ID: 1, Explicit: []catalog.DropWeight{{Template: 100, Weight: 1}, {Template: 101, Weight: 1}}}}}
	const seed = 42
	out, _, err := rollDeclaredGroups(c, []uint32{1, 1}, 2, seed, true)
	if err != nil {
		t.Fatal(err)
	}
	rng := RNG{seed}
	want := []Award{{100 + rng.Next(2), 1}, {100 + rng.Next(2), 1}}
	if !reflect.DeepEqual(out.Awards, want) || out.NextSeed != rng.Seed {
		t.Fatalf("got %+v; want %v seed %d", out, want, rng.Seed)
	}
	legacy, _, err := RollDeclaredGroups(c, []uint32{1, 1}, 2, seed)
	if err != nil || legacy.NextSeed != seed || !reflect.DeepEqual(legacy.Awards, []Award{{101, 1}, {101, 1}}) {
		t.Fatalf("existing Abyss sequence changed: %+v %v", legacy, err)
	}
}

func TestOrdinaryDeclaredPoolReceivesUnresolvedGenericBudget(t *testing.T) {
	c := catalog.LootCatalog{ClearReward: &catalog.ClearRewardTable{}, Items: map[uint32]catalog.LootItem{100: {ID: 100, Kind: "stackable", Grade: 1, Weight: 1}}, DropGroups: []catalog.DropGroup{{ID: 9, Explicit: []catalog.DropWeight{{Template: 100, Weight: 1}}}}}
	tables := Tables{Probability: []float64{1, 200, 0, 0, 10000, 0, 0}, Gold: []float64{10, 100, 0}, Grade: []float64{10, 0, 1}, Rank: make([]float64, 20), Rarity: make([]float64, 36), Difficulty: make([]float64, 25)}
	for i := range tables.Rank {
		tables.Rank[i] = 1
	}
	for i := range tables.Rarity {
		tables.Rarity[i] = 1000000
	}
	for i := range tables.Difficulty {
		tables.Difficulty[i] = 1
	}
	rules := Rules{Denominator: 10000, DifficultyBonus: []float64{1, 1, 1, 1, 1}, SupportedKinds: []string{"equipment"}}
	run := &dungeon.Session{RunID: "budget", Loaded: true, Difficulty: 1, NextEntity: 10, Room: catalog.DungeonRoom{Map: 1}, Definition: catalog.DungeonDefinition{ID: 2, Script: catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[difficulty dropitem group list]"}, {Type: 3, Text: "[group info]"}, {Type: 3, Text: "[normal group index]"}, {Type: 0, Value: 1}, {Type: 0, Value: 9}, {Type: 3, Text: "[/group info]"}, {Type: 3, Text: "[/difficulty dropitem group list]"}}}}, Monsters: []protocol.DungeonMonster{{Entity: 1, Level: 10}}, Dead: map[uint16]bool{1: true}}
	s := NewSession(c, tables, rules, nil, run.RunID, 1, 1, 3)
	s.seeds[1] = 42
	rows, err := s.Death(run, 1)
	if err != nil || len(rows) != 1 {
		t.Fatalf("declared map budget was lost with empty generic gear: rows=%d err=%v", len(rows), err)
	}
	for _, drop := range s.Objects {
		if drop.Award != (Award{100, 1}) {
			t.Fatalf("wrong declared map award: %+v", drop)
		}
	}
}

func TestOrdinaryMapMissOwnsItemResultAndDeathRetry(t *testing.T) {
	for _, mode := range []string{"zero rate", "missing group", "empty group", "rate miss", "Abyss", "Attunement", "Odyssey", "Hell Party"} {
		t.Run(mode, func(t *testing.T) {
			c := catalog.LootCatalog{MaximumGrade: 200, Items: map[uint32]catalog.LootItem{100: {ID: 100, Kind: "stackable", Grade: 10, Weight: 1}},
				DropGroups:      []catalog.DropGroup{{ID: 1}},
				DungeonDropInfo: map[uint32][]catalog.DungeonDropRateEntry{2: {{DropGroup: 1, DungeonType: "dgn_normal", RateList: map[string][]uint32{"normal": {0}}}}}}
			rate := uint32(0)
			if mode == "missing group" || mode == "empty group" {
				rate = groupRateBase
			} else if mode == "rate miss" {
				rate = 1
			}
			c.DungeonDropInfo[2][0].RateList["normal"][0] = rate
			if mode == "missing group" {
				c.DropGroups = nil
			}
			if mode == "Abyss" {
				c.DungeonDropInfo[2][0].DungeonType = "dgn_hell"
			}
			tables := Tables{Probability: []float64{1, 200, 10000, 10000, 0, 0, 0}, Gold: []float64{10, 100, 0}, Grade: []float64{10, 1, 1}, Rank: make([]float64, 20), Rarity: make([]float64, 36)}
			for i := range tables.Rank {
				tables.Rank[i] = 1
			}
			for i := range tables.Rarity {
				tables.Rarity[i] = 1000000
			}
			rules := Rules{Denominator: 10000, DifficultyBonus: []float64{1}, SupportedKinds: []string{"gold", "stackable"}}
			run := &dungeon.Session{RunID: "ordinary", Loaded: true, NextEntity: 10, Difficulty: 1, Definition: catalog.DungeonDefinition{ID: 2},
				Monsters: []protocol.DungeonMonster{{Entity: 1, Template: 9, Level: 10}}, Dead: map[uint16]bool{1: true}}
			run.Room.Map = 1
			if mode == "Odyssey" {
				run.Definition.Odyssey = true
			}
			if mode == "Hell Party" {
				run.HellPosition = &[2]byte{1, 1}
			}
			s := NewSession(c, tables, rules, nil, run.RunID, 1, 1, 3)
			s.seeds[run.Room.Map] = 42
			if mode == "Attunement" {
				s.Attunement = &AttunementRewards{Tables: []attunementDungeon{{Dungeon: 2}}}
			}
			out, err := s.Death(run, 1)
			if err != nil {
				t.Fatal(err)
			}
			wantItems := 0
			if mode == "Abyss" || mode == "Attunement" || mode == "Odyssey" || mode == "Hell Party" {
				wantItems = 1
			}
			gold, items := 0, 0
			for _, drop := range s.Objects {
				if drop.Award.Template == 0 {
					gold++
				} else {
					items++
				}
			}
			if gold != 1 || items != wantItems {
				t.Fatalf("got gold %d items %d, want 1/%d", gold, items, wantItems)
			}
			seedAfter := s.seeds[run.Room.Map]
			replay, err := s.Death(run, 1)
			if err != nil || !reflect.DeepEqual(out, replay) || seedAfter != s.seeds[run.Room.Map] {
				t.Fatalf("death rerolled: %v", err)
			}
		})
	}
}
