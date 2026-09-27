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
	"math"
	"os"
	"strings"
	"sync"
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
	// Release 由目录文件中的 "release": true 显式启用。它只放宽既不涉及扣款、
	// 也不推断发货语义的策略门禁；未配置该字段时仍沿用严格的试运行行为。
	Release bool `json:"release,omitempty"`
	// immediateTemplates caches the templates any [immediately adaptive
	// product] SKU sells, so classify stays O(1) per entry. Built by
	// LoadPilot/ImportPilot; classify falls back to a per-call derivation
	// when the config was built by hand (tests).
	immediateTemplates map[int32]bool
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

// boxStackableType answers the item's [stackable type] value when present.
func boxStackableType(item catalog.ScriptRecord) (string, bool) {
	for i, t := range item.Cells {
		if t.Type == 3 && t.Text == "[stackable type]" {
			if i+1 < len(item.Cells) && item.Cells[i+1].Type == 6 {
				return item.Cells[i+1].Text, true
			}
			return "", false
		}
	}
	return "", false
}

func boxKind(kind string) bool {
	switch kind {
	case "[booster]", "[booster selection]", "[booster random]", "[cera booster]", "[usable cera package]":
		return true
	}
	return false
}

// boxContents reports whether the script carries the open-time reward table
// ([package data], [booster info] or [booster select category]). A box family
// item with a content table is delivered as an inert stackable; opening is
// the separate booster/package flow.
func boxContents(item catalog.ScriptRecord) bool {
	for _, t := range item.Cells {
		if t.Type != 3 {
			continue
		}
		switch t.Text {
		case "[package data]", "[booster info]", "[booster select category]":
			return true
		}
	}
	return false
}

// packageHandler classifies box delivery: the box itself lands in the
// consumable slots one per slot, its contents resolve at open time.
func packageHandler(item catalog.ScriptRecord) (deliveryType, error) {
	kind := ""
	limit := uint32(0)
	contents := false
	for i, t := range item.Cells {
		if t.Type != 3 {
			continue
		}
		switch t.Text {
		case "[stackable type]":
			if i+1 < len(item.Cells) && item.Cells[i+1].Type == 6 {
				kind = item.Cells[i+1].Text
			}
		case "[stack limit]":
			if i+1 < len(item.Cells) && item.Cells[i+1].Type == 0 && item.Cells[i+1].Value > 0 {
				limit = uint32(item.Cells[i+1].Value)
			}
		case "[package data]", "[booster info]", "[booster select category]":
			contents = true
		}
	}
	h := deliveryType{Kind: kind, Slots: [2]uint16{65, 120}, Limit: 1}
	switch kind {
	case "[usable cera package]":
		if !contents {
			return h, fmt.Errorf("package without source contents")
		}
	case "[booster]", "[cera booster]", "[booster selection]", "[booster random]":
		if !contents {
			return h, fmt.Errorf("booster box without source pool")
		}
	default:
		return h, fmt.Errorf("unimplemented box type %s", kind)
	}
	if limit > 0 {
		h.Limit = limit
	}
	return h, nil
}

// Inventory families dispatch by script type, never by SKU.
//
// release 仍使用有来源依据的数据，但不再拒绝特殊发货段，并接受与现有
// throw/etc 类型共用槽位区间的消耗品类型。槽位区间只决定物品放置位置，
// 不代表已经还原发货语义；没有对应区间的类型仍会被拒绝。
func ordinaryHandler(item catalog.ScriptRecord, release bool) (deliveryType, error) {
	for i, t := range item.Cells {
		if t.Type == 3 && t.Text == "[equipment type]" {
			if i+1 < len(item.Cells) && item.Cells[i+1].Text == "[creature]" {
				return deliveryType{
					Kind:  "[creature]",
					Slots: [2]uint16{0, 139},
					Limit: 1,
				}, nil
			}
		}
	}
	// A box family item with a source content table is purchasable even in an
	// ordinary section: the delivery is the closed box, opening is handled by
	// the booster/package flows.
	if kind, ok := boxStackableType(item); ok && boxKind(kind) && boxContents(item) {
		return packageHandler(item)
	}
	h := deliveryType{Limit: math.MaxInt32}
	openAll := os.Getenv("DFO_SHOP_OPEN_ALL") == "1"
	for i, t := range item.Cells {
		if t.Type != 3 {
			continue
		}
		switch t.Text {
		case "[expiration date]", "[period]", "[package data]", "[selection]", "[booster info]", "[creature]":
			if !release && !openAll {
				return h, fmt.Errorf("special delivery field %s", t.Text)
			}
		// [action type] 不再拦截:[radiant treasure box] 交付的就是未开启的
		// 盒子,右键走既有 CMD160 光辉宝箱流程(next44);其余 action type
		// 原本就随 [stackable type] 的普通处理放行。
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
		h.Limit = math.MaxInt32
	}
	if openAll && h.Kind == "" {
		h.Kind = "[etc]"
	}
	switch h.Kind {
	case "[material]":
		h.Slots = [2]uint16{121, 176}
	case "[etc]", "[waste]", "[throw]", "[hp]", "[mp]", "[hp mp]", "[expert town potion]":
		h.Slots = [2]uint16{65, 120}
	case "[material expert job]":
		if !release && !openAll {
			return h, fmt.Errorf("unimplemented delivery type %s", h.Kind)
		}
		h.Slots = [2]uint16{121, 176}
	case "[usable cera package]", "[cera booster]", "[booster]", "[booster selection]",
		"[grouped random box]", "[contract]", "[only effect]", "[feed]",
		"[enchant waste]", "[unlimited waste]":
		if !release && !openAll {
			return h, fmt.Errorf("unimplemented delivery type %s", h.Kind)
		}
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
		width := 14
		if v.Section == "[package]" {
			width = 13
		}
		if len(v.Row) != width || v.Row[0].Type != 0 || v.Row[0].Value <= 0 {
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

	cacheMu       sync.Mutex
	cacheOpenAll  string
	cacheProducts map[uint32]Product
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

const MaxExpireTime = math.MaxInt32

// HasExpiration checks if an item script defines an expiration or period constraint.
func HasExpiration(item catalog.ScriptRecord) bool {
	for _, c := range item.Cells {
		if c.Type == 3 {
			switch c.Text {
			case "[expiration date]", "[usable period]", "[period]", "[usable datetime]":
				return true
			}
		}
	}
	return false
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
		if width := len(entry.Row); width == 13 || width == 14 {
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
	if template == 1 {
		return deliveryType{
			Kind:  "[coin]",
			Slots: [2]uint16{1, 1},
			Limit: math.MaxUint32,
		}, nil
	}
	for _, entry := range p.Config.Entries {
		if width := len(entry.Row); (width == 13 || width == 14) && uint32(entry.Row[1].Value) == template {
			_, handler, e := p.Config.classify(entry)
			if e == nil {
				return handler, nil
			}
		}
	}
	if p != nil && p.ItemCatalog != nil {
		if info, ok := p.ItemCatalog[template]; ok {
			if info.Kind == "avatar" {
				return deliveryType{
					Kind:  "[avatar]",
					Slots: [2]uint16{0, 209},
					Limit: 1,
				}, nil
			}
			limit := info.StackLimit
			if limit == 0 {
				limit = math.MaxInt32
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

// 目录或启动参数任一显式启用发布模式时，使用已实现的发布商品规则。
// 未传参数的目录检查、试运行及既有调用继续遵循目录自身的设置。
func LoadPilot(path, source string, release ...bool) (*Pilot, error) {
	if len(release) > 1 {
		return nil, fmt.Errorf("商城发布模式参数重复")
	}
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c PilotConfig
	if e = json.Unmarshal(b, &c); e != nil {
		return nil, e
	}
	if len(release) == 1 && release[0] {
		c.Release = true
	}
	c.immediateTemplates = c.deriveImmediateTemplates()
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
	openAll := os.Getenv("DFO_SHOP_OPEN_ALL")
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	if p.cacheProducts != nil && p.cacheOpenAll == openAll {
		return p.cacheProducts, nil
	}
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
	p.cacheOpenAll = openAll
	p.cacheProducts = out
	return out, nil
}
func (p *Pilot) EnabledCount() int { m, _ := p.products(); return len(m) }

type BagLedger interface {
	PurchaseCashToBag(context.Context, storage.CashOrder, func(json.RawMessage) (json.RawMessage, error)) (storage.CashReceipt, bool, error)
}

// ContractCartLedger activates every named contract line and delivers the
// remaining lines of one order inside a single transaction, so a cart may mix
// contracts with ordinary merchandise (实机 2026-09-23:合并购买契约被旧的
// "separate order" 限制整单拒绝)。
type ContractCartLedger interface {
	PurchaseCashMixed(context.Context, storage.CashOrder, func(json.RawMessage) (json.RawMessage, error), map[int]storage.CashPremiumActivation) (storage.CashReceipt, bool, error)
}

// contractActivations partitions the cart: every line whose source row
// resolves to a premium contract (direct alias or single-child wrapper)
// becomes an activation keyed by its cart index, and joins a synthetic
// catalog product so the shared Quote can price the whole cart.
func (p *Pilot) contractActivations(cart []protocol.CeraCartItem, products map[uint32]Product) (map[int]storage.CashPremiumActivation, error) {
	activations := map[int]storage.CashPremiumActivation{}
	for i, line := range cart {
		entry, ok := p.findEntry(line.Product, 0)
		if !ok {
			continue
		}
		c, isPremium, err := entryContract(entry)
		if err != nil {
			return nil, err
		}
		if !isPremium {
			continue
		}
		row := entry.Row
		if len(row) != 14 || row[0].Value <= 0 || row[1].Value <= 0 || row[2].Value <= 0 || row[5].Value <= 0 {
			return nil, fmt.Errorf("invalid contract product")
		}
		products[line.Product] = Product{ID: uint32(row[0].Value), Template: uint32(row[1].Value), Units: uint32(row[2].Value), Cera: uint32(row[5].Value), Enabled: true}
		duration := c.DurationSecond * int64(line.Quantity) * int64(row[2].Value)
		if duration <= 0 {
			return nil, fmt.Errorf("premium duration overflow")
		}
		activations[i] = storage.CashPremiumActivation{Type: c.Type, DurationSecond: duration}
	}
	return activations, nil
}

func (p *Pilot) Purchase(ctx context.Context, ledger BagLedger, account, character int64, key string, cart []protocol.CeraCartItem) (storage.CashReceipt, bool, error) {
	if p == nil || ledger == nil || len(cart) == 0 || len(cart) > 32 {
		return storage.CashReceipt{}, false, fmt.Errorf("purchase requires1..32 supported products")
	}
	if receipt, applied, handled, err := p.TryPurchaseInventoryExpansion(ctx, ledger, account, character, key, cart); handled || err != nil {
		return receipt, applied, err
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
	products = func() map[uint32]Product {
		out := make(map[uint32]Product, len(products)+len(cart))
		for id, product := range products {
			out[id] = product
		}
		return out
	}()
	activations, e := p.contractActivations(cart, products)
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
	deliver := func(raw json.RawMessage) (json.RawMessage, error) {
		for i, line := range o.Lines {
			if _, isContract := activations[i]; isContract {
				continue
			}
			entry, ok := p.findEntry(line.Product, line.Template)
			if ok {
				if pkgItems, isPkg := PackageItems(entry.Item); isPkg {
					pkgHasExp := HasExpiration(entry.Item)
					for _, sub := range pkgItems {
						deliverCnt := sub.Count * line.Units * line.Quantity
						var subExp uint32
						if pkgHasExp {
							subExp = MaxExpireTime
						}
						raw, e = p.deliverAmount(raw, sub.Template, deliverCnt, subExp)
						if e != nil {
							return nil, e
						}
					}
					continue
				}
			}
			var exp uint32
			if ok && HasExpiration(entry.Item) {
				exp = MaxExpireTime
			}
			raw, e = p.deliverAmount(raw, line.Template, line.Units*line.Quantity, exp)
			if e != nil {
				return nil, e
			}
		}
		return raw, nil
	}
	if len(activations) == 0 {
		return ledger.PurchaseCashToBag(ctx, o, deliver)
	}
	mixed, ok := ledger.(ContractCartLedger)
	if !ok {
		return storage.CashReceipt{}, false, fmt.Errorf("contract cart ledger missing")
	}
	return mixed.PurchaseCashMixed(ctx, o, deliver, activations)
}
func (p *Pilot) deliverAmount(raw json.RawMessage, template, amount uint32, expireTime ...uint32) (json.RawMessage, error) {
	if amount == 0 || amount > 112000 {
		return nil, fmt.Errorf("invalid delivery amount")
	}
	var exp uint32
	if len(expireTime) > 0 {
		exp = expireTime[0]
	}
	if template == 1 {
		b, e := inventory.ReadBag(raw)
		if e != nil {
			return nil, e
		}
		if uint64(b.Coin)+uint64(amount) > math.MaxUint32 {
			return nil, fmt.Errorf("coin overflow")
		}
		b.Coin += amount
		return inventory.SaveBag(raw, b)
	}
	h, err := p.resolveDeliveryType(template)
	if err != nil {
		return nil, err
	}
	if h.Kind == "[avatar]" {
		if info, ok := p.ItemCatalog[template]; ok && info.Kind != "avatar" {
			return nil, fmt.Errorf("shop template %d does not resolve to an avatar", template)
		}
		b, e := inventory.ReadBag(raw)
		if e != nil {
			return nil, e
		}
		occupied := map[uint16]bool{}
		if b.Special != nil {
			for _, row := range b.Special[1] {
				occupied[row.Slot] = true
			}
		}
		for i := uint32(0); i < amount; i++ {
			found := false
			for s := uint16(0); s < 210; s++ {
				if !occupied[s] {
					occupied[s] = true
					if b.Special == nil {
						b.Special = map[byte][]inventory.BagEquipment{}
					}
					b.Special[1] = append(b.Special[1], inventory.BagEquipment{
						Slot:     s,
						Template: template,
						Period:   exp,
					})
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("avatar wardrobe full")
			}
		}
		return inventory.SaveBag(raw, b)
	}
	if h.Kind == "[creature]" {
		b, e := inventory.ReadBag(raw)
		if e != nil {
			return nil, e
		}
		occupied := map[uint16]bool{}
		if b.Special != nil {
			for _, item := range b.Special[7] {
				occupied[item.Slot] = true
			}
		}
		for i := uint32(0); i < amount; i++ {
			found := false
			for s := uint16(0); s < 140; s++ {
				if !occupied[s] {
					occupied[s] = true
					if b.Special == nil {
						b.Special = map[byte][]inventory.BagEquipment{}
					}
					b.Special[7] = append(b.Special[7], inventory.BagEquipment{
						Slot:     s,
						Template: template,
					})
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("creature inventory full")
			}
		}
		return inventory.SaveBag(raw, b)
	}
	c := catalog.LootCatalog{Source: p.Config.Source, Items: map[uint32]catalog.LootItem{template: {ID: template, Kind: "stackable", StackableType: h.Kind, StackLimit: h.Limit}}}
	r := inventory.BagRules{Source: p.Config.Source.Checksum, Slots: map[string][2]uint16{h.Kind: h.Slots}, MissingStackLimit: 1000}
	b, e := inventory.ReadBag(raw)
	if e != nil {
		return nil, e
	}
	stackRows := b.Items
	if inventory.IsPetConsumable(h.Kind) {
		stackRows = b.PetItems
	}
	for _, row := range stackRows {
		if amount == 0 {
			break
		}
		if row.Template != template || row.Slot < h.Slots[0] || row.Slot > h.Slots[1] || row.Amount >= h.Limit {
			continue
		}
		n := min(amount, h.Limit-row.Amount)
		b, _, e = b.Add(c, r, template, n, exp)
		if e != nil {
			return nil, e
		}
		amount -= n
	}
	for amount > 0 {
		n := min(amount, h.Limit)
		b, _, e = b.Add(c, r, template, n, exp)
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

// DeliverySpaces reports which special equipment spaces a purchase receipt
// touches, so the packet builder can refresh the avatar wardrobe (space 1)
// and the creature tab (space 7) alongside the ordinary bag. Package lines
// resolve through their [package data] children because only those reach a
// bag.
func (p *Pilot) DeliverySpaces(receipt storage.CashReceipt) (avatar, creature bool) {
	if p == nil {
		return false, false
	}
	for _, d := range receipt.Deliveries {
		children, templates := []PackageItem(nil), []uint32(nil)
		if entry, ok := p.findEntry(d.Product, 0); ok {
			if pkgItems, isPkg := PackageItems(entry.Item); isPkg {
				children = pkgItems
			}
		}
		if children != nil {
			for _, sub := range children {
				templates = append(templates, sub.Template)
			}
		} else {
			templates = append(templates, d.Template)
		}
		for _, template := range templates {
			a, c := p.templateSpace(template)
			avatar = avatar || a
			creature = creature || c
		}
	}
	return avatar, creature
}

func (p *Pilot) templateSpace(template uint32) (avatar, creature bool) {
	if template == 0 || template == 1 {
		return false, false
	}
	h, err := p.resolveDeliveryType(template)
	if err != nil {
		return false, false
	}
	switch h.Kind {
	case "[avatar]":
		return true, false
	case "[creature]":
		return false, true
	}
	if inventory.IsPetConsumable(h.Kind) {
		return false, true
	}
	return false, false
}
