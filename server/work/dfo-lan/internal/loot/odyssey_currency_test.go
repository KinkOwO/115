package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"reflect"
	"testing"
)

func TestPermanentOdysseyCurrencyPolicy(t *testing.T) {
	r, e := LoadOdysseyCurrency("../../configs/odyssey-currency.json")
	if e != nil {
		t.Fatal(e)
	}
	if r.Rates != [4]uint32{1000, 10000, 10000, 10000} || r.Templates != [4]uint32{10418036, 10418036, 10418036, 10418035} {
		t.Fatal("operator policy changed")
	}
	count := 0
	for seed := uint32(0); seed < 10000; seed++ {
		a, next, e := r.Roll(seed, 0)
		if e != nil {
			t.Fatal(e)
		}
		count += len(a)
		b, again, e := r.Roll(seed, 0)
		if e != nil || next != again || !reflect.DeepEqual(a, b) {
			t.Fatal("unstable roll")
		}
		for rank := byte(1); rank <= 3; rank++ {
			v, _, e := r.Roll(seed, rank)
			if e != nil || len(v) != 1 || v[0] != (Award{r.Templates[rank], 1}) {
				t.Fatal("elite/boss policy", v, e)
			}
		}
	}
	if count < 850 || count > 1150 {
		t.Fatal("normal frequency", count)
	}
	t.Logf("operator rates: normal %d/10000 deterministic seeds; elites/bosses 100%%", count)
}

func TestOdysseyCurrencySceneRetryAndPoolIsolation(t *testing.T) {
	c, e := catalog.LoadLoot("../../configs/loot.level150.json")
	if e != nil {
		t.Fatal(e)
	}
	rules, e := LoadRules("../../configs/drop.current36.json")
	if e != nil {
		t.Fatal(e)
	}
	tables, e := Parse(c)
	if e != nil {
		t.Fatal(e)
	}
	currency, e := LoadOdysseyCurrency("../../configs/odyssey-currency.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []uint32{10418035, 10418036} {
		if _, ok := c.Items[id]; ok {
			t.Fatal("currency leaks into generic pool")
		}
	}
	for level := byte(1); level <= 115; level++ {
		for rank := byte(0); rank < 4; rank++ {
			if _, e := Roll(c, tables, rules, nil, 1, level, rank, 0); e != nil {
				t.Fatalf("level%d rank%d: %v", level, rank, e)
			}
		}
	}
	d := &dungeon.Session{RunID: "run", Loaded: true, Definition: catalog.DungeonDefinition{Odyssey: true}, Room: catalog.DungeonRoom{Map: 1}, Monsters: []protocol.DungeonMonster{{Entity: 4096, Level: 115, Rank: 3}}, Dead: map[uint16]bool{4096: true}, NextEntity: 4097}
	s := NewSession(c, tables, rules, nil, "run", 1, 1, 1)
	s.Currency = currency
	rows, e := s.Death(d, 4096)
	if e != nil {
		t.Fatal(e)
	}
	coins := 0
	for _, drop := range s.Objects {
		if drop.Award.Template == 10418035 {
			coins++
			if drop.Award.Amount != 1 {
				t.Fatal(drop)
			}
		}
	}
	if coins != 1 {
		t.Fatal("missing boss coin", coins)
	}
	count := len(s.Objects)
	again, e := s.Death(d, 4096)
	if e != nil || !reflect.DeepEqual(rows, again) || len(s.Objects) != count {
		t.Fatal("duplicate death rerolled")
	}
	b := inventory.Bag{Version: "ordinary-bag-v1"}
	bagRules := inventory.BagRules{Source: c.Source.Checksum, MissingStackLimit: 1000}
	overlay := currency.StorageCatalog(c)
	bagRules = currency.BagRules(bagRules)
	b, slot, e := b.Add(overlay, bagRules, 10418035, 1)
	if e != nil || slot != 65 {
		t.Fatal(slot, e)
	}
	b, slot, e = b.Add(overlay, bagRules, 10418035, 1)
	if e != nil || slot != 65 || b.Items[0].Amount != 2 {
		t.Fatal("coin merge", b, e)
	}
	if _, ok := c.Items[10418035]; ok {
		t.Fatal("storage overlay mutated drop pool")
	}
}

func TestLevel150CatalogPreservesExistingPool(t *testing.T) {
	old, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	full, e := catalog.LoadLoot("../../configs/loot.level150.json")
	if e != nil {
		t.Fatal(e)
	}
	if old.Source.Checksum != full.Source.Checksum || full.MaximumGrade != 150 || !reflect.DeepEqual(old.Rules, full.Rules) {
		t.Fatal("source tables changed")
	}
	for id, item := range old.Items {
		if !reflect.DeepEqual(item, full.Items[id]) {
			t.Fatal("old item changed", id)
		}
	}
	for id, item := range full.Items {
		if item.StackableType != "[material]" && item.StackableType != "[throw]" {
			t.Fatal("unmapped drop type", id)
		}
	}
	t.Logf("source pool retained %d old items, extended to %d; all types supported", len(old.Items), len(full.Items))
}
