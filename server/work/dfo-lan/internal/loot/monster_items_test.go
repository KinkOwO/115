package loot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

func TestMonsterItemCompatibilityRateAndSourceWeights(t *testing.T) {
	for value, want := range map[string]uint32{"": 1000, " 10 ": 1000, "0": 0, "100": 10000} {
		got, err := ParseMonsterItemDropPercent(value)
		if err != nil || got != want {
			t.Fatalf("percent %q: %d %v", value, got, err)
		}
	}
	for _, value := range []string{"101", "-1", "0.5", "NaN", "garbage"} {
		if _, err := ParseMonsterItemDropPercent(value); err == nil {
			t.Fatalf("invalid policy accepted: %q", value)
		}
	}
	c := catalog.LootCatalog{OrdinaryMonsterItemRate: 1000, Items: map[uint32]catalog.LootItem{1: {Kind: "stackable"}, 2: {Kind: "stackable"}, 3: {Kind: "equipment"}, 4: {Kind: "stackable", StackableType: "[quest]"}}, MonsterItemExclusions: map[uint32]bool{9: true}}
	table := catalog.MonsterItemTable{Declared: true, Items: []catalog.MonsterItemPair{{Template: 1, Value: 1}, {Template: 2, Value: 9}, {Template: 3, Value: 100}, {Template: 4, Value: 100}, {Template: 9, Value: 100}, {Template: 0, Value: 100}, {Template: 1, Value: 0}}}
	hits, heavy := 0, 0
	for i := uint32(1); i <= 10000; i++ {
		out, err := rollMonsterItems(c, table, i*2654435761)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Awards) > 0 {
			hits++
			if out.Awards[0].Template == 2 {
				heavy++
			}
			if out.Awards[0].Amount != 1 || out.Awards[0].Template > 2 {
				t.Fatalf("invalid award: %+v", out)
			}
		}
	}
	if hits < 900 || hits > 1100 || heavy*100/hits < 85 {
		t.Fatalf("trigger/selection wrong: hits=%d heavy=%d", hits, heavy)
	}
	c.OrdinaryMonsterItemRate = 0
	out, err := rollMonsterItems(c, table, 42)
	if err != nil || len(out.Awards) != 0 || out.NextSeed != 42 {
		t.Fatalf("disabled stream changed: %+v %v", out, err)
	}
}

func TestCurrentPVFMonsterMaterialsAndConsumablesDeathGrantAndIsolation(t *testing.T) {
	p := os.Getenv("DFO_LOOT_PVF")
	if p == "" {
		p = filepath.Join("..", "..", "..", "client-build", "Script.inner.pvf")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip("current PVF absent")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1 << 30}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	c := catalog.LootCatalog{Source: a.Snapshot(), MaximumGrade: 150, ClearReward: &catalog.ClearRewardTable{}, Items: map[uint32]catalog.LootItem{}, OrdinaryMonsterItemRate: 10000}
	if err := c.EnableMonsterItemDetails(a); err != nil {
		t.Fatal(err)
	}
	defer c.CloseDetails()
	ids := map[uint32]bool{}
	for _, mob := range []uint32{1, 70, 65005} {
		table, known, err := c.MonsterItemTable(mob)
		if err != nil || !known || len(table.Items) == 0 {
			t.Fatalf("MOB%d: %+v %v", mob, table, err)
		}
		for _, pair := range table.Items {
			if pair.Template > 0 && pair.Value > 0 {
				ids[uint32(pair.Template)] = true
			}
		}
	}
	list, err := catalog.ResolveScript(a, "list/stackable.lst")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := catalog.ParseIndex(list.Cells)
	if err != nil {
		t.Fatal(err)
	}
	index := catalog.ItemIndex{Source: a.Snapshot(), Items: map[uint32]catalog.ItemIndexEntry{}}
	for _, row := range rows {
		if !ids[row.ID] {
			continue
		}
		p := row.Path
		if !strings.HasPrefix(p, "stackable/") {
			p = "stackable/" + p
		}
		s, err := catalog.ResolveScript(a, p)
		if err != nil {
			t.Fatal(err)
		}
		it := catalog.ItemIndexEntry{ID: row.ID, Kind: "stackable", Path: p}
		tag := ""
		for _, v := range s.Cells {
			if v.Type == 3 {
				tag = v.Text
				continue
			}
			if tag == "[stackable type]" && v.Type == 6 {
				it.StackableType = v.Text
			}
			if tag == "[stack limit]" && v.Type == 0 && v.Value >= 0 {
				it.StackLimit = uint32(v.Value)
			}
		}
		index.Items[row.ID] = it
		t.Logf("MOB item%d type%s source%s", row.ID, it.StackableType, p)
	}
	if err := c.SupplementItemIndex(index); err != nil {
		t.Fatal(err)
	}
	if err := c.EnableRuntimeDetails(a, index); err != nil {
		t.Fatal(err)
	}
	// All source views must survive closing the parent importer.
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	tables := Tables{Probability: []float64{1, 200, 10000, 0, 0, 0, 0}, Gold: []float64{15, 100, 0}, Grade: []float64{15, 7, 3}, Rank: make([]float64, 20), Rarity: make([]float64, 36), Difficulty: make([]float64, 25)}
	for _, values := range [][]float64{tables.Rank, tables.Difficulty} {
		for i := range values {
			values[i] = 1
		}
	}
	rules := Rules{Denominator: 10000, DifficultyBonus: []float64{1, 1, 1, 1, 1}, SupportedKinds: []string{"gold", "stackable"}}
	bagRules, err := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	awarder := inventory.Awarder{Catalog: c, Rules: bagRules}
	sawMaterial, sawConsumable := false, false
	for _, mob := range []uint32{1, 70, 65005} {
		run := &dungeon.Session{RunID: "MOB-native", Loaded: true, NextEntity: 10, Room: catalog.DungeonRoom{Map: 58605}, Definition: catalog.DungeonDefinition{ID: 11}, Monsters: []protocol.DungeonMonster{{Entity: 1, Template: mob, Level: 15}}, Dead: map[uint16]bool{1: true}}
		for seed := uint32(1); seed <= 50; seed++ {
			s := NewSession(c, tables, rules, nil, run.RunID, 1, 1, 3)
			s.seeds[58605] = seed
			drops, err := s.Death(run, 1)
			if err != nil || len(drops) != 2 {
				t.Fatalf("MOB%d seed%d drops%d %v skip%v", mob, seed, len(drops), err, s.Skipped)
			}
			after := s.seeds[58605]
			again, err := s.Death(run, 1)
			if err != nil || !reflect.DeepEqual(drops, again) || after != s.seeds[58605] || len(s.Objects) != 2 {
				t.Fatal("death replay advanced stream or duplicated")
			}
			for _, drop := range s.Objects {
				if drop.Award.Template == 0 {
					continue
				}
				typ := c.Items[drop.Award.Template].StackableType
				sawMaterial = sawMaterial || strings.HasPrefix(typ, "[material")
				sawConsumable = sawConsumable || typ == "[waste]"
				raw := json.RawMessage(`{"future_field":{"value":123}}`)
				saved, receipt, err := awarder.Grant(raw, drop.Award.Template, drop.Award.Amount)
				if err != nil {
					t.Fatal(err)
				}
				bag, err := inventory.ReadBag(saved)
				if err != nil || len(receipt.Slots) != 1 || len(bag.Items) != 1 || bag.Items[0].Slot != receipt.Slots[0] || bag.Items[0].Template != drop.Award.Template || bag.Items[0].Amount != 1 || bag.Items[0].ExpireTime != inventory.GrantExpireTime || !strings.Contains(string(saved), `"future_field":{"value":123}`) {
					t.Fatalf("grant lost source/save: %s %v", saved, err)
				}
			}
		}
	}
	if !sawMaterial || !sawConsumable {
		t.Fatalf("native coverage material=%v consumable=%v", sawMaterial, sawConsumable)
	}
	for _, mode := range []string{"Odyssey", "Hell", "Abyss", "Attunement", "excluded", "unowned", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			cc := c
			run := &dungeon.Session{RunID: "isolated", Loaded: true, NextEntity: 10, Room: catalog.DungeonRoom{Map: 1}, Definition: catalog.DungeonDefinition{ID: 11}, Monsters: []protocol.DungeonMonster{{Entity: 1, Template: 70, Level: 15}}, Dead: map[uint16]bool{1: true}}
			switch mode {
			case "Odyssey":
				run.Definition.Odyssey = true
			case "Hell":
				run.HellPosition = &[2]byte{1, 1}
			case "Abyss":
				cc.DungeonDropInfo = map[uint32][]catalog.DungeonDropRateEntry{11: {{DungeonType: "dgn_hell"}}}
			case "excluded":
				run.Definition.Script.Cells = []pvf.Token{{Type: 3, Text: "[exclude monster random drop]"}}
			case "unowned":
				run.Unowned = map[uint16]bool{1: true}
			case "disabled":
				cc.OrdinaryMonsterItemRate = 0
			}
			s := NewSession(cc, tables, rules, nil, run.RunID, 1, 1, 3)
			if mode == "Attunement" {
				s.Attunement = &AttunementRewards{Tables: []attunementDungeon{{Dungeon: 11}}}
			}
			s.seeds[1] = 42
			if _, err := s.Death(run, 1); err != nil {
				t.Fatal(err)
			}
			for _, drop := range s.Objects {
				if drop.Award.Template != 0 {
					t.Fatalf("MOB award crossed mode: %+v", drop)
				}
			}
		})
	}
}
