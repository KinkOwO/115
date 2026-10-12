package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/json"
	"os"
	"testing"
)

func TestNativeLegionActualSourceRewardProducts(t *testing.T) {
	path := os.Getenv("DFO_REPAIR_PVF")
	if path == "" {
		t.Skip("native PVF required")
	}
	a, err := catalog.OpenTestArchiveCached(path, "")
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
	awarder := inventory.Awarder{Catalog: cat, Rules: rules, Equipment: &inventory.EquipmentCatalog{Source: a.Snapshot(), Full: full}}
	boxes := boosterBoxSource{catalog: &catalog.BoosterCatalog{Definitions: basics.Boosters, Items: basics.Index.Items}}
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		c := contents.Contents[name]
		seen := map[uint32]bool{}
		for mode, ids := range c.NativeOperations.Modes {
			for _, id := range ids {
				op := c.NativeOperations.Definitions[id]
				if op.Probability == 0 {
					continue
				}
				rewards, e := catalog.NativePhaseRewards(&contents, name, c.Dungeons[0].Dungeon, id, mode, "normal")
				if e != nil {
					t.Fatal(name, mode, id, e)
				}
				for _, global := range c.NativeOperations.ClearRewards {
					rewards.GlobalClear = append(rewards.GlobalClear, catalog.LegionOperationReward{Template: global.Template, Amount: global.Amount, Weight: global.Weight})
				}
				for _, row := range append(append(rewards.Basic, rewards.Additional...), rewards.GlobalClear...) {
					if seen[row.Template] {
						continue
					}
					seen[row.Template] = true
					src := nativeLegionRewardSource{RewardBoxSource: boxes, catalog: cat, roots: map[uint32]bool{row.Template: true}}
					for seed := uint32(0); seed < 50; seed++ {
						products, _, skipped := loot.OpenRewardBoxes(seed, src, []loot.Award{{Template: row.Template, Amount: row.Amount}})
						if len(skipped) != 0 || len(products) == 0 {
							t.Fatal(name, mode, id, row, products, skipped)
						}
						state := json.RawMessage(`{"keep":true}`)
						for _, item := range products {
							state, _, e = awarder.Grant(state, item.Template, item.Amount)
							if e != nil {
								t.Fatal(name, mode, id, row, item, e)
							}
						}
					}
				}
			}
		}
		t.Logf("source=%s content=%s verified reward roots=%d", a.Snapshot().Checksum, name, len(seen))
	}
}
