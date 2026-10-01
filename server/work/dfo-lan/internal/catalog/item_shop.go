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

	// 限购，来自物品自身 `.stk` 的 `[purchase limit] <scope> <period> <count>`：
	//
	//	LimitScope  = "character" | "account"
	//	LimitPeriod = "daily" | "weekly" | "monthly" | "accumulate" | "version"
	//
	// LimitCount == 0 表示**不限购**（多数商品）。
	//
	// "accumulate" 是**累计**：从首次购买起永久累计、不随周期重置（全库最多，1215 个）。
	// "version" 语义**未查证**（仅 11 个），服务端按累计处理 —— 见 loot 侧的注释。
	LimitScope  string `json:"limit_scope,omitempty"`
	LimitPeriod string `json:"limit_period,omitempty"`
	LimitCount  uint32 `json:"limit_count,omitempty"`
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
	return NewItemShops(s)
}

// NewItemShops owns the source offers and builds the historical first-payable
// lookup for both PVF imports and legacy audit artifacts.
func NewItemShops(s ItemShops) (*ItemShops, error) {
	shops := make(map[string]ItemShop, len(s.Shops))
	for key, shop := range s.Shops {
		shop.Offers = append([]ItemShopOffer(nil), shop.Offers...)
		for n := range shop.Offers {
			shop.Offers[n].Materials = append([]ItemShopMaterial(nil), shop.Offers[n].Materials...)
		}
		shops[key] = shop
	}
	s.Shops = shops
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

// ResolveShop 从一组候选 id 里挑出真正开商店的那个。
//
// 背景（2026-09-29 实机取证，42/42 样本）：CMD21 的 p[8] 与 p[12] 都是
// npc-like 的 id，但**哪个是商店取决于客户端当前从哪个 NPC 打开界面**：
//
//	p8=100000694 p12=100001774  -> 100001774 是商店（"装备之力魔法书"）
//	p8=100001019 p12=100003035  -> 100001019 是商店（奥德赛商店）
//
// 42 个样本里「两个都不在商店表」的情况为 0，所以「谁在表里谁是商店」是可靠判据。
// 返回 ok=false 表示候选都不是商店（调用方按无 source shop 处理）。
//
// 这里只做「查表选一」，不做任何猜测性推断。
func (s *ItemShops) ResolveShop(candidates ...uint32) (shopID uint32, ok bool) {
	if s == nil {
		return 0, false
	}
	for _, id := range candidates {
		if id == 0 {
			continue
		}
		if _, found := s.byShop[id]; found {
			return id, true
		}
	}
	return 0, false
}

// PurchaseLimit 返回某商品在该商店的限购规则。
//
// ok=false 表示不限购（LimitCount 为 0，或该商店没有这个模板）。
// 调用方应把 ok=false 当作「无限制」，不要当作错误。
func (s *ItemShops) PurchaseLimit(shopID, template uint32) (scope, period string, count uint32, ok bool) {
	if s == nil {
		return "", "", 0, false
	}
	offers, found := s.byShop[shopID]
	if !found {
		return "", "", 0, false
	}
	offer, exists := offers[template]
	if !exists || offer.LimitCount == 0 {
		return "", "", 0, false
	}
	return offer.LimitScope, offer.LimitPeriod, offer.LimitCount, true
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
