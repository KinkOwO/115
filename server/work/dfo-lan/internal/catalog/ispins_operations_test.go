package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestIspinsOperationMinutesAreSourceDefined(t *testing.T) {
	for _, minutes := range []int32{4, 10} {
		cells := []pvf.Token{section("[operation data set]"), section("[index]"), number(11), section("[type]"), number(6), section("[type fixed value]"), number(minutes), section("[/operation data set]")}
		rules, err := ParseIspinsOperations(cells)
		if err != nil || rules[11].FixedValue != uint32(minutes) {
			t.Fatal("lost source minutes", err)
		}
		if _, err := ParseIspinsOperations(append(cells, cells...)); err == nil {
			t.Fatal("duplicate operation accepted")
		}
	}
}

func TestIspinsOperationRangesAndSignedCoinFollowSource(t *testing.T) {
	text := func(value string) pvf.Token { return pvf.Token{Type: 6, Text: value} }
	for _, limit := range []int32{-1, 0, 2} {
		cells := []pvf.Token{section("[operation data set]"), section("[index]"), number(3), section("[type]"), number(3), section("[random range value]"), text("party"), text("normal"), number(70), number(80), number(5), section("[/random range value]"), section("[probability]"), number(500000), section("[allow coin]"), number(limit), section("[recommend fame]"), number(18000), section("[/operation data set]")}
		rules, err := ParseIspinsOperations(cells)
		if err != nil {
			t.Fatal(err)
		}
		r := rules[3]
		if len(r.RandomRanges) != 1 || r.RandomRanges[0].Maximum != 80 || r.RandomRanges[0].Step != 5 || r.AllowCoin == nil || *r.AllowCoin != limit || r.Probability != 500000 || r.RecommendFame != 18000 {
			t.Fatal("lost source operands", r)
		}
		cells[9].Value = 95
		rules, err = ParseIspinsOperations(cells)
		if err != nil || rules[3].RandomRanges[0].Maximum != 95 {
			t.Fatal("range remained a parallel constant", err)
		}
		cells[10].Value = 0
		if _, err = ParseIspinsOperations(cells); err == nil {
			t.Fatal("nonconstant range without step accepted")
		}
	}
}
