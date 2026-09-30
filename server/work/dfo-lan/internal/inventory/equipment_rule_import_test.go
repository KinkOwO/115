package inventory

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestAccountVaultSourceRejectsIncompleteAndNegativeFee(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[required level]"}, {Type: 0, Value: 60}, {Type: 3, Text: "[upgrade info]"}}
	for _, v := range []int32{8, 100000000, 3262, 100, 0, -1} {
		cells = append(cells, pvf.Token{Type: 0, Value: v})
	}
	if r, err := ParseAccountVaultSource(cells); err != nil || r.Upgrades[0][5] != -1 {
		t.Fatal(r, err)
	}
	if _, err := ParseAccountVaultSource(cells[:len(cells)-1]); err == nil {
		t.Fatal("truncated six-column row accepted")
	}
	cells[7].Value = -1
	if _, err := ParseAccountVaultSource(cells); err == nil {
		t.Fatal("negative gold cost accepted")
	}
}

func TestRandomOptionImportRejectsAbsentArchive(t *testing.T) {
	if _, err := ImportRandomOptionData(nil); err == nil {
		t.Fatal("missing source accepted")
	}
	if _, err := optionInts(map[string][][]pvf.Token{"x": {{{Type: 6, Text: "unknown"}}}}, "x"); err == nil {
		t.Fatal("unknown ratio cell accepted")
	}
}
