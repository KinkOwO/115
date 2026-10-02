package loot

import (
	"encoding/binary"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

// Current native source -> automatically discovered pool -> frozen plan ->
// inventory grant, including levels omitted by the old basic ID selection.
func TestCurrentPVFOrdinaryCardsPlanAndGrant(t *testing.T) {
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
	list, err := catalog.ResolveScript(a, "list/equipment.lst")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := catalog.ParseIndex(list.Cells)
	if err != nil {
		t.Fatal(err)
	}
	index := catalog.ItemIndex{Source: a.Snapshot(), IndexHashes: map[string]string{list.Path: list.SHA256}, Items: map[uint32]catalog.ItemIndexEntry{}}
	for _, r := range rows {
		p := r.Path
		if !strings.HasPrefix(p, "equipment/") {
			p = path.Join("equipment", p)
		}
		index.Items[r.ID] = catalog.ItemIndexEntry{ID: r.ID, Kind: "equipment", Path: p}
	}
	t.Setenv("DFO_ALLOW_TRADE_EQUIPMENT", "")
	pool, err := inventory.ImportOrdinaryDropPool(a, index, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(pool) == 0 {
		t.Fatal("native ordinary pool empty")
	}
	full, err := inventory.OpenPVFEquipmentCatalog(a, index)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	s := Service{Catalog: catalog.LootCatalog{Source: a.Snapshot(), Rules: map[string]catalog.ScriptRecord{}}, Equipment: &inventory.EquipmentCatalog{Source: a.Snapshot(), Full: full, OrdinaryPool: pool}}
	for _, p := range []string{"etc/itemdropinfo_monseter.etc", "etc/itemdropinfo_common.etc", "etc/itemdropinfo_clearreward.etc"} {
		sc, e := catalog.ResolveScript(a, p)
		if e != nil {
			t.Fatal(e)
		}
		s.Catalog.Rules[p] = sc
	}
	clear, err := catalog.ParseClearRewardTable(s.Catalog.Rules["etc/itemdropinfo_clearreward.etc"])
	if err != nil {
		t.Fatal(err)
	}
	s.Catalog.ClearReward = &clear
	s.Tables, err = Parse(s.Catalog)
	if err != nil {
		t.Fatal(err)
	}
	s.BagRules, err = inventory.LoadBagRules("../../configs/inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}
	policy := CardRules{Model: "reference90-free-gold-v1", GoldNumerator: 175, GoldDenominator: 1000, Difficulty: []float64{1, 1.2, 1.4, 1.6, 1.8}}
	role := Role{ConfigVersion: s.Catalog.Source.SaveIdentity(), State: json.RawMessage(`{"future_field":42}`)}
	dropRules, err := LoadRules("../../configs/drop.current36.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, level := range []byte{5, 15, 20, 55, 100, 115} {
		run := &dungeon.Session{RunID: "native-test", Difficulty: 4, Definition: catalog.DungeonDefinition{ID: 999}, Maze: catalog.DungeonMaze{Rooms: make([]catalog.DungeonRoom, 5)}, Visited: map[uint32][]protocol.DungeonMonster{1: nil, 2: nil, 3: nil}, Monsters: []protocol.DungeonMonster{{Entity: 1, Level: level}}}
		run.MarkSceneCompleted()
		// The live dungeon 11 / level 15 run reported difficulty 0. It must
		// freeze the same first-column plan as difficulty 1, without stopping
		// settlement. Exercise both with the current PVF and native pool.
		for seed := uint32(1); seed <= 20; seed++ {
			run.Difficulty = 0
			zero, e := s.PlanCards(role, run, policy, seed*2654435761)
			if e != nil {
				t.Fatalf("zero-difficulty settlement level %d: %v", level, e)
			}
			run.Difficulty = 1
			first, e := s.PlanCards(role, run, policy, seed*2654435761)
			if e != nil || zero != first || zero.Gold == 0 || zero.ItemModel != ordinaryFreeCardModel {
				t.Fatalf("zero difficulty differs from first column: %+v %+v %v", zero, first, e)
			}
		}
		run.Difficulty = 4
		hits := 0
		var selected CardPlan
		for seed := uint32(1); seed <= 2000; seed++ {
			plan, e := s.PlanCards(role, run, policy, seed*2654435761)
			if e != nil {
				t.Fatal(e)
			}
			if plan.Items[0].Template != 0 {
				hits++
				selected = plan
			}
		}
		t.Logf("native ordinary pool=%d level=%d free item hits=%d/2000", len(pool), level, hits)
		if hits < 850 || hits > 1050 {
			t.Fatalf("ordinary source card probability or candidates changed at %d: hits=%d", level, hits)
		}
		state, _, e := s.PrepareFrozenCard(role, selected, 2)
		if e != nil {
			t.Fatal(e)
		}
		bag, e := inventory.ReadBag(state)
		if e != nil || len(bag.Equipment) != 1 || bag.Equipment[0].Template != selected.Items[0].Template {
			t.Fatalf("frozen native card not granted: %+v %v", bag, e)
		}
		var saved map[string]json.RawMessage
		_ = json.Unmarshal(state, &saved)
		if string(saved["future_field"]) != "42" {
			t.Fatal("unrelated save field lost")
		}
		run.Definition.Odyssey = true
		old, e := s.PlanCards(role, run, policy, 42)
		if e != nil || old.Items != ([8]Award{}) || old.ItemModel != "" {
			t.Fatalf("deferred Odyssey changed: %+v %v", old, e)
		}
		run.Definition.Odyssey = false
		run.HellPosition = &[2]byte{1, 1}
		old, e = s.PlanCards(role, run, policy, 42)
		if e != nil || old.Items != ([8]Award{}) || old.ItemModel != "" {
			t.Fatalf("deferred Hell changed: %+v %v", old, e)
		}
		if level == 5 || level == 115 {
			run.HellPosition = nil
			run.Loaded = true
			run.Room.Map = 1
			run.Monsters[0].Rank = 3
			run.Dead = map[uint16]bool{1: true}
			found := false
			for seed := uint32(1); seed <= 200; seed++ {
				run.NextEntity = 10
				ls := NewSession(s.Catalog, s.Tables, dropRules, s.Equipment, run.RunID, 1, 1, 3)
				ls.seeds[1] = seed * 2654435761
				ground, e := ls.Death(run, 1)
				if e != nil {
					t.Fatal(e)
				}
				for _, row := range ground {
					award := ls.Objects[row.Object].Award
					if award.Template == 0 {
						continue
					}
					found = true
					durability, e := s.Equipment.Reward(award.Template)
					if e != nil || binary.LittleEndian.Uint16(row.Item[11:13]) != durability {
						t.Fatalf("ground gear source/record mismatch: %+v %v", award, e)
					}
				}
				if found {
					again, e := ls.Death(run, 1)
					if e != nil || len(again) != len(ground) {
						t.Fatal("confirmed death changed on replay")
					}
					break
				}
			}
			if !found {
				t.Fatalf("native ordinary ground gear never generated at %d", level)
			}
		}
	}
	// Replay the actual native dungeon from the reported coin-only run. Its
	// rate table declares only unique/legendary; generic stackables and basic
	// gear must reach scene objects and inventory with the unmodified PVF rates.
	nativeLoot, err := catalog.ImportLoot(a, 150)
	if err != nil {
		t.Fatal(err)
	}
	dropPolicy, err := inventory.ReadDropPolicy("../../configs/pvf-drop-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range dropPolicy.ExcludedLootIDs {
		delete(nativeLoot.Items, id)
	}
	dungeons, err := catalog.ImportDungeons(a, []uint32{11})
	if err != nil {
		t.Fatal(err)
	}
	run, err := dungeon.Select(dungeons, protocol.DungeonSelection{ID: 11, Party: 65535}, 15, nil)
	if err != nil || len(run.Monsters) == 0 {
		t.Fatalf("live dungeon source unavailable: %v", err)
	}
	run.Loaded = true
	entity := run.Monsters[0].Entity
	run.Dead = map[uint16]bool{entity: true}
	if run.Difficulty != 0 || run.Monsters[0].Level != 15 {
		t.Fatalf("live source boundary drifted: difficulty=%d monster=%+v", run.Difficulty, run.Monsters[0])
	}
	counts := map[string]int{}
	granted := map[string]bool{}
	for seed := uint32(1); seed <= 2000; seed++ {
		run.NextEntity = 60000
		ls := NewSession(nativeLoot, s.Tables, dropRules, s.Equipment, run.RunID, 1, 1, 3)
		ls.seeds[run.Room.Map] = seed * 2654435761
		rows, e := ls.Death(run, entity)
		if e != nil {
			t.Fatal(e)
		}
		for _, row := range rows {
			award := ls.Objects[row.Object].Award
			kind := "gold"
			if nativeLoot.Items[award.Template].Kind == "stackable" {
				kind = "stackable"
			} else if award.Template != 0 {
				kind = "map equipment"
				for _, item := range pool {
					if item.ID == award.Template {
						kind = "basic equipment"
						break
					}
				}
			}
			counts[kind]++
			if !granted[kind] && (kind == "stackable" || kind == "basic equipment") {
				awarder := inventory.Awarder{Catalog: nativeLoot, Equipment: s.Equipment, Rules: s.BagRules}
				state, _, e := awarder.Grant(json.RawMessage(`{"future_field":42}`), award.Template, award.Amount)
				if e != nil {
					t.Fatalf("native ground %s cannot enter inventory: %v", kind, e)
				}
				bag, e := inventory.ReadBag(state)
				if e != nil || kind == "stackable" && len(bag.Items) != 1 || kind == "basic equipment" && len(bag.Equipment) != 1 {
					t.Fatalf("native ground %s not granted: %+v %v", kind, bag, e)
				}
				granted[kind] = true
			}
		}
	}
	t.Logf("native dungeon 11 difficulty 0 / 2000 normal deaths: %v", counts)
	if counts["stackable"] == 0 || counts["basic equipment"] == 0 || !granted["stackable"] || !granted["basic equipment"] {
		t.Fatalf("unique/legendary map table still erases other classes: %v", counts)
	}
}
