package adventure

import (
	"dfolan/internal/catalog/pvf"
	"errors"
	"testing"
)

func TestSeasonCostKeepsSignedGoldAndRejectsBranches(t *testing.T) {
	values := func(n ...float64) []pvf.CTPCell {
		out := []pvf.CTPCell{}
		for _, v := range n {
			out = append(out, pvf.CTPCell{Kind: "float", Float: v})
		}
		return out
	}
	table := &pvf.CTPTable{Records: []pvf.CTPRecord{
		{Index: 0, Name: "[cost]", Parent: -1, Refs: []pvf.CTPRef{{Name: "[idx]", Values: []uint64{1}}, {Name: "[required item]", Values: []uint64{2}}}},
		{Index: 1, Parent: 0, Name: "[idx]", Cells: values(16)},
		{Index: 2, Parent: 0, Name: "[required item]", Refs: []pvf.CTPRef{{Name: "[gold]", Values: []uint64{3}}, {Name: "[materials]", Values: []uint64{4}}}},
		{Index: 3, Parent: 2, Name: "[gold]", Cells: values(-1)},
		{Index: 4, Parent: 2, Name: "[materials]", Cells: values(10403609, 10)},
	}}
	cost, err := seasonCostByKey(table, 16)
	if err != nil || cost.Gold != 0 || len(cost.Materials) != 1 || cost.Materials[0].Count != 10 {
		t.Fatal(cost, err)
	}
	table.Records[2].Refs = append(table.Records[2].Refs, pvf.CTPRef{Name: "[optional]", Values: []uint64{4}})
	if _, err := seasonCostByKey(table, 16); err == nil {
		t.Fatal("optional cost accepted")
	}
	table.Records[2].Refs = table.Records[2].Refs[:2]
	table.Records[4].Cells = values(10403609, 1.5)
	if _, err := seasonCostByKey(table, 16); err == nil {
		t.Fatal("fractional cost accepted")
	}
}

func TestInstalledSeasonAvoidsEmbeddedReadAndMutation(t *testing.T) {
	old, err := EmbeddedSeasonRules()
	if err != nil {
		t.Fatal(err)
	}
	restore, err := InstallSeasonRules(old)
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := CurrentSeason()
	if err != nil {
		t.Fatal(err)
	}
	original := loadSeason
	loadSeason = func() (*SeasonRules, error) { return nil, errors.New("embedded source unavailable") }
	defer func() { loadSeason = original }()
	current.Levels[0].Upper++
	if old.Levels[0].Upper == current.Levels[0].Upper {
		t.Fatal("installed chart shares source slice")
	}
	current.OathCost.Materials[0].Count++
	if old.OathCost.Materials[0].Count == current.OathCost.Materials[0].Count {
		t.Fatal("installed costs share source slice")
	}
	bad := *old
	bad.Season = 0
	if _, err := InstallSeasonRules(&bad); err == nil {
		t.Fatal("invalid season installed")
	}
	after, err := CurrentSeason()
	if err != nil || after != current {
		t.Fatal("native runtime fell back or failed install damaged source", err)
	}
}
