package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"
	"time"
)

type bundleTestBoxes map[uint32]loot.RewardBox

func (b bundleTestBoxes) RewardBox(id uint32) (loot.RewardBox, bool) { v, ok := b[id]; return v, ok }
func (b bundleTestBoxes) Item(id uint32) bool                        { return id != math.MaxUint32 }
func (b bundleTestBoxes) Container(id uint32) bool                   { _, ok := b[id]; return ok }

func TestApocalypseBundleFollowsSourceKeepsBoxesAndOpensInstantWrappers(t *testing.T) {
	s, w, svc, _ := terminalApocalypseFixture(t)
	cat := svc.Awarder.Catalog
	cat.Items[78] = catalog.LootItem{Kind: "stackable", StackableType: "[virtual]", Script: catalog.ScriptRecord{Cells: []pvf.Token{{Text: "[instantly open]"}}}}
	cat.Items[79] = catalog.LootItem{Kind: "stackable", StackableType: "[booster]"}
	boxes := bundleTestBoxes{
		77: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 99, Weight: 1, Count: 7}}}, {Candidates: []loot.RewardBoxCandidate{{Template: 78, Weight: 1, Count: 2}}}, {Candidates: []loot.RewardBoxCandidate{{Template: 79, Weight: 1, Count: 1}}}, {Candidates: []loot.RewardBoxCandidate{{Template: math.MaxUint32, Weight: 1, Count: 1}}}}},
		78: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 99, Weight: 1, Count: 3}}}}},
		79: {Pools: []loot.RewardBoxPool{{Candidates: []loot.RewardBoxCandidate{{Template: 99, Weight: 1, Count: 28}}}}},
	}
	a, err := apocalypseBasicProducts(s.contents, cat, boxes, "normal", w.apocalypseNavigation.RunID)
	if err != nil || len(a) != 2 || a[0].Template != 99 || a[0].Amount != 13 || a[1].Template != 79 || a[1].Amount != 1 {
		t.Fatal(a, err)
	}
	again, err := apocalypseBasicProducts(s.contents, cat, boxes, "normal", w.apocalypseNavigation.RunID)
	if err != nil || !reflect.DeepEqual(a, again) {
		t.Fatal("retry rerolled", again, err)
	}
	delete(boxes, 78)
	if _, err := apocalypseBasicProducts(s.contents, cat, boxes, "normal", "run"); err == nil {
		t.Fatal("unresolved instant box silently discarded")
	}
}

func TestApocalypseBasicBundleNativeSource(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("native PVF required")
	}
	a, err := catalog.OpenTestArchiveCached(p, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	contents, err := catalog.ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	basics, err := catalog.ImportItemBasics(a, catalog.ItemBasicOptions{Boosters: true})
	if err != nil {
		t.Fatal(err)
	}
	cat := catalog.LootCatalog{Source: a.Snapshot(), Items: map[uint32]catalog.LootItem{}}
	for id, item := range basics.Index.Items {
		cat.Items[id] = catalog.LootItem{ID: id, Kind: item.Kind, StackableType: item.StackableType, StackLimit: item.StackLimit}
	}
	if err = cat.EnableRuntimeDetails(a, basics.Index); err != nil {
		t.Fatal(err)
	}
	defer cat.CloseDetails()
	full, err := inventory.OpenPVFEquipmentCatalog(a, basics.Index)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	rules, err := inventory.LoadBagRules("../../configs/inventory.next29.json", a.Snapshot().Checksum)
	if err != nil {
		t.Fatal(err)
	}
	rules.Slots["[booster]"] = [2]uint16{65, 120}
	awarder := inventory.Awarder{Catalog: cat, Rules: rules, Equipment: &inventory.EquipmentCatalog{Source: a.Snapshot(), Full: full}}
	boxes := boosterBoxSource{catalog: &catalog.BoosterCatalog{Definitions: basics.Boosters, Items: basics.Index.Items}}
	for _, diff := range []string{"normal", "expert"} {
		root, err := legion.GuaranteedBasicClearReward(contents, a.Snapshot().Checksum, diff)
		if err != nil {
			t.Fatal(err)
		}
		pools := basics.Boosters[root.Template].Pools
		for n := 0; n < 50; n++ {
			items, err := apocalypseBasicProducts(contents, cat, boxes, diff, fmt.Sprintf("native-test-%d", n))
			if err != nil {
				t.Fatal(diff, n, err)
			}
			amounts := map[uint32]uint32{}
			for _, item := range items {
				amounts[item.Template] += item.Amount
			}
			for _, pool := range pools {
				if len(pool.Candidates) != 1 {
					continue
				}
				fixed := pool.Candidates[0]
				script, err := cat.ItemScript(fixed.Template)
				if err != nil {
					t.Fatal(err)
				}
				instant := false
				for _, c := range script.Cells {
					if c.Text == "[instantly open]" {
						instant = true
					}
				}
				if !instant && amounts[fixed.Template] != fixed.Count*max(pool.DrawCount, 1) {
					t.Fatal("fixed source quantity changed", diff, fixed, amounts)
				}
			}
			state := json.RawMessage(`{"future":{"keep":true}}`)
			for _, item := range items {
				state, _, err = awarder.Grant(state, item.Template, item.Amount)
				if err != nil {
					t.Fatal("native bundle product cannot enter inventory", diff, n, item, err)
				}
			}
			if n == 0 {
				t.Logf("source=%s difficulty=%s root=%d products=%v", a.Snapshot().Checksum, diff, root.Template, items)
			}
		}
	}
}

func TestApocalypseCommittedBasicProductsReachInventoryAndNativeList(t *testing.T) {
	s, w, svc, _ := terminalApocalypseFixture(t)
	out, err := s.prepareApocalypseClear(w, svc, w.apocalypseNavigation.StartedAt.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	p := out.Packets[2].Payload
	if binary.LittleEndian.Uint32(p[1600:]) != 99 || binary.LittleEndian.Uint32(p[1604:]) != 20 {
		t.Fatal("source bundle products absent from result")
	}
}
