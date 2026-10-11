package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"dfolan/internal/loot"
	"os"
	"testing"
)

// Exercises every currently payable normal source reward root with the same
// item/booster preparation and adapter used by the production gateway.
func TestBakalEveryNormalWrapperCurrentPVF(t *testing.T) {
	path := os.Getenv("DFO_REPAIR_PVF")
	if path == "" {
		t.Skip("set DFO_REPAIR_PVF for current source wrappers")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1 << 30}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	directory, err := catalog.ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	entrances, err := catalog.ImportRaidEntrances(a, &directory)
	if err != nil {
		t.Fatal(err)
	}
	var rules *catalog.BakalRaidRules
	for _, entry := range entrances {
		if entry.Bakal != nil {
			rules = entry.Bakal
			break
		}
	}
	if rules == nil {
		t.Fatal("current source has no Bakal binding")
	}
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "items,boosters", ArchivePath: path}, gamedata.CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	boxes := boosterBoxSource{catalog: &BoosterCatalog{Definitions: c.Boosters, Items: c.Items.Items}}
	roots, checks := 0, 0
	for mode, selected := range map[string]*catalog.BakalRaidRules{"normal": rules, "hard": entrances[82].BakalHard} {
		if selected == nil {
			t.Fatal("native mode reward source missing", mode)
		}
		for _, row := range selected.Rewards {
			if row.Kind != "party_card" && row.Kind != "squad_item" && row.Kind != "bidding_item" {
				continue
			}
			if row.Template == -1 {
				continue
			}
			roots++
			for _, seed := range []uint32{1, 7, 1337} {
				items, _, skipped := loot.OpenRewardBoxes(seed, boxes, []loot.Award{{Template: uint32(row.Template), Amount: 1}})
				for _, why := range skipped {
					if row.Kind != "bidding_item" || why != "attunement_empty_prize_4294967295" {
						t.Fatalf("source root%d seed%d unresolved: %v", row.Template, seed, skipped)
					}
				}
				if len(items) == 0 && len(skipped) == 0 {
					t.Fatalf("source root%d paid no products", row.Template)
				}
				for _, item := range items {
					if item.Amount == 0 || !boxes.Item(item.Template) || boxes.Container(item.Template) {
						t.Fatalf("root%d invalid product: %+v", row.Template, item)
					}
				}
				checks++
			}
			t.Logf("%s source root kind=%s template=%d weight=%d", mode, row.Kind, row.Template, row.Weight)
		}
		for group, rows := range selected.Bidding.WeeklyItems {
			for _, row := range rows {
				definition, ok := c.Items.Items[row.Template]
				if !ok || definition.Kind != "stackable" {
					t.Fatal("auction preview requires native stackable branch", mode, group, row.Template)
				}
				for _, seed := range []uint32{1, 7, 1337} {
					items, _, skipped := loot.OpenRewardBoxes(seed, boxes, []loot.Award{{Template: row.Template, Amount: 1}})
					for _, why := range skipped {
						if why != "attunement_empty_prize_4294967295" {
							t.Fatal(mode, group, row.Template, why)
						}
					}
					for _, item := range items {
						if item.Amount == 0 || !boxes.Item(item.Template) || boxes.Container(item.Template) {
							t.Fatal("unresolved weekly auction product", mode, group, item)
						}
					}
					checks++
				}
				roots++
			}
		}
	}
	if roots == 0 {
		t.Fatal("source reward table empty")
	}
	t.Logf("current source %s: %d roots, %d resolution checks", c.SourceChecksum, roots, checks)
}
