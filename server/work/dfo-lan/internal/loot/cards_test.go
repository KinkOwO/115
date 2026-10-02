package loot

import (
	"encoding/json"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

func TestCardGold(t *testing.T) {
	tables := Tables{
		Gold: []float64{18, 250, 20},
	}
	rules := CardRules{
		GoldNumerator:   175,
		GoldDenominator: 1000,
		Difficulty:      []float64{1.0, 1.2, 1.5},
	}
	gold, err := CardGold(tables, rules, 12345, 18, 0)
	if err != nil {
		t.Fatalf("CardGold failed: %v", err)
	}
	if gold == 0 {
		t.Fatalf("expected non-zero gold, got 0")
	}
}

func TestOrdinaryZeroDifficultyDoesNotBlockCardPlan(t *testing.T) {
	s := Service{
		Catalog: catalog.LootCatalog{ClearReward: &catalog.ClearRewardTable{
			Profiles:            []catalog.ClearRewardProfile{{Name: "default", Rows: []catalog.ClearRewardLevelRow{{MinimumLevel: 15, MaximumLevel: 15, Probability: 10000}}}},
			ItemTypeProbability: [4]int32{0, 0, 0, 10000},
			Rarity:              [9]int32{1000000},
		}},
		Tables:    Tables{Gold: []float64{15, 1000, 0}},
		Equipment: &inventory.EquipmentCatalog{},
	}
	run := &dungeon.Session{RunID: "zero-difficulty", Definition: catalog.DungeonDefinition{ID: 11, BasisLevel: 15}, Maze: catalog.DungeonMaze{Rooms: make([]catalog.DungeonRoom, 1)}, Visited: map[uint32][]protocol.DungeonMonster{58605: nil}}
	run.MarkSceneCompleted()
	role := Role{ConfigVersion: s.Catalog.Source.SaveIdentity()}
	rules := CardRules{Model: "reference90-free-gold-v1", GoldNumerator: 175, GoldDenominator: 1000, Difficulty: []float64{1, 1.2, 1.5, 1.8, 2}}
	zero, err := s.PlanCards(role, run, rules, 42)
	if err != nil || zero.Gold != 175 || zero.ItemModel != ordinaryFreeCardModel {
		t.Fatalf("native zero difficulty blocked settlement: %+v %v", zero, err)
	}
	run.Difficulty = 1
	first, err := s.PlanCards(role, run, rules, 42)
	if err != nil || zero != first {
		t.Fatalf("zero difficulty uses a different column: %+v %+v %v", zero, first, err)
	}
	run.Difficulty = 6
	if _, err := s.PlanCards(role, run, rules, 42); err == nil {
		t.Fatal("out-of-range difficulty accepted")
	}
}

func TestCardGoldUsesRunDifficultyAndPreservesAbyss(t *testing.T) {
	s := Service{Tables: Tables{Gold: []float64{18, 1000, 0}}}
	rules := CardRules{GoldNumerator: 175, GoldDenominator: 1000, Difficulty: []float64{1, 1.2, 1.5, 1.8, 2}}
	run := &dungeon.Session{Difficulty: 5, Definition: catalog.DungeonDefinition{ID: 10}}
	gold, err := s.cardGoldForDungeon(run, rules, 42, 18)
	if err != nil || gold != 350 {
		t.Fatalf("highest ordinary difficulty got %d %v", gold, err)
	}
	s.Catalog.DungeonDropInfo = map[uint32][]catalog.DungeonDropRateEntry{10: {{DungeonType: "dgn_hell"}}}
	gold, err = s.cardGoldForDungeon(run, rules, 42, 18)
	if err != nil || gold != 175 {
		t.Fatalf("existing Abyss gold changed: %d %v", gold, err)
	}
	s.Catalog.DungeonDropInfo = nil
	s.Attunement = &AttunementRewards{Tables: []attunementDungeon{{Dungeon: 10}}}
	gold, err = s.cardGoldForDungeon(run, rules, 42, 18)
	if err != nil || gold != 175 {
		t.Fatalf("existing Attunement gold changed: %d %v", gold, err)
	}
	s.Attunement = nil
	for _, mode := range []string{"Odyssey", "Hell Party"} {
		run.Definition.Odyssey = mode == "Odyssey"
		if mode == "Hell Party" {
			run.HellPosition = &[2]byte{1, 1}
		}
		gold, err = s.cardGoldForDungeon(run, rules, 42, 18)
		if err != nil || gold != 175 {
			t.Fatalf("deferred %s gold changed: %d %v", mode, gold, err)
		}
	}
	run.Definition.Odyssey = false
	run.HellPosition = nil
	run.Difficulty = 6
	if _, err = s.cardGoldForDungeon(run, rules, 42, 18); err == nil {
		t.Fatal("out-of-policy difficulty silently clamped")
	}
}

func TestFrozenCardAwardsGrantEquipmentAndPreserveSave(t *testing.T) {
	c, err := catalog.LoadLoot("../../configs/loot.next25.json")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := inventory.LoadBagRules("../../configs/inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}
	equipment, err := inventory.LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	gear := equipment.DropPool()[0]
	s := Service{Catalog: c, BagRules: rules, Equipment: equipment}
	raw := json.RawMessage(`{"future_field":{"value":123}}`)
	before := string(raw)
	p := CardPlan{Gold: 42, Items: [8]Award{{Template: gear.ID, Amount: 1}}}
	state, err := s.grantCardAwards(raw, p)
	if err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.ReadBag(state)
	if err != nil {
		t.Fatal(err)
	}
	if bag.Gold != 42 || len(bag.Equipment) != 1 || bag.Equipment[0].Template != gear.ID || bag.Equipment[0].Durability != gear.Durability || bag.Equipment[0].Period != inventory.GrantExpireTime {
		t.Fatalf("bad card grant: %+v", bag)
	}
	var saved map[string]json.RawMessage
	if err = json.Unmarshal(state, &saved); err != nil || string(saved["future_field"]) != `{"value":123}` {
		t.Fatalf("unrelated save field changed: %s %v", state, err)
	}
	p.Items[1] = Award{Template: 0xffffffff, Amount: 1}
	failed, err := s.grantCardAwards(raw, p)
	if err == nil || failed != nil || string(raw) != before {
		t.Fatalf("partial failed award published: %s %v", failed, err)
	}
}
