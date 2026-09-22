package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// ItemShopModel identifies the exported item-shop artifact.
const ItemShopModel = "source-item-shops-v1"

// ItemShopMaterial is one [need material] entry: pay Count of Template.
type ItemShopMaterial struct {
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
}

// ItemShopOffer prices one good. Materials empty means the ordinary gold price
// applies (the gateway's historical behaviour).
type ItemShopOffer struct {
	Tab            uint32             `json:"tab"`
	Index          uint32             `json:"index"`
	Template       uint32             `json:"template"`
	PurchaseAmount uint32             `json:"purchase_amount,omitempty"`
	Materials      []ItemShopMaterial `json:"materials,omitempty"`
}

// ItemShop is one itemshop/*.shp table, keyed by the id the client sends as
// NpcID in CMD21 (the file name before the underscore, e.g. 100001019).
type ItemShop struct {
	Npc    uint32          `json:"npc,omitempty"`
	Type   string          `json:"type,omitempty"`
	Path   string          `json:"path"`
	SHA256 string          `json:"sha256"`
	Offers []ItemShopOffer `json:"offers"`
}

// ItemShops is the exported catalog. The source prices goods two ways: with
// [need material] (the Odyssey shop pays silver/gold coins) or, when the block
// is absent, in gold.
type ItemShops struct {
	Model  string              `json:"model"`
	Source pvf.ArchiveSnapshot `json:"source"`
	Shops  map[string]ItemShop `json:"shops"`

	byShop map[uint32]map[uint32]ItemShopOffer
}

// LoadItemShops reads and validates an exported item-shop catalog.
func LoadItemShops(path string) (*ItemShops, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s ItemShops
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, err
	}
	if s.Model != ItemShopModel {
		return nil, fmt.Errorf("unexpected item shop model %q", s.Model)
	}
	if len(s.Source.Checksum) != 64 {
		return nil, fmt.Errorf("invalid item shop source")
	}
	if len(s.Shops) == 0 {
		return nil, fmt.Errorf("empty item shop catalog")
	}
	s.byShop = make(map[uint32]map[uint32]ItemShopOffer, len(s.Shops))
	for key, shop := range s.Shops {
		id, err := strconv.ParseUint(key, 10, 32)
		if err != nil || id == 0 {
			return nil, fmt.Errorf("item shop key %q is not a shop id", key)
		}
		if shop.Path == "" || len(shop.SHA256) != 64 {
			return nil, fmt.Errorf("item shop %s lacks a source reference", key)
		}
		if _, err := hex.DecodeString(shop.SHA256); err != nil {
			return nil, fmt.Errorf("item shop %s has an invalid script hash", key)
		}
		offers := make(map[uint32]ItemShopOffer, len(shop.Offers))
		for _, o := range shop.Offers {
			if o.Template == 0 {
				return nil, fmt.Errorf("item shop %s has an offer without a template", key)
			}
			for _, m := range o.Materials {
				if m.Template == 0 || m.Count == 0 {
					return nil, fmt.Errorf("item shop %s offer %d has an invalid material", key, o.Template)
				}
			}
			// 同一模板出现多次（不同 tab 或不同支付方式）时保留第一个可支付的报价，
			// 否则保留第一个，避免"最后写入者胜"把可支付的那条覆盖掉。
			if prev, ok := offers[o.Template]; ok {
				if len(prev.Materials) > 0 || len(o.Materials) == 0 {
					continue
				}
			}
			offers[o.Template] = o
		}
		s.byShop[uint32(id)] = offers
	}
	return &s, nil
}

// Materials reports how the shop prices a template, when the source says the
// good is paid for with [need material]. ok is false when this shop does not
// list the template at all.
func (s *ItemShops) Materials(shopID, template uint32) (materials []ItemShopMaterial, listed bool, ok bool) {
	if s == nil {
		return nil, false, false
	}
	offers, found := s.byShop[shopID]
	if !found {
		return nil, false, false
	}
	offer, exists := offers[template]
	if !exists {
		return nil, false, false
	}
	if len(offer.Materials) == 0 {
		return nil, true, false // listed, but priced in gold
	}
	return offer.Materials, true, true
}

// PurchaseAmount is how many units one purchase grants (source default 1).
func (s *ItemShops) PurchaseAmount(shopID, template uint32) uint32 {
	if s == nil {
		return 1
	}
	offer, ok := s.byShop[shopID][template]
	if !ok || offer.PurchaseAmount == 0 {
		return 1
	}
	return offer.PurchaseAmount
}

// Listed reports whether the shop lists a template at all (regardless of how it
// is priced).
func (s *ItemShops) Listed(shopID, template uint32) bool {
	if s == nil {
		return false
	}
	_, ok := s.byShop[shopID][template]
	return ok
}
