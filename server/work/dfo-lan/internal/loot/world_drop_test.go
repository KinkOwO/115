package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWorldDropCompatibilityProbabilityAndNoRenormalization(t *testing.T) {
	for v, w := range map[string]uint32{"": 100, " 100 ": 100, "0": 0, "200": 200, "10000": 10000} {
		got, err := ParseWorldDropPercent(v)
		if err != nil || got != w {
			t.Fatalf("policy %q %d %v", v, got, err)
		}
	}
	for _, v := range []string{"-1", "10001", "1.5", "NaN", "bad"} {
		if _, err := ParseWorldDropPercent(v); err == nil {
			t.Fatalf("invalid multiplier %q", v)
		}
	}
	c := catalog.LootCatalog{OrdinaryWorldDropPercent: 100, WorldDrop: &catalog.WorldDropTable{Levels: map[uint32]catalog.WorldDropLevel{15: {Items: []catalog.MonsterItemPair{{Template: 1, Value: 850}, {Template: 2, Value: 830}, {Template: 3, Value: 830}, {Template: 4, Value: 830}, {Template: 5, Value: 830}, {Template: 6, Value: 830}, {Template: 7, Value: 1000}, {Template: 8, Value: 1000}, {Template: 99, Value: 6}}}}}, Items: map[uint32]catalog.LootItem{}}
	for id := uint32(1); id <= 8; id++ {
		c.Items[id] = catalog.LootItem{ID: id, Kind: "stackable"}
	}
	hits, missed := 0, 0
	counts := map[uint32]int{}
	for i := uint32(1); i <= 100000; i++ {
		out, err := rollWorldItems(c, i*2654435761, 15)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Awards) > 0 {
			hits++
			counts[out.Awards[0].Template]++
		}
		if len(out.SkippedKinds) > 0 {
			missed++
		}
	}
	if hits < 6900 || hits > 7100 || counts[7] < 900 || counts[7] > 1100 || missed == 0 {
		t.Fatalf("source probability changed: hits%d counts%v refused%d", hits, counts, missed)
	}
	c.MonsterItemExclusions = map[uint32]bool{7: true}
	for i := uint32(1); i < 200; i++ {
		base := c
		base.MonsterItemExclusions = nil
		a, _ := rollWorldItems(base, i, 15)
		b, _ := rollWorldItems(c, i, 15)
		if a.NextSeed != b.NextSeed || len(a.Awards) > 0 && a.Awards[0].Template != 7 && !reflect.DeepEqual(a.Awards, b.Awards) {
			t.Fatal("exclusion rerolled or reweighted")
		}
	}
	c.OrdinaryWorldDropPercent = 0
	out, err := rollWorldItems(c, 42, 15)
	if err != nil || len(out.Awards) != 0 || out.NextSeed != 42 {
		t.Fatal("disabled world advances stream")
	}
	c.OrdinaryWorldDropPercent = 100
	for _, lv := range []byte{0, 14, 200} {
		out, err := rollWorldItems(c, 42, lv)
		if err != nil || out.NextSeed != 42 {
			t.Fatal("absent/out of reference range consumes RNG")
		}
	}
}

func TestCurrentPVFWorldDropDeathGrantAndDeferredModeIsolation(t *testing.T) {
	p := filepath.Join("..", "..", "..", "client-build", "Script.inner.pvf")
	if _, err := os.Stat(p); err != nil {
		t.Skip("current PVF absent")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1 << 30}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	c := catalog.LootCatalog{Source: a.Snapshot(), MaximumGrade: 150, Items: map[uint32]catalog.LootItem{}, OrdinaryWorldDropPercent: 10000}
	if err := c.EnableWorldDrop(a); err != nil {
		t.Fatal(err)
	}
	if err := c.EnableMonsterItemDetails(a); err != nil {
		t.Fatal(err)
	}
	defer c.CloseDetails()
	wanted := map[uint32]bool{}
	for _, lv := range []uint32{15, 16, 17, 18} {
		for _, p := range c.WorldDrop.Levels[lv].Items {
			if p.Template > 0 && p.Value > 0 {
				wanted[uint32(p.Template)] = true
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
	for _, r := range rows {
		if !wanted[r.ID] {
			continue
		}
		path := r.Path
		if !strings.HasPrefix(path, "stackable/") {
			path = "stackable/" + path
		}
		s, err := catalog.ResolveScript(a, path)
		if err != nil {
			t.Fatal(err)
		}
		it := catalog.ItemIndexEntry{ID: r.ID, Kind: "stackable", Path: path}
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
		index.Items[r.ID] = it
		t.Logf("world item%d type%s path%s", r.ID, it.StackableType, path)
	}
	if len(index.Items) != len(wanted) {
		t.Fatalf("world pool lacks native STK definitions: %d/%d", len(index.Items), len(wanted))
	}
	if err := c.SupplementItemIndex(index); err != nil {
		t.Fatal(err)
	}
	if err := c.EnableRuntimeDetails(a, index); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	tables := Tables{Probability: []float64{1, 200, 10000, 0, 0, 0, 0}, Gold: []float64{15, 100, 0, 16, 100, 0, 17, 100, 0, 18, 100, 0}, Grade: []float64{15, 7, 3, 16, 7, 3, 17, 7, 3, 18, 7, 3}, Rank: make([]float64, 20), Rarity: make([]float64, 36)}
	for i := range tables.Rank {
		tables.Rank[i] = 1
	}
	rules := Rules{Denominator: 10000, DifficultyBonus: []float64{1, 1, 1, 1, 1}, SupportedKinds: []string{"gold", "stackable"}}
	br, err := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	awarder := inventory.Awarder{Catalog: c, Rules: br}
	seen := map[uint32]bool{}
	for _, lv := range []byte{15, 16, 17, 18} {
		for seed := uint32(1); seed < 250; seed++ {
			d := &dungeon.Session{RunID: "world-source", Loaded: true, NextEntity: 10, Room: catalog.DungeonRoom{Map: 1}, Definition: catalog.DungeonDefinition{ID: 13}, Monsters: []protocol.DungeonMonster{{Entity: 1, Template: 109014957, Level: lv}}, Dead: map[uint16]bool{1: true}}
			s := NewSession(c, tables, rules, nil, d.RunID, 1, 1, 3)
			s.seeds[1] = seed * 2654435761
			drops, err := s.Death(d, 1)
			if err != nil || len(drops) != 2 {
				t.Fatalf("world death level%d seed%d drops%d %v skip%v", lv, seed, len(drops), err, s.Skipped)
			}
			after := s.seeds[1]
			again, err := s.Death(d, 1)
			if err != nil || !reflect.DeepEqual(drops, again) || after != s.seeds[1] || len(s.Objects) != 2 {
				t.Fatal("replayed world drop duplicated")
			}
			for _, obj := range s.Objects {
				if obj.Award.Template == 0 {
					continue
				}
				seen[obj.Award.Template] = true
				raw := json.RawMessage(`{"future_field":{"value":123}}`)
				saved, receipt, err := awarder.Grant(raw, obj.Award.Template, obj.Award.Amount)
				if err != nil {
					t.Fatal(err)
				}
				bag, err := inventory.ReadBag(saved)
				if err != nil || len(receipt.Slots) != 1 || len(bag.Items) != 1 || bag.Items[0].Template != obj.Award.Template || bag.Items[0].Amount != 1 || bag.Items[0].ExpireTime != inventory.GrantExpireTime || !strings.Contains(string(saved), `"future_field":{"value":123}`) {
					t.Fatalf("world grant loses saved data: %s %v", saved, err)
				}
			}
		}
	}
	for _, id := range []uint32{3030, 3028, 3027, 3142, 3151, 3156, 1107, 1113, 1108, 1114} {
		if !seen[id] {
			t.Fatalf("native world template%d never generated", id)
		}
	}
	for _, mode := range []string{"Odyssey", "Hell", "Abyss", "Attunement", "excluded", "unowned", "NonCombat", "APC", "disabled"} {
		t.Run(mode, func(t *testing.T) {
			cc := c
			d := &dungeon.Session{RunID: "isolate-world", Loaded: true, NextEntity: 10, Room: catalog.DungeonRoom{Map: 1}, Definition: catalog.DungeonDefinition{ID: 13}, Monsters: []protocol.DungeonMonster{{Entity: 1, Template: 109014957, Level: 15}}, Dead: map[uint16]bool{1: true}}
			switch mode {
			case "Odyssey":
				d.Definition.Odyssey = true
			case "Hell":
				d.HellPosition = &[2]byte{1, 1}
			case "Abyss":
				cc.DungeonDropInfo = map[uint32][]catalog.DungeonDropRateEntry{13: {{DungeonType: "dgn_hell"}}}
			case "excluded":
				d.Definition.Script.Cells = []pvf.Token{{Type: 3, Text: "[exclude monster random drop]"}}
			case "unowned":
				d.Unowned = map[uint16]bool{1: true}
			case "NonCombat":
				d.Monsters[0].NonCombat = true
			case "APC":
				d.Monsters[0].APC = true
			case "disabled":
				cc.OrdinaryWorldDropPercent = 0
			}
			s := NewSession(cc, tables, rules, nil, d.RunID, 1, 1, 3)
			if mode == "Attunement" {
				s.Attunement = &AttunementRewards{Tables: []attunementDungeon{{Dungeon: 13}}}
			}
			s.seeds[1] = 42
			if _, err := s.Death(d, 1); err != nil {
				t.Fatal(err)
			}
			for _, obj := range s.Objects {
				if obj.Award.Template != 0 {
					t.Fatalf("world crossed %s boundary: %+v", mode, obj)
				}
			}
		})
	}
}
