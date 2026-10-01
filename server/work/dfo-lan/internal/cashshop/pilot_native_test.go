package cashshop

import (
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func TestNewPilotOwnsImportedCellsAndProductCache(t *testing.T) {
	c := syntheticCatalog(t, "[material]")
	c.Policies["[cargo 1]"] = []pvf.Token{{Value: 9}}
	p, err := NewPilot(c, c.Source.Checksum, true)
	if err != nil {
		t.Fatal(err)
	}
	want, err := p.ProductSnapshot()
	if err != nil || len(want) != 1 {
		t.Fatal(want, err)
	}
	c.Entries[0].Row[5].Value = 9999
	c.Entries[0].Item.Cells[1].Text = "unsupported"
	c.Policies["[immediately adaptive product]"] = append(c.Policies["[immediately adaptive product]"], c.Entries[0].Row[0])
	c.Policies["[cargo 1]"][0].Value = 999
	got, err := p.ProductSnapshot()
	if err != nil || got[3999917] != want[3999917] || p.Config.Entries[0].Item.Cells[1].Text != "[material]" || len(p.Config.Policies["[immediately adaptive product]"]) != 0 || p.Config.Policies["[cargo 1]"][0].Value != 9 {
		t.Fatal("imported slices aliased", err)
	}
	delete(got, 3999917)
	if p.EnabledCount() != 1 {
		t.Fatal("product snapshot exposed cache")
	}
}

func TestNewPilotRefusesForeignSourceAndInvalidRows(t *testing.T) {
	c := syntheticCatalog(t, "[material]")
	if _, err := NewPilot(c, strings.Repeat("b", 64)); err == nil {
		t.Fatal("foreign source accepted")
	}
	if _, err := NewPilot(c, c.Source.Checksum, true, false); err == nil {
		t.Fatal("duplicate release options accepted")
	}
	c.Entries[0].Row = c.Entries[0].Row[:13]
	if _, err := NewPilot(c, c.Source.Checksum); err == nil {
		t.Fatal("invalid source row accepted")
	}
}
