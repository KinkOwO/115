package cashshop

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"strings"
	"testing"
)

// syntheticFamily builds a one-product shop catalog for the two source
// families the ordinary sections never carried: single avatar pieces
// ([dont trade avatar], equipment.lst templates) and closed packages
// ([package], stackable.lst templates with a 13-cell price row).
func syntheticFamily(t *testing.T, section string, row []pvf.Token, equipment map[uint32]string, scriptFor func(name string) (catalog.ScriptRecord, error)) PilotConfig {
	t.Helper()
	hash := strings.Repeat("a", 64)
	shop := catalog.ScriptRecord{SHA256: hash, Cells: append([]pvf.Token{{Type: 3, Text: section}}, row...)}
	// The stackable index never lists avatar pieces; packages list theirs.
	var index catalog.ScriptRecord
	if section == "[package]" {
		stackableCells := []pvf.Token{{Value: row[1].Value}, pvf.Token{Type: 6, Text: "stackable/cash/box.stk"}}
		index = catalog.ScriptRecord{SHA256: hash, Cells: stackableCells}
	} else {
		index = catalog.ScriptRecord{SHA256: hash}
	}
	source := pvf.ArchiveSnapshot{Checksum: hash}
	resolve := func(name string) (catalog.ScriptRecord, error) {
		if scriptFor != nil {
			return scriptFor(name)
		}
		return catalog.ScriptRecord{SHA256: hash, Path: name}, nil
	}
	c, e := importShopScripts(source, shop, index, resolve)
	if e != nil {
		t.Fatal(e)
	}
	eqIndex := catalog.ScriptRecord{SHA256: strings.Repeat("b", 64)}
	for id, p := range equipment {
		eqIndex.Cells = append(eqIndex.Cells, pvf.Token{Value: int32(id)}, pvf.Token{Type: 6, Text: p})
	}
	if e = c.resolveEquipmentEntries(eqIndex, resolve); e != nil {
		t.Fatal(e)
	}
	return c
}

func TestAvatarPiecePurchase(t *testing.T) {
	row := make([]pvf.Token, 14)
	row[0].Value = 3107733
	row[1].Value = 515540507
	row[2].Value = 4
	row[5].Value = 120
	for _, i := range []int{10, 11, 12} {
		row[i].Value = -1
	}
	row[13] = pvf.Token{Type: 6}
	c := syntheticFamily(t, "[dont trade avatar]", row,
		map[uint32]string{515540507: "equipment/character/priest/at_avatar/shoes/515540507.equ"}, nil)
	p := &Pilot{Config: c}
	products, e := p.products()
	if e != nil || products[3107733].Units != 1 || products[3107733].Cera != 120 || products[3107733].Template != 515540507 {
		t.Fatal("avatar product not enabled", e, products[3107733])
	}
	if products[3107733].Kind != 4 {
		t.Fatal("avatar purchase kind not projected from source cell2", products[3107733])
	}
	// 实机 2026-09-23:客户端时装购买条目 Kind=4(option 0)。
	ledger := &packLedger{state: json.RawMessage(`{}`)}
	_, applied, e := p.Purchase(context.Background(), ledger, 1, 1, "avatar-piece-0001", []protocol.CeraCartItem{{Option: 0, Kind: 4, Product: 3107733, Quantity: 2}})
	if e != nil || !applied {
		t.Fatal("avatar purchase", e)
	}
	if ledger.order.Lines[0].UnitPrice != 120 || ledger.order.Lines[0].Units != 1 {
		t.Fatal("avatar quote", ledger.order.Lines[0])
	}
	bag, e := inventory.ReadBag(ledger.state)
	if e != nil || len(bag.Items) != 0 {
		t.Fatal("ordinary bag touched by avatar delivery", e, bag)
	}
	if len(bag.Special[1]) != 2 {
		t.Fatal("expected two wardrobe rows", bag.Special[1])
	}
	for i, av := range bag.Special[1] {
		if av.Template != 515540507 || av.Slot != uint16(i) {
			t.Fatal("avatar row", av)
		}
	}
	// Any other cart kind than the projected source value must be refused.
	l := &packLedger{state: json.RawMessage(`{}`)}
	if _, _, e = p.Purchase(context.Background(), l, 1, 1, "avatar-kind-0001", []protocol.CeraCartItem{{Option: 0, Kind: 0, Product: 3107733, Quantity: 1}}); e == nil || string(l.state) != "{}" {
		t.Fatal("avatar cart kind mismatch accepted", e)
	}
	if _, _, e = p.Config.classify(c.Entries[0]); e != nil {
		t.Fatal("avatar classify regressed", e)
	}
}

// The live 整套 cart mixes avatar pieces (kind 4) with closed boxes (kind 0);
// every line must quote and deliver inside one atomic order.
func TestMixedAvatarAndBoxCart(t *testing.T) {
	avatarRow := make([]pvf.Token, 14)
	avatarRow[0].Value = 3107733
	avatarRow[1].Value = 515540507
	avatarRow[2].Value = 4
	avatarRow[5].Value = 150
	for _, i := range []int{10, 11, 12} {
		avatarRow[i].Value = -1
	}
	avatarRow[13] = pvf.Token{Type: 6}
	boxRow := make([]pvf.Token, 13)
	boxRow[0].Value = 3400245
	boxRow[1].Value = 590713421
	boxRow[4].Value = 599
	boxRow[7] = pvf.Token{Type: 6, Text: "Hot Summer 2015 Avatar Box (Look)"}
	boxRow[8].Value = 90
	boxRow[10].Value = -1
	boxRow[11].Value = -1
	boxRow[12] = pvf.Token{Type: 6}
	script := catalog.ScriptRecord{
		SHA256: strings.Repeat("a", 64),
		Cells: []pvf.Token{
			{Type: 3, Text: "[stackable type]"}, {Type: 6, Text: "[booster]"},
			{Type: 3, Text: "[booster info]"}, {Value: 100}, {Value: 1}, {Value: 50}, {Value: 2},
		},
	}
	row := append([]pvf.Token(nil), avatarRow...)
	avatar := syntheticFamily(t, "[dont trade avatar]", row,
		map[uint32]string{515540507: "equipment/character/priest/at_avatar/shoes/515540507.equ"}, nil)
	box := syntheticFamily(t, "[package]", boxRow, nil, func(name string) (catalog.ScriptRecord, error) {
		s := script
		s.Path = name
		return s, nil
	})
	box.Entries = append(box.Entries, avatar.Entries...)
	p := &Pilot{Config: box}
	ledger := &packLedger{state: json.RawMessage(`{}`)}
	cart := []protocol.CeraCartItem{
		{Option: 0, Kind: 4, Product: 3107733, Quantity: 1},
		{Option: 0, Kind: 0, Product: 3400245, Quantity: 1},
	}
	_, applied, e := p.Purchase(context.Background(), ledger, 1, 1, "mixed-avatar-box-0001", cart)
	if e != nil || !applied || len(ledger.order.Lines) != 2 {
		t.Fatal("mixed cart", e)
	}
	bag, e := inventory.ReadBag(ledger.state)
	if e != nil {
		t.Fatal(e)
	}
	if len(bag.Special[1]) != 1 || bag.Special[1][0].Template != 515540507 {
		t.Fatal("avatar line delivery", bag.Special[1])
	}
	if len(bag.Items) != 1 || bag.Items[0].Template != 590713421 || bag.Items[0].Amount != 1 {
		t.Fatal("box line delivery", bag.Items)
	}
}

func TestAvatarPieceRejections(t *testing.T) {
	base := make([]pvf.Token, 14)
	base[0].Value = 3107733
	base[1].Value = 515540507
	base[2].Value = 4
	base[5].Value = 120
	for _, i := range []int{10, 11, 12} {
		base[i].Value = -1
	}
	base[13] = pvf.Token{Type: 6}
	mutations := map[string]func([]pvf.Token){
		"price":    func(r []pvf.Token) { r[5].Value = 0 },
		"class":    func(r []pvf.Token) { r[2].Value = 5 },
		"currency": func(r []pvf.Token) { r[3].Value = 7 },
		"marker":   func(r []pvf.Token) { r[10].Value = 0 },
		"trailer":  func(r []pvf.Token) { r[13] = pvf.Token{Type: 6, Text: "condition"} },
		"limit":    func(r []pvf.Token) { r[2].Value = 0 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			row := append([]pvf.Token(nil), base...)
			mutate(row)
			c := syntheticFamily(t, "[dont trade avatar]", row,
				map[uint32]string{515540507: "equipment/character/priest/at_avatar/shoes/515540507.equ"}, nil)
			if _, _, e := c.classify(c.Entries[0]); e == nil {
				t.Fatal("bad avatar row admitted")
			}
		})
	}
	t.Run("missing template", func(t *testing.T) {
		c := syntheticFamily(t, "[dont trade avatar]", base, nil, nil)
		if _, _, e := c.classify(c.Entries[0]); e == nil {
			t.Fatal("unresolved avatar admitted")
		}
	})
	t.Run("non-avatar path", func(t *testing.T) {
		c := syntheticFamily(t, "[dont trade avatar]", base,
			map[uint32]string{515540507: "equipment/character/common/jacket/cloth/515540507.equ"}, nil)
		if _, _, e := c.classify(c.Entries[0]); e == nil {
			t.Fatal("non-avatar equipment admitted")
		}
	})
}

func TestPackagePurchaseExpandsChildren(t *testing.T) {
	row := make([]pvf.Token, 13)
	row[0].Value = 3400245
	row[1].Value = 590713421
	row[4].Value = 599
	row[7] = pvf.Token{Type: 6, Text: "Hot Summer 2015 Avatar Box (Look)"}
	row[8].Value = 90
	row[10].Value = -1
	row[11].Value = -1
	row[12] = pvf.Token{Type: 6}
	script := catalog.ScriptRecord{
		SHA256: strings.Repeat("a", 64),
		Cells: []pvf.Token{
			{Type: 3, Text: "[stackable type]"}, {Type: 6, Text: "[usable cera package]"},
			{Type: 3, Text: "[package data]"},
			{Value: 10417792}, {Value: 1},
			{Value: 10418036}, {Value: 100},
			{Type: 3, Text: "[/package data]"},
		},
	}
	c := syntheticFamily(t, "[package]", row, nil, func(name string) (catalog.ScriptRecord, error) {
		s := script
		s.Path = name
		return s, nil
	})
	p := &Pilot{Config: c}
	products, e := p.products()
	if e != nil || products[3400245].Units != 1 || products[3400245].Cera != 599 {
		t.Fatal("package product not enabled", e, products[3400245])
	}
	ledger := &packLedger{state: json.RawMessage(`{}`)}
	_, applied, e := p.Purchase(context.Background(), ledger, 1, 1, "package-open-0001", []protocol.CeraCartItem{{Product: 3400245, Quantity: 1}})
	if e != nil || !applied {
		t.Fatal("package purchase", e)
	}
	bag, e := inventory.ReadBag(ledger.state)
	if e != nil {
		t.Fatal(e)
	}
	amounts := map[uint32]uint32{}
	for _, it := range bag.Items {
		amounts[it.Template] += it.Amount
		if it.Template == 590713421 {
			t.Fatal("placeholder box delivered instead of children")
		}
	}
	if amounts[10417792] != 1 || amounts[10418036] != 100 || len(bag.Items) != 2 {
		t.Fatal("package children", bag.Items)
	}
}

func TestPackageRejections(t *testing.T) {
	base := make([]pvf.Token, 13)
	base[0].Value = 3400245
	base[1].Value = 590713421
	base[4].Value = 599
	base[7] = pvf.Token{Type: 6, Text: "Hot Summer 2015 Avatar Box (Look)"}
	base[8].Value = 90
	base[10].Value = -1
	base[11].Value = -1
	base[12] = pvf.Token{Type: 6}
	withContents := func(name string) (catalog.ScriptRecord, error) {
		return catalog.ScriptRecord{
			SHA256: strings.Repeat("a", 64),
			Path:   name,
			Cells: []pvf.Token{
				{Type: 3, Text: "[stackable type]"}, {Type: 6, Text: "[usable cera package]"},
				{Type: 3, Text: "[package data]"}, {Value: 10417792}, {Value: 1},
			},
		}, nil
	}
	mutations := map[string]func([]pvf.Token){
		"price":     func(r []pvf.Token) { r[4].Value = 0 },
		"alt price": func(r []pvf.Token) { r[2].Value = 12000000 },
		"currency":  func(r []pvf.Token) { r[3].Value = 1 },
		"tag":       func(r []pvf.Token) { r[9].Value = 70086 },
		"sale":      func(r []pvf.Token) { r[10].Value = 0 },
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			row := append([]pvf.Token(nil), base...)
			mutate(row)
			c := syntheticFamily(t, "[package]", row, nil, withContents)
			if _, _, e := c.classify(c.Entries[0]); e == nil {
				t.Fatal("bad package row admitted")
			}
		})
	}
	t.Run("empty package", func(t *testing.T) {
		c := syntheticFamily(t, "[package]", base, nil, func(name string) (catalog.ScriptRecord, error) {
			return catalog.ScriptRecord{SHA256: strings.Repeat("a", 64), Path: name, Cells: []pvf.Token{{Type: 3, Text: "[stackable type]"}, {Type: 6, Text: "[usable cera package]"}}}, nil
		})
		if _, _, e := c.classify(c.Entries[0]); e == nil {
			t.Fatal("package without contents admitted")
		}
	})
}

// The regenerated release catalog must admit the audited source families and
// deliver them end to end.
func TestAvatarAndPackageCurrentCatalog(t *testing.T) {
	p, e := LoadPilot("../../configs/shop-vault-release.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	products, e := p.products()
	if e != nil {
		t.Fatal(e)
	}
	piece := products[3107733]
	if piece.Template != 515540507 || piece.Units != 1 || piece.Cera != 120 || piece.Kind != 4 {
		t.Fatal("source avatar product missing", piece)
	}
	l := &packLedger{state: json.RawMessage(`{}`)}
	if _, _, e = p.Purchase(context.Background(), l, 1, 1, "avatar-catalog-0001", []protocol.CeraCartItem{{Option: 0, Kind: 4, Product: 3107733, Quantity: 1}}); e != nil {
		t.Fatal(e)
	}
	b, e := inventory.ReadBag(l.state)
	if e != nil || len(b.Special[1]) != 1 || b.Special[1][0].Template != 515540507 {
		t.Fatal("avatar delivery", e, b.Special[1])
	}
	pkg := products[3400245]
	if pkg.Units != 1 || pkg.Cera != 599 {
		t.Fatal("source package product missing", pkg)
	}
	l = &packLedger{state: json.RawMessage(`{}`)}
	if _, _, e = p.Purchase(context.Background(), l, 1, 1, "package-catalog-0001", []protocol.CeraCartItem{{Product: 3400245, Quantity: 1}}); e != nil {
		t.Fatal(e)
	}
	b, e = inventory.ReadBag(l.state)
	if e != nil {
		t.Fatal(e)
	}
	if len(b.Items) == 0 {
		t.Fatal("package delivery empty")
	}
	for _, it := range b.Items {
		if it.Template == 590713421 {
			t.Fatal("package placeholder delivered")
		}
	}
	if avatar, creature := p.DeliverySpaces(storage.CashReceipt{Deliveries: []storage.CashDelivery{{Product: 3107733, Template: 515540507}}}); !avatar || creature {
		t.Fatal("avatar space detection")
	}
}
