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
