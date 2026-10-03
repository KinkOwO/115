package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"math"
)

type BuyReceipt struct {
	NpcID    uint32 `json:"npc_id"`
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
	Slot     uint16 `json:"slot"`
	Cost     uint32 `json:"cost"`
	NewGold  uint32 `json:"new_gold"`
	Source   string `json:"source"`
	Seq      uint64 `json:"seq"`
}

// SoldRowReceipt records one row of a completed sale.
type SoldRowReceipt struct {
	List      byte   `json:"list"`
	Slot      uint16 `json:"slot"`
	Template  uint32 `json:"template"`
	Count     uint32 `json:"count"`
	UnitPrice uint32 `json:"unit_price"`
}

type SellReceipt struct {
	NpcID      uint32           `json:"npc_id"`
	Rows       []SoldRowReceipt `json:"rows"`
	GoldGained uint32           `json:"gold_gained"`
	NewGold    uint32           `json:"new_gold"`
	Source     string           `json:"source"`
	Seq        uint64           `json:"seq"`
}

// ShopService owns NPC shop price, payment, and bag rules.
type ShopService struct {
	Catalog       catalog.LootCatalog
	EventModel    string
	BagRules      BagRules
	ItemShops     *catalog.ItemShops
	Prices        *catalog.ShopPrices
	ItemMaterials *catalog.ItemMaterials
}

func (s *ShopService) shopPrice(template uint32) (catalog.ShopPrice, error) {
	if s.Prices != nil && s.Prices.Source == s.Catalog.Source.Checksum {
		if price, ok := s.Prices.Items[template]; ok {
			return price, nil
		}
	}
	return catalog.ShopPrice{}, fmt.Errorf("missing current-source shop price for item %d", template)
}

// ShopBuyPlan resolves the shop identity and payment before settlement.
type ShopBuyPlan struct {
	ShopID        uint32
	Materials     []MaterialCost
	Cost          uint32
	StackableType string
}

// QuoteBuy gives shop materials priority over item materials, then source gold price.
func (s *ShopService) QuoteBuy(r protocol.BuyItemRequest) (ShopBuyPlan, error) {
	if s.ItemMaterials == nil {
		return ShopBuyPlan{}, fmt.Errorf("shop purchases require the native PVF materials catalog")
	}
	shopID := r.NpcID
	if id, ok := s.ItemShops.ResolveShop(r.NpcID, r.ActorID); ok {
		shopID = id
	}
	shopMats, _, shopPaid := s.ItemShops.Materials(shopID, r.Template)
	itemMats, itemPaid := s.ItemMaterials.Materials(r.Template)
	var mats []MaterialCost
	switch {
	case shopPaid:
		for _, m := range shopMats {
			mats = append(mats, MaterialCost{Template: m.Template, Count: m.Count})
		}
	case itemPaid:
		for _, m := range itemMats {
			mats = append(mats, MaterialCost{Template: m.Template, Count: m.Count})
		}
	}
	var cost uint32
	if len(mats) == 0 {
		p, err := s.shopPrice(r.Template)
		if err != nil {
			return ShopBuyPlan{}, err
		}
		buy := p.Buy
		if buy == nil {
			fallback := p.Sell * 5
			buy = &fallback
		}
		if r.Count == 0 || uint64(*buy)*uint64(r.Count) > math.MaxUint32 {
			return ShopBuyPlan{}, fmt.Errorf("missing or overflowing source purchase price")
		}
		cost = *buy * r.Count
	}

	var stackableType string
	if item, ok := s.Catalog.Items[r.Template]; ok {
		stackableType = item.StackableType
	}

	return ShopBuyPlan{shopID, mats, cost, stackableType}, nil
}

func (s *ShopService) ApplyBuyMaterials(current Role, rawCounts json.RawMessage, r protocol.BuyItemRequest, plan ShopBuyPlan, seq uint64) (json.RawMessage, json.RawMessage, BuyReceipt, error) {
	mats, cost, stackableType := plan.Materials, plan.Cost, plan.StackableType

	b, e := ReadBag(current.State)
	if e != nil {
		return nil, nil, BuyReceipt{}, e
	}
	m, e := ReadAccountMaterials(rawCounts)
	if e != nil {
		return nil, nil, BuyReceipt{}, e
	}
	b, m, slot, e := b.BuyWithMaterialsWithStore(s.BagRules, m, r.Template, r.Count, mats, stackableType)
	if e != nil {
		return nil, nil, BuyReceipt{}, e
	}
	state, e := SaveBag(current.State, b)
	if e != nil {
		return nil, nil, BuyReceipt{}, e
	}
	updated, e := m.Save()
	if e != nil {
		return nil, nil, BuyReceipt{}, e
	}
	out := BuyReceipt{
		NpcID: r.NpcID, Template: r.Template, Count: r.Count, Slot: slot,
		Cost: cost, NewGold: b.Gold, Source: s.Catalog.Source.SaveIdentity(), Seq: seq,
	}
	return state, updated, out, nil
}

func (s *ShopService) ApplyBuyGold(current Role, r protocol.BuyItemRequest, plan ShopBuyPlan, seq uint64) (json.RawMessage, BuyReceipt, error) {
	cost, stackableType := plan.Cost, plan.StackableType

	b, e := ReadBag(current.State)
	if e != nil {
		return nil, BuyReceipt{}, e
	}
	b, slot, e := b.Buy(s.BagRules, r.Template, r.Count, cost, stackableType)
	if e != nil {
		return nil, BuyReceipt{}, e
	}
	updated, e := SaveBag(current.State, b)
	if e != nil {
		return nil, BuyReceipt{}, e
	}
	out := BuyReceipt{
		NpcID: r.NpcID, Template: r.Template, Count: r.Count, Slot: slot,
		Cost: cost, NewGold: b.Gold, Source: s.Catalog.Source.SaveIdentity(), Seq: seq,
	}
	return updated, out, nil
}

func (s *ShopService) ApplySell(current Role, r protocol.SellItemRequest, seq uint64) (json.RawMessage, SellReceipt, error) {
	b, e := ReadBag(current.State)
	if e != nil {
		return nil, SellReceipt{}, e
	}
	rows := make([]SoldRowReceipt, 0, len(r.Rows))
	var gained uint32
	for _, row := range r.Rows {
		// Resolve the identity from the transaction's current owned bag, never
		// from request metadata or a potentially stale session snapshot.
		_, template, _, e := b.Sell(s.BagRules, row.List, row.Slot, row.Count, 0)
		if e != nil {
			return nil, SellReceipt{}, e
		}
		price, e := s.shopPrice(template)
		if e != nil {
			return nil, SellReceipt{}, e
		}
		next, template, goldGained, e := b.Sell(s.BagRules, row.List, row.Slot, row.Count, price.Sell)
		if e != nil {
			return nil, SellReceipt{}, e
		}
		b = next
		gained += goldGained
		rows = append(rows, SoldRowReceipt{
			List:      row.List,
			Slot:      row.Slot,
			Template:  template,
			Count:     row.Count,
			UnitPrice: price.Sell,
		})
	}
	updated, e := SaveBag(current.State, b)
	if e != nil {
		return nil, SellReceipt{}, e
	}
	out := SellReceipt{
		NpcID:      r.NpcID,
		Rows:       rows,
		GoldGained: gained,
		NewGold:    b.Gold,
		Source:     s.Catalog.Source.SaveIdentity(),
		Seq:        seq,
	}
	return updated, out, nil
}

func (s *ShopService) ValidateBuy(role Role, r protocol.BuyItemRequest) error {

	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return fmt.Errorf("buy source mismatch")
	}
	if s.Catalog.HasRuntimeDetails() && s.Catalog.Items[r.Template].Kind == "stackable" {
		if _, err := s.Catalog.ItemScript(r.Template); err != nil {
			return err
		}
	}
	return nil
}

func (s *ShopService) ValidateSell(role Role, r protocol.SellItemRequest) error {

	if role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return fmt.Errorf("sell source mismatch")
	}
	if len(r.Rows) == 0 {
		return fmt.Errorf("sell requires at least one row")
	}
	return nil
}
