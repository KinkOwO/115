package cashshop

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"testing"
)

func TestSpecialCatalogImportsWithoutOrdinaryDelivery(t *testing.T) {
	base := syntheticCatalog(t, "[etc]")
	families := []string{"[item mod or ext]", "[item period or contract]", "[creature]", "[package related]"}
	shop := catalog.ScriptRecord{SHA256: base.PriceScriptHash}
	for i, family := range families {
		row := append([]pvf.Token(nil), base.Entries[0].Row...)
		row[0].Value += int32(i)
		shop.Cells = append(shop.Cells, pvf.Token{Type: 3, Text: family})
		shop.Cells = append(shop.Cells, row...)
	}
	shop.Cells = append(shop.Cells, pvf.Token{Type: 3, Text: "[cargo 2]"}, pvf.Token{Value: 1}, pvf.Token{Value: 3999917})
	index := catalog.ScriptRecord{SHA256: base.IndexHash, Cells: []pvf.Token{{Value: 999817}, {Type: 6, Text: "test/new.stk"}}}
	c, e := importShopScripts(base.Source, shop, index, func(string) (catalog.ScriptRecord, error) { return base.Entries[0].Item, nil })
	if e != nil {
		t.Fatal(e)
	}
	if len(c.Entries) != 4 || len(c.Policies["[cargo 2]"]) != 2 {
		t.Fatal("special source rows lost")
	}
	for i, r := range c.Report() {
		if r.Enabled || r.Reason == "" || r.Section != families[i] {
			t.Fatalf("special family delivered as ordinary: %+v", r)
		}
	}
	products, e := (&Pilot{Config: c}).products()
	if e != nil || len(products) != 0 {
		t.Fatal("special products entered ordinary purchase map", products, e)
	}
	c, e = importShopScripts(base.Source, shop, index, func(string) (catalog.ScriptRecord, error) {
		return catalog.ScriptRecord{}, fmt.Errorf("missing fixture script")
	})
	if e != nil {
		t.Fatal(e)
	}
	if c.Report()[0].ImportError != "missing fixture script" {
		t.Fatal("lost separate import diagnostic")
	}
	shop.Cells = shop.Cells[:15-1]
	if _, e = importShopScripts(base.Source, shop, index, func(string) (catalog.ScriptRecord, error) { return base.Entries[0].Item, nil }); e == nil {
		t.Fatal("truncated special price row accepted")
	}
}
