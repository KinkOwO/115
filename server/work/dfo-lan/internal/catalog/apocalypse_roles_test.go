package catalog

import (
	"dfolan/internal/catalog/pvf"
	"math"
	"testing"
)

func roleCatalog(rows ...[]float64) *ApocalypseCatalog {
	c := &ApocalypseCatalog{Records: []pvf.CTPRecord{{Index: 0, Parent: -1, Name: "[role per member limit]"}}}
	ref := pvf.CTPRef{Name: "[member]"}
	for _, row := range rows {
		r := pvf.CTPRecord{Index: len(c.Records), Parent: 0, Name: "[member]"}
		for _, v := range row {
			r.Cells = append(r.Cells, pvf.CTPCell{Kind: "float", Float: v})
		}
		ref.Values = append(ref.Values, uint64(r.Index))
		c.Records = append(c.Records, r)
	}
	c.Records[0].Refs = []pvf.CTPRef{ref}
	return c
}

func TestApocalypseRoleLimitsFollowSourceChanges(t *testing.T) {
	c := roleCatalog([]float64{4, 2, 2, 1, 2}, []float64{1, 1, 1})
	rules, err := c.CompileRoleLimits()
	if err != nil {
		t.Fatal(err)
	}
	if q, ok := rules.Quota(4, 1); !ok || q != 2 {
		t.Fatal("final cumulative map lost the scout quota", q, ok)
	}
	if _, ok := rules.Quota(1, 2); ok {
		t.Fatal("invented a solo guardian quota")
	}
	if _, ok := rules.Quota(2, 1); ok {
		t.Fatal("invented a missing member row")
	}
	c.Records[1].Cells[2].Float = 1
	c.Records[1].Cells[4].Float = 3
	changed, err := c.CompileRoleLimits()
	if err != nil {
		t.Fatal(err)
	}
	if q, _ := changed.Quota(4, 1); q != 3 {
		t.Fatal("source quota mutation did not reach compiled rule", q)
	}
	if q, _ := rules.Quota(4, 1); q != 2 {
		t.Fatal("compiled rules alias mutable source records")
	}
}

func TestApocalypseRoleLimitsRejectInvalidSource(t *testing.T) {
	for _, row := range [][]float64{
		{4, 1}, {4, 1, 1, 2, 1, 1, 1}, {4, 1, 3, 2, 2},
		{4, 1, 2, 1, 2}, {5, 1, 1}, {4, 0, 1}, {4, 3, 1},
		{4, 1, 0}, {4, 1, 1.5}, {4, 1, math.NaN()}, {4, 1, math.Inf(1)},
	} {
		if _, err := roleCatalog(row).CompileRoleLimits(); err == nil {
			t.Fatal("accepted invalid source row", row)
		}
	}
	if _, err := roleCatalog([]float64{1, 1, 1}, []float64{1, 1, 1}).CompileRoleLimits(); err == nil {
		t.Fatal("accepted duplicate member count")
	}
	for _, mutate := range []func(*ApocalypseCatalog){
		func(c *ApocalypseCatalog) { c.Records[0].Refs = nil },
		func(c *ApocalypseCatalog) { c.Records[0].Refs[0].Values = []uint64{100} },
		func(c *ApocalypseCatalog) { c.Records[0].Refs[0].Values = []uint64{1, 1} },
		func(c *ApocalypseCatalog) { c.Records[1].Parent = -1 },
		func(c *ApocalypseCatalog) { c.Records[1].Cells[2].Kind = "name" },
	} {
		c := roleCatalog([]float64{1, 1, 1})
		mutate(c)
		if _, err := c.CompileRoleLimits(); err == nil {
			t.Fatal("accepted inconsistent reference or non-numeric cell")
		}
	}
}

func TestCommittedApocalypseRoleSource(t *testing.T) {
	c, err := LoadApocalypseCatalog("../../configs/apocalypse.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := c.CompileRoleLimits()
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 4 {
		t.Fatal("source member rows lost", rules)
	}
	if q, ok := rules.Quota(3, 2); !ok || q != 2 {
		t.Fatal("native two-pair row parsed incorrectly", q, ok)
	}
}
