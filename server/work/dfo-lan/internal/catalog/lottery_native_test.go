package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"path/filepath"
	"testing"
)

func TestLotteryPolicyRefusesOverlapAndEmbeddedRewardRows(t *testing.T) {
	for _, raw := range []string{
		`{"version":1,"item_pools":[7],"equipment_pools":[7]}`,
		`{"version":1,"item_pools":[7],"equipment_pools":[0]}`,
		`{"version":1,"item_pools":[7],"equipment_pools":[8],"candidates":[]}`,
		`{"version":1,"item_pools":[7],"equipment_pools":[8]} {}`,
	} {
		p := filepath.Join(t.TempDir(), "policy.json")
		if err := os.WriteFile(p, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadLotteryPolicy(p); err == nil {
			t.Fatal("invalid policy accepted", raw)
		}
	}
}

func TestLotterySourceParserKeepsUnsupportedCellsUnavailable(t *testing.T) {
	start, end := pvf.Token{Type: 3, Text: "[int data]"}, pvf.Token{Type: 3, Text: "[/int data]"}
	gold := []pvf.Token{start, {Type: 0, Value: 0}, {Type: 0, Value: 10000}, {Type: 0, Value: 1000000}, end}
	if got := ParseLotteryCells(gold); len(got) != 3 || got[0] != 0 || got[2] != 1000000 {
		t.Fatal("gold triple changed", got)
	}
	for _, cells := range [][]pvf.Token{
		{start, {Type: 0, Value: 7}, end},
		{start, {Type: 6, Text: "unresolved"}, {Type: 0, Value: 1}, {Type: 0, Value: 1}, end},
	} {
		if ParseLotteryCells(cells) != nil {
			t.Fatal("unsupported source accepted")
		}
	}
}
