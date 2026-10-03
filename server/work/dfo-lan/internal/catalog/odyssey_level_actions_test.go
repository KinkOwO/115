package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestOdysseyLevelActionsFollowChangedSource(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[level action]"}, {Type: 3, Text: "[action data]"}, {Type: 0, Value: 12}, {Type: 6, Text: "unlock support"}, {Type: 3, Text: "[/action data]"}, {Type: 3, Text: "[/level action]"}}
	r, e := ParseOdysseyLevelActions(cells)
	if e != nil || len(r[12]) != 1 || r[12][0] != "unlock support" {
		t.Fatal(r, e)
	}
	cells[2].Value = 13
	cells[3].Text = "unlock earring"
	r, e = ParseOdysseyLevelActions(cells)
	if e != nil || r[12] != nil || len(r[13]) != 1 || r[13][0] != "unlock earring" {
		t.Fatal("source change ignored", r, e)
	}
	cells[3].Type = 0
	if _, e = ParseOdysseyLevelActions(cells); e == nil {
		t.Fatal("malformed action accepted")
	}
}
