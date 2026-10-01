package cashshop

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func syntheticCatalog(t *testing.T, kind string) PilotConfig {
	t.Helper()
	hash := strings.Repeat("a", 64)
	row := make([]pvf.Token, 14)
	row[0].Value = 3999917
	row[1].Value = 999817
	row[2].Value = 7
	row[5].Value = 123
	row[8] = pvf.Token{Type: 6, Text: "New PVF item"}
	row[11].Value = -1
	row[12] = pvf.Token{Type: 6}
	row[13].Value = -1
	shop := catalog.ScriptRecord{SHA256: hash, Cells: append([]pvf.Token{{Type: 3, Text: "[item etc]"}}, row...)}
	index := catalog.ScriptRecord{SHA256: hash, Cells: []pvf.Token{{Value: 999817}, {Type: 6, Text: "test/new.stk"}}}
	source := pvf.ArchiveSnapshot{Checksum: hash}
	c, e := importShopScripts(source, shop, index, func(name string) (catalog.ScriptRecord, error) {
		if name != "stackable/test/new.stk" {
			return catalog.ScriptRecord{}, fmt.Errorf("wrong list resolution")
		}
		return catalog.ScriptRecord{SHA256: hash, Path: name, Cells: []pvf.Token{{Type: 3, Text: "[stackable type]"}, {Type: 6, Text: kind}, {Type: 3, Text: "[stack limit]"}, {Value: 10}}}, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return c
}

func TestShopPilotPVFNewIDsNeedNoCode(t *testing.T) {
	for _, kind := range []string{"[material]", "[etc]", "[waste]", "[throw]", "[hp]", "[mp]", "[hp mp]", "[expert town potion]"} {
		t.Run(kind, func(t *testing.T) {
			c := syntheticCatalog(t, kind)
			p := &Pilot{Config: c}
			ledger := &packLedger{state: json.RawMessage(`{"level":55}`)}
			_, applied, e := p.Purchase(context.Background(), ledger, 1, 1, "new-source-product-0001", []protocol.CeraCartItem{{Product: 3999917, Quantity: 3}})
			if e != nil || !applied || ledger.order.Lines[0].UnitPrice != 123 {
				t.Fatal("new ID purchase", e)
			}
			bag, e := inventory.ReadBag(ledger.state)
			if e != nil || len(bag.Items) != 3 {
				t.Fatal("stack splits", e, bag)
			}
			start := uint16(65)
			if kind == "[material]" {
				start = 121
			}
			for i, amount := range []uint32{10, 10, 1} {
				if bag.Items[i].Slot != start+uint16(i) || bag.Items[i].Amount != amount || bag.Items[i].Template != 999817 {
					t.Fatal("new template delivery", bag)
				}
			}
			if _, e = protocol.CeraPurchaseOrdinarySuccess(3999917, 3); e != nil {
				t.Fatal("ACK still whitelisted", e)
			}
		})
	}
}

func TestShopPilotPVFRejectPolicies(t *testing.T) {
	mutations := map[string]func(*PilotConfig){
		"price":    func(c *PilotConfig) { c.Entries[0].Row[5].Value = -1 },
		"currency": func(c *PilotConfig) { c.Entries[0].Row[3].Value = 5 },
		"date":     func(c *PilotConfig) { c.Entries[0].Row[12].Text = "future" },
		"hidden":   func(c *PilotConfig) { c.Entries[0].Row[9].Value = 90 },
		"path":     func(c *PilotConfig) { c.Entries[0].Item.Path = "stackable/wrong.stk" },
		"limit": func(c *PilotConfig) {
			c.Policies["[purchasing limit]"] = make([]pvf.Token, 8)
			c.Policies["[purchasing limit]"][0].Value = 3999917
		},
		"package": func(c *PilotConfig) {
			c.Entries[0].Item.Cells = append(c.Entries[0].Item.Cells, pvf.Token{Type: 3, Text: "[package data]"})
		},
		"expiration": func(c *PilotConfig) {
			c.Entries[0].Item.Cells = append(c.Entries[0].Item.Cells, pvf.Token{Type: 3, Text: "[expiration date]"})
		},
		"stack":          func(c *PilotConfig) { c.Entries[0].Item.Cells[3].Value = 0 },
		"missing script": func(c *PilotConfig) { c.Entries[0].ImportError = "read failed" },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			c := syntheticCatalog(t, "[material]")
			mutate(&c)
			p := &Pilot{Config: c}
			ledger := &packLedger{state: json.RawMessage(`{}`)}
			if _, _, e := p.Purchase(context.Background(), ledger, 1, 1, "invalid-source-0001", []protocol.CeraCartItem{{Product: 3999917, Quantity: 1}}); e == nil {
				t.Fatal("invalid policy bought")
			}
			if string(ledger.state) != "{}" || ledger.order.Key != "" {
				t.Fatal("invalid policy reached ledger")
			}
			if r := c.Report(); len(r) != 1 || r[0].Enabled || r[0].Reason == "" {
				t.Fatal("missing rejection report")
			}
		})
	}
	for _, kind := range []string{"[usable cera package]", "[creature]", "[booster]", "[new unknown type]"} {
		c := syntheticCatalog(t, kind)
		if _, _, e := c.classify(c.Entries[0]); e == nil {
			t.Fatal("special type projected into ordinary bag", kind)
		}
	}
	c := syntheticCatalog(t, "[etc]")
	other := c.Entries[0]
	other.Row = append([]pvf.Token(nil), other.Row...)
	other.Row[0].Value++
	c.Entries = append(c.Entries, other)
	c.Policies["[immediately adaptive product]"] = []pvf.Token{{Value: 3999917}}
	if _, _, e := c.classify(c.Entries[1]); e == nil {
		t.Fatal("immediate multipack bypass")
	}
}

func TestShopPilotPVFCurrentCatalog(t *testing.T) {
	p, e := LoadPilot("../../configs/shop-purchase-pilot.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	products, e := p.products()
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []uint32{3000666, 3000667, 3000668, 3000116, 3000117, 3000113, 3000114, 3000115, 3000126, 3000127, 3000109, 3000110, 3000111, 3000112} {
		if _, ok := products[id]; !ok {
			t.Fatal("ordinary consumable not enabled", id)
		}
	}
	// A closed booster box with a source pool is now delivered as an inert
	// stackable (opening is the booster flow) instead of being refused; the
	// same covers [action type] [radiant treasure box] items (next44 flow).
	if box, ok := products[3400268]; !ok || box.Units != 1 || box.Template != 590713836 {
		t.Fatal("title box not purchasable as a closed box", box)
	}
	if _, ok := products[3400235]; !ok {
		t.Fatal("radiant treasure box not purchasable")
	}
	for _, id := range []uint32{} {
		if _, ok := products[id]; ok {
			t.Fatal("special item incorrectly enabled", id)
		}
	}
	// Every admitted row can be delivered and acknowledged, not just chosen SKUs.
	for id, product := range products {
		l := &packLedger{state: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","gold":4294967295}}`)}
		if _, _, e = p.Purchase(context.Background(), l, 1, 1, fmt.Sprintf("catalog-all-%d-0001", id), []protocol.CeraCartItem{{Product: id, Quantity: 1}}); e != nil {
			t.Fatal(id, e)
		}
		b, e := inventory.ReadBag(l.state)
		if e != nil {
			t.Fatal(id, e)
		}
		var children []PackageItem
		if entry, ok := p.findEntry(id, product.Template); ok {
			children, _ = PackageItems(entry.Item)
		}
		// A [package data] child may be template 1 (the coin wallet), which
		// never shows up as an ordinary bag row.
		countsCoin := product.Template == 1
		for _, sub := range children {
			if sub.Template == 1 {
				countsCoin = true
			}
		}
		var total uint64
		if countsCoin {
			total += uint64(b.Coin)
		}
		for _, row := range b.Items {
			matched := row.Template == product.Template
			for _, sub := range children {
				if sub.Template == row.Template {
					matched = true
					break
				}
			}
			if !matched {
				t.Fatal("wrong template", id)
			}
			total += uint64(row.Amount)
		}
		if total == 0 {
			t.Fatal("empty delivery", id)
		}
		if _, e = protocol.CeraPurchaseOrdinarySuccess(id, 1); e != nil {
			t.Fatal(id, e)
		}
	}
	t.Logf("PVF catalog: %d ordinary SKUs verified end-to-end", len(products))
}

func TestShopPilotOpenAll(t *testing.T) {
	t.Setenv("DFO_SHOP_OPEN_ALL", "1")
	p, e := LoadPilot("../../configs/shop-vault-release.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	products, e := p.products()
	if e != nil {
		t.Fatal(e)
	}
	if len(products) != 17154 {
		t.Fatalf("expected 17154 enabled products under DFO_SHOP_OPEN_ALL=1, got %d", len(products))
	}
}
