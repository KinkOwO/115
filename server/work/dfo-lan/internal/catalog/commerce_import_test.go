package catalog

import (
	"dfolan/internal/catalog/pvf"
	"reflect"
	"testing"
)

func TestItemMaterialCostsPreservesGoldAndMultiplePairs(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[need material]"}, {Type: 0, Value: 0}, {Type: 0, Value: 2000000}, {Type: 0, Value: 3037}, {Type: 0, Value: 1000}, {Type: 3, Text: "[name]"}, {Type: 6, Text: "irrelevant"}}
	got, err := ItemMaterialCosts(cells)
	want := []ItemMaterialCost{{Template: 0, Count: 2000000}, {Template: 3037, Count: 1000}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, err)
	}
	for _, bad := range [][]pvf.Token{
		{{Type: 3, Text: "[need material]"}, {Type: 0, Value: 1}},
		{{Type: 3, Text: "[need material]"}, {Type: 6, Text: "unknown"}, {Type: 0, Value: 1}},
		{{Type: 3, Text: "[need material]"}, {Type: 0, Value: 1}, {Type: 0, Value: 0}},
	} {
		if _, err = ItemMaterialCosts(bad); err == nil {
			t.Fatal("invalid costs accepted", bad)
		}
	}
}

func TestCommerceImportsRejectMissingOrDifferentSource(t *testing.T) {
	index := ItemIndex{}
	if _, err := ImportShopPrices(nil, index); err == nil {
		t.Fatal("price source missing")
	}
	if _, err := ImportItemMaterials(nil, index); err == nil {
		t.Fatal("material source missing")
	}
	if _, err := ImportBoosters(nil, index); err == nil {
		t.Fatal("reward source missing")
	}
}

func TestBoosterProjectionKeepsSmartDropPoolOrderAndUnresolvedMarkers(t *testing.T) {
	pools := []BoosterRewardPool{{DrawCount: 2, Candidates: []BoosterRewardCandidate{{Template: 490000001, Weight: 1000, Count: 1}}}, {DrawCount: 1, Candidates: []BoosterRewardCandidate{{Template: 15, Weight: 1000, Count: 2}}}}
	groups := map[uint32][]BoosterRewardCandidate{7: {{Template: 10, Weight: 30, Count: 1}, {Template: 20, Weight: 70, Count: 1}}}
	n := 0
	got := resolveSmartDrop(pools, 7, groups, &n)
	if n != 1 || got[0].DrawCount != 2 || !reflect.DeepEqual(got[0].Candidates, groups[7]) || !reflect.DeepEqual(got[1], pools[1]) {
		t.Fatal(got, n)
	}
	if got = resolveSmartDrop(pools, 9, groups, &n); !reflect.DeepEqual(got, pools) {
		t.Fatal("missing group was guessed", got)
	}
}
