package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
)

// ServerShopID is the existing service's routing key. It is deliberately
// separate from a source list ID, including the old unlisted source routes.
type ItemShopRoute struct {
	ServerShopID uint32 `json:"server_shop_id"`
	NativeShopID uint32 `json:"native_shop_id,omitempty"`
	ScriptPath   string `json:"script_path,omitempty"`
}
type ItemShopSourcePolicy struct {
	Version           int             `json:"version"`
	PurchaseLimitMode string          `json:"purchase_limit_mode"`
	Routes            []ItemShopRoute `json:"routes"`
}
type NativeItemShops struct {
	Catalog                                     *ItemShops
	IndexHash                                   string
	NativeRoutes, AliasedRoutes, ExplicitRoutes int
}

func ReadItemShopSourcePolicy(name string) (ItemShopSourcePolicy, error) {
	var p ItemShopSourcePolicy
	f, e := os.Open(name)
	if e != nil {
		return p, e
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if e = d.Decode(&p); e != nil {
		return p, e
	}
	if e = d.Decode(new(any)); e != io.EOF {
		return p, fmt.Errorf("item shop policy has trailing data")
	}
	return p, nil
}
func ImportItemShops(a *pvf.Archive, p ItemShopSourcePolicy) (NativeItemShops, error) {
	var out NativeItemShops
	if a == nil || p.Version != 1 || p.PurchaseLimitMode != "disabled" || len(p.Routes) == 0 {
		return out, fmt.Errorf("invalid item shop source/policy")
	}
	index, e := ReadScript(a, "list/itemshop.lst")
	if e != nil {
		return out, e
	}
	entries, e := ParseIndex(index.Cells)
	if e != nil {
		return out, e
	}
	bindings := map[uint32]string{}
	for _, r := range entries {
		bindings[r.ID] = strings.ToLower(r.Path)
	}
	out.IndexHash = index.SHA256
	c := ItemShops{Model: ItemShopModel, Source: a.Snapshot(), Shops: map[string]ItemShop{}}
	for _, route := range p.Routes {
		key := strconv.FormatUint(uint64(route.ServerShopID), 10)
		if route.ServerShopID == 0 {
			return out, fmt.Errorf("zero server shop route")
		}
		if _, ok := c.Shops[key]; ok {
			return out, fmt.Errorf("duplicate server shop route %d", route.ServerShopID)
		}
		var name string
		if route.NativeShopID != 0 {
			if route.ScriptPath != "" {
				return out, fmt.Errorf("shop %d has two binding sources", route.ServerShopID)
			}
			var ok bool
			name, ok = bindings[route.NativeShopID]
			if !ok {
				return out, fmt.Errorf("shop %d lacks native list binding %d", route.ServerShopID, route.NativeShopID)
			}
			if route.NativeShopID == route.ServerShopID {
				out.NativeRoutes++
			} else {
				out.AliasedRoutes++
			}
		} else {
			name = route.ScriptPath
			out.ExplicitRoutes++
		}
		if path.Clean(name) != name || name != strings.ToLower(name) || strings.Contains(name, "\\") || strings.HasPrefix(name, "../") || path.Ext(name) != ".shp" || !strings.Contains(name, "itemshop/") {
			return out, fmt.Errorf("shop %d invalid exact source path", route.ServerShopID)
		}
		// Explicit routes preserve a previously selected service scope. Never
		// infer routing IDs from file names or choose a collision by file order.
		script, e := ReadScript(a, name)
		if e != nil {
			return out, e
		}
		shop, e := ParseNativeItemShop(script)
		if e != nil {
			return out, fmt.Errorf("shop %d: %w", route.ServerShopID, e)
		}
		c.Shops[key] = shop
	}
	out.Catalog, e = NewItemShops(c)
	return out, e
}

func ParseNativeItemShop(script ScriptRecord) (ItemShop, error) {
	s := ItemShop{Path: script.Path, SHA256: script.SHA256}
	var current *ItemShopOffer
	var tab uint32
	inList := false
	seenNPC, seenType := false, false
	readInt := func(at int) (uint32, error) {
		if at >= len(script.Cells) || script.Cells[at].Type != 0 || script.Cells[at].Value < 0 {
			return 0, fmt.Errorf("invalid source item shop integer at %d", at)
		}
		return uint32(script.Cells[at].Value), nil
	}
	for i := 0; i < len(script.Cells); i++ {
		c := script.Cells[i]
		if c.Type != 3 {
			continue
		}
		switch c.Text {
		case "[NPC]", "[npc]":
			if seenNPC {
				return s, fmt.Errorf("duplicate shop NPC")
			}
			n, e := readInt(i + 1)
			if e != nil {
				return s, e
			}
			s.Npc = n
			seenNPC = true
			i++
		case "[type]":
			if seenType || i+1 >= len(script.Cells) || script.Cells[i+1].Type != 6 {
				return s, fmt.Errorf("invalid/duplicate shop type")
			}
			s.Type = script.Cells[i+1].Text
			seenType = true
			i++
		case "[tab]":
			if current != nil {
				return s, fmt.Errorf("unclosed item before tab")
			}
			tab++
			inList = false
		case "[sell item list]":
			if current != nil || inList {
				return s, fmt.Errorf("nested sell list")
			}
			inList = true
		case "[/sell item list]":
			if current != nil {
				return s, fmt.Errorf("unclosed item/sell list")
			}
			// Odruz's source tab 3 has items outside a sell-list opener and
			// an orphan closer. The existing importer omits those items;
			// retain that admission boundary rather than enabling a new tab.
			inList = false
		case "[item]":
			if !inList {
				continue
			}
			if current != nil {
				return s, fmt.Errorf("nested source item")
			}
			n, e := readInt(i + 1)
			if e != nil {
				return s, e
			}
			current = &ItemShopOffer{Tab: tab, Index: n}
			i++
		case "[index]":
			if current == nil {
				continue
			}
			n, e := readInt(i + 1)
			if e != nil || n == 0 {
				return s, fmt.Errorf("invalid shop template")
			}
			if current.Template != 0 {
				return s, fmt.Errorf("duplicate shop template")
			}
			current.Template = n
			i++
		case "[purchase amount]":
			if current == nil {
				continue
			}
			n, e := readInt(i + 1)
			if e != nil {
				return s, e
			}
			current.PurchaseAmount = n
			i++
		case "[need material]":
			if current == nil {
				return s, fmt.Errorf("payment outside source item")
			}
			if current.Materials != nil {
				return s, fmt.Errorf("duplicate payment block")
			}
			closed := false
			for j := i + 1; j < len(script.Cells); j += 2 {
				if script.Cells[j].Type == 3 && script.Cells[j].Text == "[/need material]" {
					i = j
					closed = true
					break
				}
				tpl, e := readInt(j)
				if e != nil {
					return s, e
				}
				count, e := readInt(j + 1)
				if e != nil || tpl == 0 || count == 0 {
					return s, fmt.Errorf("invalid source payment pair")
				}
				current.Materials = append(current.Materials, ItemShopMaterial{Template: tpl, Count: count})
			}
			if !closed {
				return s, fmt.Errorf("unclosed source payment")
			}
		case "[/item]":
			if current == nil {
				continue
			}
			if current.Template == 0 {
				return s, fmt.Errorf("source offer lacks template")
			}
			if current.PurchaseAmount == 0 {
				current.PurchaseAmount = 1
			}
			s.Offers = append(s.Offers, *current)
			current = nil
		}
	}
	if current != nil || inList || len(s.Offers) == 0 {
		return s, fmt.Errorf("unclosed/empty source shop")
	}
	return s, nil
}
