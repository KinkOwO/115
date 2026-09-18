package cashshop

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const shopSchema = "pvf-shop-types-v1"

type PilotConfig struct {
	Schema             string                 `json:"schema"`
	Source             pvf.ArchiveSnapshot    `json:"source"`
	PriceScriptHash    string                 `json:"price_script_hash"`
	IndexHash          string                 `json:"index_hash"`
	EquipmentIndexHash string                 `json:"equipment_index_hash,omitempty"`
	Entries            []OrdinaryProduct      `json:"entries"`
	Policies           map[string][]pvf.Token `json:"policies"`
}
type OrdinaryProduct struct {
	Section     string               `json:"section"`
	Row         []pvf.Token          `json:"row"`
	Item        catalog.ScriptRecord `json:"item"`
	IndexPath   string               `json:"index_path"`
	ImportError string               `json:"import_error,omitempty"`
}
type deliveryType struct {
	Kind  string
	Slots [2]uint16
	Limit uint32
}

// Inventory families dispatch by script type, never by SKU.
func ordinaryHandler(item catalog.ScriptRecord) (deliveryType, error) {
	h := deliveryType{Limit: 1000}
	openAll := os.Getenv("DFO_SHOP_OPEN_ALL") == "1"
	for i, t := range item.Cells {
		if t.Type != 3 {
			continue
		}
		switch t.Text {
		case "[expiration date]", "[period]", "[package data]", "[selection]", "[booster info]", "[creature]":
			if !openAll {
				return h, fmt.Errorf("special delivery field %s", t.Text)
			}
		case "[action type]":
			if i+1 < len(item.Cells) && item.Cells[i+1].Text == "[radiant treasure box]" && !openAll {
				return h, fmt.Errorf("special delivery action radiant treasure box")
			}
		case "[stackable type]":
			if h.Kind != "" || i+1 >= len(item.Cells) || item.Cells[i+1].Type != 6 {
				if !openAll {
					return h, fmt.Errorf("invalid stackable type")
				}
			}
			if i+1 < len(item.Cells) && item.Cells[i+1].Type == 6 {
				h.Kind = item.Cells[i+1].Text
			}
		case "[stack limit]":
			if i+1 >= len(item.Cells) || item.Cells[i+1].Type != 0 || item.Cells[i+1].Value <= 0 {
				if !openAll {
					return h, fmt.Errorf("invalid source stack limit")
				}
			}
			if i+1 < len(item.Cells) && item.Cells[i+1].Type == 0 && item.Cells[i+1].Value > 0 {
				h.Limit = uint32(item.Cells[i+1].Value)
			}
		}
	}
	if openAll && h.Limit == 0 {
		h.Limit = 1000
	}
	if openAll && h.Kind == "" {
		h.Kind = "[etc]"
	}
	switch h.Kind {
	case "[material]":
		h.Slots = [2]uint16{121, 176}
	case "[etc]", "[waste]", "[throw]", "[hp]", "[mp]", "[hp mp]", "[expert town potion]":
		h.Slots = [2]uint16{65, 120}
	default:
		if openAll {
			h.Slots = [2]uint16{65, 120}
			return h, nil
		}
		return h, fmt.Errorf("unimplemented delivery type %s", h.Kind)
	}
	return h, nil
}
func digestValid(s string) bool { b, e := hex.DecodeString(s); return e == nil && len(b) == 32 }
func (c PilotConfig) validate() error {
	if c.Schema != shopSchema || !digestValid(c.Source.Checksum) || !digestValid(c.PriceScriptHash) || !digestValid(c.IndexHash) || len(c.Entries) == 0 {
		return fmt.Errorf("invalid PVF shop catalog; reimport required")
	}
	seen := map[int32]bool{}
	for name, width := range map[string]int{"[purchasing limit]": 8, "[not stackable buy]": 1, "[immediately adaptive product]": 1, "[specific product mileage]": 2, "[auto open booster item]": 1} {
		cells, ok := c.Policies[name]
		if !ok || len(cells)%width != 0 {
			return fmt.Errorf("missing or malformed shop policy %s", name)
		}
		for _, t := range cells {
			if t.Type != 0 {
				return fmt.Errorf("invalid shop policy cell %s", name)
			}
		}
	}
	templates := map[int32]string{}
	for _, v := range c.Entries {
		if len(v.Row) != 14 || v.Row[0].Type != 0 || v.Row[0].Value <= 0 {
			return fmt.Errorf("malformed shop row")
		}
		id := v.Row[0].Value
		if seen[id] {
			return fmt.Errorf("duplicate shop product %d", id)
		}
		seen[id] = true
		if v.ImportError == "" {
			if old, ok := templates[v.Row[1].Value]; ok && old != v.Item.SHA256 {
				return fmt.Errorf("conflicting template definitions")
			}
			templates[v.Row[1].Value] = v.Item.SHA256
		}
	}
	return nil
}

type ItemInfo struct {
	ID            uint32
	Kind          string
	StackableType string
	StackLimit    uint32
}

type Pilot struct {
	Config      PilotConfig
	ItemCatalog map[uint32]ItemInfo
}

type PackageItem struct {
	Template uint32
	Count    uint32
}

// PackageItems extracts sub-items from [package data] if present in the item script.
func PackageItems(item catalog.ScriptRecord) ([]PackageItem, bool) {
	cells := item.Cells
	for i, c := range cells {
		if c.Type == 3 && c.Text == "[package data]" {
			var items []PackageItem
			j := i + 1
			for j < len(cells) && !(cells[j].Type == 3 && cells[j].Text == "[/package data]") {
				if cells[j].Type == 0 && cells[j].Value > 0 {
					tpl := uint32(cells[j].Value)
					cnt := uint32(1)
					if j+1 < len(cells) && cells[j+1].Type == 0 && cells[j+1].Value > 0 {
						cnt = uint32(cells[j+1].Value)
						j += 2
					} else {
						j++
					}
					items = append(items, PackageItem{Template: tpl, Count: cnt})
				} else {
					j++
				}
			}
			if len(items) > 0 {
				return items, true
			}
		}
	}
	return nil, false
}

func (p *Pilot) SetItemCatalog(items map[uint32]catalog.LootItem) {
	if p == nil {
		return
	}
	m := make(map[uint32]ItemInfo, len(items))
	for id, it := range items {
		m[id] = ItemInfo{
			ID:            it.ID,
			Kind:          it.Kind,
			StackableType: it.StackableType,
			StackLimit:    it.StackLimit,
		}
	}
	p.ItemCatalog = m
}

func (p *Pilot) findEntry(product, template uint32) (OrdinaryProduct, bool) {
	if p == nil {
		return OrdinaryProduct{}, false
	}
	for _, entry := range p.Config.Entries {
		if len(entry.Row) == 14 {
			if product != 0 && uint32(entry.Row[0].Value) == product {
				return entry, true
			}
			if template != 0 && uint32(entry.Row[1].Value) == template {
				return entry, true
			}
		}
	}
	return OrdinaryProduct{}, false
}

func (p *Pilot) resolveDeliveryType(template uint32) (deliveryType, error) {
	for _, entry := range p.Config.Entries {
		if len(entry.Row) == 14 && uint32(entry.Row[1].Value) == template {
			_, handler, e := p.Config.classify(entry)
			if e == nil {
				return handler, nil
			}
		}
	}
	if p != nil && p.ItemCatalog != nil {
		if info, ok := p.ItemCatalog[template]; ok {
			limit := info.StackLimit
			if limit == 0 {
				limit = 1000
			}
			kind := info.StackableType
			if kind == "" {
				kind = "[etc]"
			}
			slots := [2]uint16{65, 120}
			if strings.Contains(strings.ToLower(kind), "material") {
				slots = [2]uint16{121, 176}
			}
			return deliveryType{
				Kind:  kind,
				Slots: slots,
				Limit: limit,
			}, nil
		}
	}
	return deliveryType{
		Kind:  "[etc]",
		Slots: [2]uint16{65, 120},
		Limit: 1000,
	}, nil
}

func LoadPilot(path, source string) (*Pilot, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c PilotConfig
	if e = json.Unmarshal(b, &c); e != nil {
		return nil, e
	}
	if e = c.validate(); e != nil {
		return nil, e
	}
	if c.Source.Checksum != source {
		return nil, fmt.Errorf("shop source mismatch; rebuild dependent catalogs from the same PVF")
	}
	p := &Pilot{Config: c}
	products, e := p.products()
	if e != nil {
		return nil, e
	}
	if len(products) == 0 {
		return nil, fmt.Errorf("shop has no supported products")
	}
	return p, nil
}
func (p *Pilot) products() (map[uint32]Product, error) {
	if e := p.Config.validate(); e != nil {
		return nil, e
	}
	out := map[uint32]Product{}
	for _, v := range p.Config.Entries {
		product, _, e := p.Config.classify(v)
		if e == nil {
			out[product.ID] = product
		}
	}
	return out, nil
}
func (p *Pilot) EnabledCount() int { m, _ := p.products(); return len(m) }

type BagLedger interface {
	PurchaseCashToBag(context.Context, storage.CashOrder, func(json.RawMessage) (json.RawMessage, error)) (storage.CashReceipt, bool, error)
}

func (p *Pilot) Purchase(ctx context.Context, ledger BagLedger, account, character int64, key string, cart []protocol.CeraCartItem) (storage.CashReceipt, bool, error) {
	if p == nil || ledger == nil || len(cart) == 0 || len(cart) > 32 {
		return storage.CashReceipt{}, false, fmt.Errorf("purchase requires1..32 supported products")
	}
	for _, item := range cart {
		if item.Quantity == 0 || item.Quantity > 56 {
			return storage.CashReceipt{}, false, fmt.Errorf("purchase quantity must be1..56 per line")
		}
	}
	products, e := p.products()
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	for _, line := range cart {
		if _, ok := products[line.Product]; !ok {
			for _, entry := range p.Config.Entries {
				if uint32(entry.Row[0].Value) == line.Product {
					_, _, reason := p.Config.classify(entry)
					return storage.CashReceipt{}, false, fmt.Errorf("product%d: %v", line.Product, reason)
				}
			}
		}
	}
	s := Service{Catalog: Catalog{Source: p.Config.Source.Checksum, Products: products}}
	o, e := s.Quote(account, character, key, cart, time.Now())
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	var total uint64
	for _, line := range o.Lines {
		total += uint64(line.Units) * uint64(line.Quantity)
	}
	if total > 112000 {
		return storage.CashReceipt{}, false, fmt.Errorf("purchase exceeds delivery budget")
	}
	return ledger.PurchaseCashToBag(ctx, o, func(raw json.RawMessage) (json.RawMessage, error) {
		for _, line := range o.Lines {
			entry, ok := p.findEntry(line.Product, line.Template)
			if ok {
				if pkgItems, isPkg := PackageItems(entry.Item); isPkg {
					for _, sub := range pkgItems {
						deliverCnt := sub.Count * line.Units * line.Quantity
						raw, e = p.deliverAmount(raw, sub.Template, deliverCnt)
						if e != nil {
							return nil, e
						}
					}
					continue
				}
			}
			raw, e = p.deliverAmount(raw, line.Template, line.Units*line.Quantity)
			if e != nil {
				return nil, e
			}
		}
		return raw, nil
	})
}
func (p *Pilot) deliverAmount(raw json.RawMessage, template, amount uint32) (json.RawMessage, error) {
	if amount == 0 || amount > 112000 {
		return nil, fmt.Errorf("invalid delivery amount")
	}
	h, err := p.resolveDeliveryType(template)
	if err != nil {
		return nil, err
	}
	c := catalog.LootCatalog{Source: p.Config.Source, Items: map[uint32]catalog.LootItem{template: {ID: template, Kind: "stackable", StackableType: h.Kind, StackLimit: h.Limit}}}
	r := inventory.BagRules{Source: p.Config.Source.Checksum, Slots: map[string][2]uint16{h.Kind: h.Slots}, MissingStackLimit: 1000}
	b, e := inventory.ReadBag(raw)
	if e != nil {
		return nil, e
	}
	for _, row := range b.Items {
		if amount == 0 {
			break
		}
		if row.Template != template || row.Slot < h.Slots[0] || row.Slot > h.Slots[1] || row.Amount >= h.Limit {
			continue
		}
		n := min(amount, h.Limit-row.Amount)
		b, _, e = b.Add(c, r, template, n)
		if e != nil {
			return nil, e
		}
		amount -= n
	}
	for amount > 0 {
		n := min(amount, h.Limit)
		b, _, e = b.Add(c, r, template, n)
		if e != nil {
			return nil, e
		}
		amount -= n
	}
	if _, e = protocol.InventoryUpdate(b.Rows()); e != nil {
		return nil, e
	}
	return inventory.SaveBag(raw, b)
}
