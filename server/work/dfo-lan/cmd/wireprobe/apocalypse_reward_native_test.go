package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

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
	for i, diff := range []string{"normal", "expert", "master"} {
		r, err := catalog.NativePhaseRewards(&contents, "Venus Legion", legion.VenusStageDungeons[legion.VenusEndpoint(byte(i))], uint16(i+1), diff, diff)
		if err != nil {
			t.Fatal(err)
		}
		for _, pool := range [][]catalog.LegionOperationReward{r.Basic, r.Additional} {
			for _, root := range pool {
				for seed := uint32(0); seed < 50; seed++ {
					src := &apocalypseBundleSource{RewardBoxSource: boxes, catalog: cat, root: root.Template}
					items, _, skipped := loot.OpenRewardBoxes(seed, src, []loot.Award{{Template: root.Template, Amount: root.Amount}})
					if src.err != nil {
						t.Fatal(diff, root, src.err)
					}
					for _, why := range skipped {
						if why != "attunement_empty_prize_4294967295" {
							t.Fatal(diff, root, why)
						}
					}
					state := json.RawMessage(`{"keep":true}`)
					for _, item := range items {
						state, _, err = awarder.Grant(state, item.Template, item.Amount)
						if err != nil {
							t.Fatal(diff, item, err)
						}
					}
				}
			}
		}
	}
	for _, diff := range []string{"normal", "expert", "master"} {
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
