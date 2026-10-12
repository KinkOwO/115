package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestNativeLegionOperationsRetainSharedModesFloatAndCoins(t *testing.T) {
	s := func(v string) pvf.Token { return pvf.Token{Type: 3, Text: v} }
	txt := func(v string) pvf.Token { return pvf.Token{Type: 6, Text: v} }
	n := func(v int32) pvf.Token { return pvf.Token{Type: 0, Value: v} }
	cells := []pvf.Token{s("[operation table]")}
	for _, mode := range []string{"normal", "expert"} {
		cells = append(cells, s("[operation]"), s("[type]"), txt(mode), s("[index]"), n(15), s("[/operation]"))
	}
	cells = append(cells, s("[/operation table]"), s("[operation data set]"), s("[index]"), n(15), s("[type]"), n(4), s("[member limit]"), txt("party"), s("[select pool]"), n(3), s("[probability]"), n(100), s("[random range value]"), txt("party"), txt("normal"), pvf.Token{Type: 2, Number: 1.5}, pvf.Token{Type: 2, Number: 2.5}, n(0), s("[/random range value]"), s("[allow coin]"), n(-1), n(2), s("[/allow coin]"), s("[operation formula]"), txt("c:oper_4_armor_data1"), pvf.Token{Type: 2, Number: 1.5}, s("[/operation formula]"), s("[/operation data set]"))
	r, err := ParseNativeLegionOperations(ScriptRecord{Path: "fixture.etc", Cells: cells})
	if err != nil {
		t.Fatal(err)
	}
	d := r.Definitions[15]
	if r.Modes["normal"][0] != 15 || r.Modes["expert"][0] != 15 || d.Ranges[0].Minimum != 1.5 || d.Ranges[0].Maximum != 2.5 || d.Ranges[0].Step != 0 || d.AllowCoins[0] != -1 || d.AllowCoins[1] != 2 || d.Fields["[operation formula]"][1].Number != 1.5 {
		t.Fatal("source operands lost", r, d)
	}
	cells[11].Value = 16
	if _, err := ParseNativeLegionOperations(ScriptRecord{Cells: cells}); err == nil {
		t.Fatal("dangling mode index accepted")
	}
}
