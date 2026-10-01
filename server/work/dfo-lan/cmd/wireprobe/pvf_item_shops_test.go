package main

import (
	"os"
	"testing"
)

func TestPVFItemShopsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete SHP/runtime routing audit")
	}
	c, err := preparePVFCoreCatalogs("item-shops", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{itemShopPath: "../../configs/itemshop-candidate.json", itemShopPolicyPath: "../../configs/pvf-item-shop-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.loadItemShops("missing-item-shops.json", os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil || len(s.Shops) != 527 {
		t.Fatal("native shop lookup unavailable", err)
	}
	if _, err := c.loadItemShops("missing", "foreign"); err == nil {
		t.Fatal("foreign save source accepted")
	}
	if s.Shops["100000375"].Path != "itemshop/100000375_global_6th_seria.shp" || s.Shops["100001019"].Npc != 100003035 {
		t.Fatal("compatibility route changed")
	}
	count := 0
	for _, shop := range s.Shops {
		count += len(shop.Offers)
	}
	t.Logf("%d shops / %d source offers and complete first-payable/amount/limit lookup match", len(s.Shops), count)
}
