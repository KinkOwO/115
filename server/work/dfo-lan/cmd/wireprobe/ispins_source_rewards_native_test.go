package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"dfolan/internal/loot"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestIspinsCurrentSourceRewardWrappers(t *testing.T) {
	path := os.Getenv("DFO_REPAIR_PVF")
	if path == "" {
		t.Skip("current source PVF required")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1 << 30}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	contents, err := catalog.ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := a.Tokens(catalog.IspinsOperationsPath)
	if err != nil {
		t.Fatal(err)
	}
	operations, err := catalog.ParseIspinsOperations(tokens)
	if err != nil {
		t.Fatal(err)
	}
	var ids []uint32
	for _, dungeon := range contents.Contents["Ispins"].Dungeons {
		ids = append(ids, dungeon.Dungeon)
	}
	cold, err := catalog.ImportRuntimeDungeons(a, ids)
	if err != nil {
		t.Fatal(err)
	}
	defer cold.CloseMapSource()
	encoded, err := json.Marshal(cold)
	if err != nil {
		t.Fatal(err)
	}
	var warm catalog.DungeonCatalog
	if err := json.Unmarshal(encoded, &warm); err != nil {
		t.Fatal(err)
	}
	if warm.LegionContents != nil {
		t.Fatal("test must exercise omitted source rules")
	}
	warm, err = catalog.RestoreRuntimeDungeons(a, warm)
	if err != nil {
		t.Fatal(err)
	}
	defer warm.CloseMapSource()
	if cold.LegionContents == nil || warm.LegionContents == nil || cold.LegionContents.Source.Checksum != warm.LegionContents.Source.Checksum || !reflect.DeepEqual(cold.LegionContents.Contents, warm.LegionContents.Contents) || !reflect.DeepEqual(cold.IspinsOperations, warm.IspinsOperations) {
		t.Fatalf("cold and warm Ispins reward sources differ: contents=%v operations=%v", reflect.DeepEqual(cold.LegionContents, warm.LegionContents), reflect.DeepEqual(cold.IspinsOperations, warm.IspinsOperations))
	}
	bindings := 0
	for index, op := range warm.IspinsOperations {
		if !catalog.IspinsOperationKindSupported(op.Type) || (op.Type == 6 && op.FixedValue == 0) {
			continue
		}
		for _, id := range ids {
			if _, err := catalog.IspinsPhaseRewards(warm.LegionContents, warm.IspinsOperations, id, index); err != nil {
				t.Fatalf("operation %d dungeon %d: %v", index, id, err)
			}
			bindings++
		}
	}
	if bindings != len(warm.IspinsOperations)*len(ids) {
		t.Fatal("no executable source reward binding")
	}
	t.Logf("cold/warm rules agree; %d source phase-operation bindings", bindings)
	c, err := gamedata.PrepareCatalogs(gamedata.CatalogInputs{Selection: "items,boosters", ArchivePath: path}, gamedata.CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	boxes := boosterBoxSource{catalog: &BoosterCatalog{Definitions: c.Boosters, Items: c.Items.Items}}
	roots := map[uint32]bool{}
	for _, row := range contents.Contents["Ispins"].Rewards {
		if row.Kind == "basic" {
			roots[uint32(row.Values[1])] = true
		}
	}
	for index, rule := range operations {
		if rule.Difficulty == "" || len(rule.Rewards) == 0 {
			t.Fatal("missing source operation", index)
		}
		for _, row := range rule.Rewards {
			roots[row.Template] = true
		}
	}
	checks := 0
	for root := range roots {
		for _, seed := range []uint32{1, 7, 1337} {
			products, _, skipped := loot.OpenRewardBoxes(seed, boxes, []loot.Award{{Template: root, Amount: 1}})
			if len(skipped) > 0 || len(products) == 0 {
				t.Fatal(root, seed, products, skipped)
			}
			for _, product := range products {
				item, ok := c.Items.Items[product.Template]
				if !ok || item.StackableType == "[booster]" || item.StackableType == "[virtual]" || product.Amount == 0 {
					t.Fatal("unresolved source product", product)
				}
			}
			checks++
			t.Logf("root=%d seed=%d products=%+v", root, seed, products)
		}
	}
	t.Logf("source %s: %d reward roots, %d wrapper checks", c.SourceChecksum, len(roots), checks)
}
