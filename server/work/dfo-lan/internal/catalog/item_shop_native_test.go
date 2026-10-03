package catalog

import (
	"dfolan/internal/catalog/pvf"
	"strings"
	"testing"
)

func nativeShopFixture() ScriptRecord {
	return ScriptRecord{Path: "itemshop/source.shp", SHA256: strings.Repeat("a", 64), Cells: []pvf.Token{
		{Type: 3, Text: "[NPC]"}, {Value: 50}, {Type: 3, Text: "[type]"}, {Type: 6, Text: "[etc shop]"}, {Type: 3, Text: "[tab]"}, {Type: 3, Text: "[sell item list]"},
		{Type: 3, Text: "[item]"}, {Value: 0}, {Type: 3, Text: "[index]"}, {Value: 900}, {Type: 3, Text: "[/item]"},
		{Type: 3, Text: "[item]"}, {Value: 1}, {Type: 3, Text: "[index]"}, {Value: 900}, {Type: 3, Text: "[purchase amount]"}, {Value: 7}, {Type: 3, Text: "[need material]"}, {Value: 800}, {Value: 10}, {Type: 3, Text: "[/need material]"}, {Type: 3, Text: "[/item]"},
		{Type: 3, Text: "[item]"}, {Value: 2}, {Type: 3, Text: "[index]"}, {Value: 900}, {Type: 3, Text: "[need material]"}, {Value: 801}, {Value: 20}, {Type: 3, Text: "[/need material]"}, {Type: 3, Text: "[/item]"}, {Type: 3, Text: "[/sell item list]"},
	}}
}
func TestNativeItemShopKeepsSourceRowsAndFirstPayableOffer(t *testing.T) {
	shop, err := ParseNativeItemShop(nativeShopFixture())
	if err != nil || shop.Npc != 50 || len(shop.Offers) != 3 || shop.Offers[0].PurchaseAmount != 1 || shop.Offers[1].Materials[0].Count != 10 {
		t.Fatal(shop, err)
	}
	raw := ItemShops{Model: ItemShopModel, Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}, Shops: map[string]ItemShop{"40": shop}}
	s, err := NewItemShops(raw)
	if err != nil {
		t.Fatal(err)
	}
	materials, listed, paid := s.Materials(40, 900)
	if !listed || !paid || materials[0].Template != 800 || s.PurchaseAmount(40, 900) != 7 {
		t.Fatal("first payable policy changed")
	}
	raw.Shops["40"].Offers[1].Materials[0].Count = 999
	raw.Shops["40"].Offers[1].PurchaseAmount = 999
	delete(raw.Shops, "40")
	materials, _, _ = s.Materials(40, 900)
	if materials[0].Count != 10 || s.PurchaseAmount(40, 900) != 7 {
		t.Fatal("external source aliases lookup")
	}
}
func TestNativeItemShopRefusesPartialPaymentsAndUnclosedItems(t *testing.T) {
	for _, tag := range []string{"[/item]", "[/need material]", "[/sell item list]"} {
		t.Run(tag, func(t *testing.T) {
			s := nativeShopFixture()
			for n, c := range s.Cells {
				if c.Text == tag {
					s.Cells = append(s.Cells[:n:n], s.Cells[n+1:]...)
					break
				}
			}
			if _, err := ParseNativeItemShop(s); err == nil {
				t.Fatal("malformed source accepted")
			}
		})
	}
}

func TestNativeItemShopRetainsAdmissionForSourceTabWithoutListOpener(t *testing.T) {
	script := nativeShopFixture()
	script.Cells = append(script.Cells, pvf.Token{Type: 3, Text: "[tab]"}, pvf.Token{Type: 3, Text: "[item]"}, pvf.Token{Value: 31}, pvf.Token{Type: 3, Text: "[index]"}, pvf.Token{Value: 901}, pvf.Token{Type: 3, Text: "[/item]"}, pvf.Token{Type: 3, Text: "[/sell item list]"})
	shop, err := ParseNativeItemShop(script)
	if err != nil || len(shop.Offers) != 3 {
		t.Fatal("previously excluded source tab became enabled", err)
	}
}
