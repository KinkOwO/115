package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"fmt"
	"log"
	"path/filepath"
	"reflect"
	"strconv"
)

func preparePVFItemShops(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["item-shops"] {
		return nil
	}
	p, err := catalog.ReadItemShopSourcePolicy(i.itemShopPolicyPath)
	if err != nil {
		return err
	}
	native, err := s.ItemShops(p)
	if err != nil {
		return err
	}
	direct := native.Catalog
	if i.checksBaselines() {
		name := i.itemShopPath
		if name == "" {
			name = filepath.Join(filepath.Dir(i.indexPath), "itemshop-candidate.json")
		}
		old, err := catalog.LoadItemShops(name)
		if err != nil {
			return err
		}
		if old.Source.Checksum != direct.Source.Checksum {
			return fmt.Errorf("item shop source identity changed")
		}
		if err = verifyPVFCatalog(old, direct); err != nil {
			return fmt.Errorf("item shops: %w", err)
		}
		for key, shop := range old.Shops {
			id64, _ := strconv.ParseUint(key, 10, 32)
			id := uint32(id64)
			for _, offer := range shop.Offers {
				a, al, ap := old.Materials(id, offer.Template)
				b, bl, bp := direct.Materials(id, offer.Template)
				if al != bl || ap != bp || !reflect.DeepEqual(a, b) || old.PurchaseAmount(id, offer.Template) != direct.PurchaseAmount(id, offer.Template) || old.Listed(id, offer.Template) != direct.Listed(id, offer.Template) {
					return fmt.Errorf("item shop %s effective payable lookup differs", key)
				}
				x1, x2, x3, x4 := old.PurchaseLimit(id, offer.Template)
				y1, y2, y3, y4 := direct.PurchaseLimit(id, offer.Template)
				if x1 != y1 || x2 != y2 || x3 != y3 || x4 != y4 {
					return fmt.Errorf("item shop %s limit behavior changed", key)
				}
			}
		}
	}
	c.itemShops = direct
	s.ReleaseReadCaches()
	count := 0
	for _, shop := range direct.Shops {
		count += len(shop.Offers)
	}
	log.Printf("PVF item shops prepared: shops=%d offers=%d native routes=%d compatibility list routes=%d explicit service routes=%d native list SHA=%s; original first-payable and unlimited policy retained", len(direct.Shops), count, native.NativeRoutes, native.AliasedRoutes, native.ExplicitRoutes, native.IndexHash)
	return nil
}
func (c pvfCoreCatalogs) loadItemShops(name, source string) (*catalog.ItemShops, error) {
	if c.itemShops != nil {
		if c.itemShops.Source.Checksum != source {
			return nil, fmt.Errorf("prepared item shop source differs from save catalog")
		}
		return c.itemShops, nil
	}
	return catalog.LoadItemShops(name)
}
