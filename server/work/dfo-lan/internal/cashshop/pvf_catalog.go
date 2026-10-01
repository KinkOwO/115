package cashshop

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"os"
	"path"
	"strings"
)

func shopSections(cells []pvf.Token) map[string][]pvf.Token {
	out := map[string][]pvf.Token{}
	section := ""
	for _, t := range cells {
		if t.Type == 3 {
			section = t.Text
			continue
		}
		out[section] = append(out[section], t)
	}
	return out
}

func ImportPilot(a *pvf.Archive) (PilotConfig, error) {
	shop, e := catalog.ResolveScript(a, "etc/(r)cerashop.etc")
	if e != nil {
		return PilotConfig{}, e
	}
	index, e := catalog.ResolveScript(a, "list/stackable.lst")
	if e != nil {
		return PilotConfig{}, e
	}
	resolve := func(name string) (catalog.ScriptRecord, error) { return catalog.ResolveScript(a, name) }
	c, e := importShopScripts(a.Snapshot(), shop, index, resolve)
	if e != nil {
		return c, e
	}
	equipment, e := resolve("list/equipment.lst")
	if e != nil {
		return c, e
	}
	if e = c.resolveEquipmentEntries(equipment, resolve); e != nil {
		return c, e
	}
	return c, c.Validate()
}

// Single avatar pieces and creature eggs live in equipment.lst, not
// stackable.lst. Import marks both families "template missing from
// stackable.lst" and this pass resolves them from the equipment index;
// every other family keeps that marker as a rejection reason.
func (c *PilotConfig) resolveEquipmentEntries(index catalog.ScriptRecord, resolve func(string) (catalog.ScriptRecord, error)) error {
	if !digestValid(index.SHA256) {
		return fmt.Errorf("missing equipment index hash")
	}
	rows, err := catalog.ParseIndex(index.Cells)
	if err != nil {
		return err
	}
	refs := make(map[uint32]string, len(rows))
	for _, row := range rows {
		refs[row.ID] = row.Path
	}
	c.EquipmentIndexHash = index.SHA256
	for i := range c.Entries {
		e := &c.Entries[i]
		if e.ImportError != "template missing from stackable.lst" {
			continue
		}
		switch e.Section {
		case "[creature]":
			name, ok := refs[uint32(e.Row[1].Value)]
			if !ok {
				e.ImportError = "template missing from stackable.lst and equipment.lst"
				continue
			}
			name = path.Clean(name)
			if !strings.HasPrefix(name, "equipment/creature/") || path.Ext(name) != ".equ" {
				e.ImportError = "creature product resolves to non-creature equipment"
				continue
			}
			script, err := resolve(name)
			if err != nil {
				e.ImportError = err.Error()
				continue
			}
			alternate := path.Join(path.Dir(name), "(r)"+path.Base(name))
			if !digestValid(script.SHA256) || (script.Path != name && script.Path != alternate) {
				e.ImportError = "creature equipment script does not match index"
				continue
			}
			e.IndexPath, e.Item, e.ImportError = name, script, ""
		case "[dont trade avatar]":
			name, ok := refs[uint32(e.Row[1].Value)]
			if !ok {
				e.ImportError = "template missing from stackable.lst and equipment.lst"
				continue
			}
			name = path.Clean(name)
			if path.Ext(name) != ".equ" || !strings.Contains(name, "avatar") {
				e.ImportError = "avatar product resolves to non-avatar equipment"
				continue
			}
			script, err := resolve(name)
			if err != nil {
				e.ImportError = err.Error()
				continue
			}
			alternate := path.Join(path.Dir(name), "(r)"+path.Base(name))
			if !digestValid(script.SHA256) || (script.Path != name && script.Path != alternate) {
				e.ImportError = "avatar equipment script does not match index"
				continue
			}
			e.IndexPath, e.Item, e.ImportError = name, script, ""
		}
	}
	return nil
}

func importShopScripts(source pvf.ArchiveSnapshot, shop, index catalog.ScriptRecord, resolve func(string) (catalog.ScriptRecord, error)) (PilotConfig, error) {
	c := PilotConfig{Schema: shopSchema, Source: source, Policies: map[string][]pvf.Token{}}
	rows, e := catalog.ParseIndex(index.Cells)
	if e != nil {
		return c, e
	}
	refs := map[uint32]string{}
	for _, r := range rows {
		name := r.Path
		if !strings.HasPrefix(name, "stackable/") {
			name = path.Join("stackable", name)
		}
		refs[r.ID] = name
	}
	c.PriceScriptHash = shop.SHA256
	c.IndexHash = index.SHA256
	sections := shopSections(shop.Cells)
	for _, name := range []string{"[purchasing limit]", "[not stackable buy]", "[immediately adaptive product]", "[specific product mileage]", "[auto open booster item]", "[cargo 1]", "[cargo 2]", "[cargo account]"} {
		c.Policies[name] = sections[name]
	}
	cache := map[string]catalog.ScriptRecord{}
	// These sections share 14-cell price rows, but only the ordinary families
	// have delivery handlers. Retain special rows for diagnostics, never infer
	// delivery semantics from their price layout or stackable type.
	for _, section := range []string{"[item]", "[item etc]", "[item second]", "[item event]", "[item mod or ext]", "[item period or contract]", "[creature]", "[package related]"} {
		cells := sections[section]
		if len(cells)%14 != 0 {
			return c, fmt.Errorf("%s has incomplete price row", section)
		}
		for at := 0; at < len(cells); at += 14 {
			r := append([]pvf.Token(nil), cells[at:at+14]...)
			entry := OrdinaryProduct{Section: section, Row: r}
			if r[1].Type != 0 || r[1].Value <= 0 {
				entry.ImportError = "invalid item reference"
			} else if name, ok := refs[uint32(r[1].Value)]; !ok {
				entry.ImportError = "template missing from stackable.lst"
			} else {
				entry.IndexPath = name
				e = nil
				script, ok := cache[name]
				if !ok {
					script, e = resolve(name)
					if e == nil {
						cache[name] = script
					}
				}
				if e != nil {
					entry.ImportError = e.Error()
				} else {
					entry.Item = script
				}
			}
			c.Entries = append(c.Entries, entry)
		}
	}
	// The avatar tab and the package tab use their own price-row shapes:
	// [dont trade avatar] keeps the 14-cell layout (Cera at cell 5, the
	// trailing string closes the row) while [package] drops the units cell
	// entirely (13 cells, Cera at cell 4). Templates resolve through
	// stackable.lst except avatar pieces, which resolveEquipmentEntries
	// repairs from equipment.lst.
	for _, spec := range []struct {
		section string
		width   int
	}{{"[dont trade avatar]", 14}, {"[package]", 13}} {
		cells := sections[spec.section]
		if len(cells)%spec.width != 0 {
			return c, fmt.Errorf("%s has incomplete price row", spec.section)
		}
		for at := 0; at < len(cells); at += spec.width {
			r := append([]pvf.Token(nil), cells[at:at+spec.width]...)
			if r[0].Type != 0 || r[0].Value <= 0 || r[1].Type != 0 || r[1].Value <= 0 {
				// Placeholder rows (product -1) are not merchandise; unlike the
				// ordinary families they are dropped instead of retained for
				// diagnostics because validate cannot carry them.
				continue
			}
			entry := OrdinaryProduct{Section: spec.section, Row: r}
			if name, ok := refs[uint32(r[1].Value)]; !ok {
				entry.ImportError = "template missing from stackable.lst"
			} else {
				entry.IndexPath = name
				e = nil
				script, ok := cache[name]
				if !ok {
					script, e = resolve(name)
					if e == nil {
						cache[name] = script
					}
				}
				if e != nil {
					entry.ImportError = e.Error()
				} else {
					entry.Item = script
				}
			}
			c.Entries = append(c.Entries, entry)
		}
	}
	return c, c.Validate()
}

func isCreatureEgg(v OrdinaryProduct) bool {
	if v.Section != "[creature]" || !strings.HasPrefix(v.IndexPath, "equipment/creature/") {
		return false
	}
	for i, t := range v.Item.Cells {
		if t.Type == 3 && t.Text == "[equipment type]" {
			if i+1 < len(v.Item.Cells) && v.Item.Cells[i+1].Text == "[creature]" {
				return true
			}
		}
	}
	return false
}

// checkPolicies rejects products the shop policies reserve for behaviours the
// server does not implement (purchase limits, non-stackable buys, immediate
// application, mileage pricing, auto-open). Template 1 (Cera coin) is exempt
// from the two markers the ordinary path explicitly permits.
func (c PilotConfig) checkPolicies(productID, template int32) error {
	for _, name := range []string{"[purchasing limit]", "[not stackable buy]", "[immediately adaptive product]", "[specific product mileage]", "[auto open booster item]"} {
		// 发布模式保留本地销售限制策略，立即生效及自动开启仍需真实处理器。
		if c.Release && (name == "[purchasing limit]" || name == "[not stackable buy]" || name == "[specific product mileage]") {
			continue
		}
		for _, t := range c.Policies[name] {
			if t.Type == 0 && t.Value == productID {
				if template == 1 && (name == "[not stackable buy]" || name == "[immediately adaptive product]") {
					continue
				}
				return fmt.Errorf("unimplemented purchase policy %s", name)
			}
		}
	}
	return nil
}

// deriveImmediateTemplates answers the template set sold by any
// [immediately adaptive product] SKU without mutating the receiver.
func (c PilotConfig) deriveImmediateTemplates() map[int32]bool {
	productTemplate := make(map[int32]int32, len(c.Entries))
	for _, v := range c.Entries {
		if width := len(v.Row); width == 13 || width == 14 {
			productTemplate[v.Row[0].Value] = v.Row[1].Value
		}
	}
	immediate := map[int32]bool{}
	for _, t := range c.Policies["[immediately adaptive product]"] {
		if t.Type == 0 {
			if tpl, ok := productTemplate[t.Value]; ok {
				immediate[tpl] = true
			}
		}
	}
	return immediate
}

// isSaleWindowDate answers whether the col12 marker is a YYYYMMDD sale
// window (the only non-empty col12 shape the seasonal families use).
func isSaleWindowDate(s string) bool {
	if len(s) != 8 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// classifyAvatarPiece admits the [dont trade avatar] family: one avatar
// piece per purchase, priced in Cera at cell 5. The audited source shape is
// rigid - cells 3..9 zero, 10..12 -1, a trailing empty string - and every
// deviation stays rejected until captured live.
func (c PilotConfig) classifyAvatarPiece(v OrdinaryProduct) (Product, deliveryType, error) {
	var p Product
	var h deliveryType
	fail := func(reason string) (Product, deliveryType, error) { return p, h, fmt.Errorf("%s", reason) }
	r := v.Row
	if len(r) != 14 {
		return fail("invalid avatar price row width")
	}
	for _, i := range []int{0, 1, 2, 3, 4, 5, 6, 7, 9, 10, 11} {
		if r[i].Type != 0 {
			return fail("invalid avatar price cell type")
		}
	}
	if r[0].Value <= 0 || r[1].Value <= 0 {
		return fail("invalid avatar product or template")
	}
	if r[5].Value <= 0 {
		return fail("avatar product without Cera price")
	}
	if r[2].Value != 3 && r[2].Value != 4 {
		return fail(fmt.Sprintf("unverified avatar display class %d", r[2].Value))
	}
	for _, i := range []int{3, 4, 6, 7, 8, 9} {
		if r[i].Value != 0 {
			return fail(fmt.Sprintf("avatar cell %d carries an unsupported policy", i))
		}
	}
	for _, i := range []int{10, 11, 12} {
		if r[i].Value != -1 {
			return fail(fmt.Sprintf("avatar cell %d carries an unsupported marker", i))
		}
	}
	if r[13].Type != 6 || r[13].Text != "" {
		return fail("avatar sale condition requires handler")
	}
	if e := c.checkPolicies(r[0].Value, r[1].Value); e != nil {
		return fail(e.Error())
	}
	if !digestValid(v.Item.SHA256) || !strings.HasPrefix(v.IndexPath, "equipment/") || !strings.Contains(v.IndexPath, "avatar") || path.Ext(v.IndexPath) != ".equ" {
		return fail("missing avatar script/index provenance")
	}
	alternate := path.Join(path.Dir(v.IndexPath), "(r)"+path.Base(v.IndexPath))
	if v.Item.Path != v.IndexPath && v.Item.Path != alternate {
		return fail("avatar script does not match index path")
	}
	// 实机取证 2026-09-23:客户端购买时装单件时购物车条目 Kind 与源行 cell2
	// 一致(3107370..73 均为 4),Quote 的严格比对要求把它投影到 Product.Kind。
	return Product{ID: uint32(r[0].Value), Template: uint32(r[1].Value), Units: 1, Cera: uint32(r[5].Value), Kind: uint8(r[2].Value), Enabled: true},
		deliveryType{Kind: "[avatar]", Slots: [2]uint16{0, 209}, Limit: 1}, nil
}

// classifyPackage admits the [package] tab: closed boxes and packages sold
// for Cera at cell 4. Packages with [package data] expand at purchase, boxes
// with a booster pool open through the booster flow; both are audited source
// families. The 77 source rows with an alternative-currency cell (12M/15M)
// stay rejected.
func (c PilotConfig) classifyPackage(v OrdinaryProduct) (Product, deliveryType, error) {
	var p Product
	var h deliveryType
	fail := func(reason string) (Product, deliveryType, error) { return p, h, fmt.Errorf("%s", reason) }
	r := v.Row
	if len(r) != 13 {
		return fail("invalid package price row width")
	}
	for _, i := range []int{0, 1, 2, 3, 4, 5, 6, 8, 9, 10, 11} {
		if r[i].Type != 0 {
			return fail("invalid package price cell type")
		}
	}
	if r[0].Value <= 0 || r[1].Value <= 0 {
		return fail("invalid package product or template")
	}
	if r[4].Value <= 0 {
		return fail("package product without Cera price")
	}
	if r[2].Value != 0 {
		return fail(fmt.Sprintf("package alternative price policy %d", r[2].Value))
	}
	for _, i := range []int{3, 5, 6} {
		if r[i].Value != 0 {
			return fail(fmt.Sprintf("package cell %d carries an unsupported policy", i))
		}
	}
	if r[8].Value != 0 && r[8].Value != 90 {
		return fail(fmt.Sprintf("unverified package display policy %d", r[8].Value))
	}
	if r[9].Value != 0 {
		return fail(fmt.Sprintf("unverified package tag %d", r[9].Value))
	}
	if r[10].Value != -1 || r[11].Value != -1 {
		return fail("package sale condition requires handler")
	}
	if r[7].Type != 6 || r[12].Type != 6 {
		return fail("invalid package name row")
	}
	if e := c.checkPolicies(r[0].Value, r[1].Value); e != nil {
		return fail(e.Error())
	}
	if !digestValid(v.Item.SHA256) || !strings.HasPrefix(v.IndexPath, "stackable/") {
		return fail("missing package script/index provenance")
	}
	alternate := path.Join(path.Dir(v.IndexPath), "(r)"+path.Base(v.IndexPath))
	if v.Item.Path != v.IndexPath && v.Item.Path != alternate {
		return fail("package script does not match index path")
	}
	h, e := packageHandler(v.Item)
	if e != nil {
		return p, h, e
	}
	return Product{ID: uint32(r[0].Value), Template: uint32(r[1].Value), Units: 1, Cera: uint32(r[4].Value), Enabled: true}, h, nil
}

func (c PilotConfig) classify(v OrdinaryProduct) (Product, deliveryType, error) {
	var p Product
	var h deliveryType
	fail := func(reason string) (Product, deliveryType, error) { return p, h, fmt.Errorf("%s", reason) }
	openAll := os.Getenv("DFO_SHOP_OPEN_ALL") == "1"
	if v.ImportError != "" {
		return fail(v.ImportError)
	}
	switch v.Section {
	case "[dont trade avatar]":
		return c.classifyAvatarPiece(v)
	case "[package]":
		return c.classifyPackage(v)
	}
	if len(v.Row) != 14 {
		return fail("invalid price row width")
	}
	r := v.Row
	// 保留上游已支持的背包扩展券入口，发布模式不改变其原有放行条件。
	invExtWhitelist := map[int32]bool{3000147: true, 3000148: true}
	allowAny := openAll || invExtWhitelist[r[0].Value]
	for _, i := range []int{0, 1, 2, 3, 4, 5, 6, 7, 9, 10, 11, 13} {
		if r[i].Type != 0 {
			return fail("invalid price cell type")
		}
	}
	if r[0].Value <= 0 || r[1].Value <= 0 || r[2].Value <= 0 {
		return fail("invalid product, units or template")
	}
	if !allowAny && (r[2].Value > 112000 || r[5].Value <= 0) {
		return fail("invalid product, units or Cera price")
	}
	ceraPrice := uint32(0)
	if r[5].Value > 0 {
		ceraPrice = uint32(r[5].Value)
	}

	// 发布目录（目录中 "release": true）与 DFO_SHOP_OPEN_ALL 都是放宽门禁的开关：
	// 前者只放宽有来源依据的销售限制段，来源校验、替代货币与发货处理器仍然生效；
	// 后者整段跳过，仅用于取证。
	if !allowAny {
		// 契约必须走上游账号激活流程，发布模式也不能作为普通物品发放。
		_, isContract, err := entryContract(v)
		if err != nil {
			return fail(err.Error())
		}
		if isContract {
			return fail("premium contract requires account activation")
		}
		if !c.Release {
			switch v.Section {
			case "[item mod or ext]":
				return fail("expansion requires capacity state, upgrade prerequisites and refresh handler")
			case "[item period or contract]":
				return fail("contract family requires effect, renewal and expiry handlers")
			case "[creature]":
				if !isCreatureEgg(v) {
					return fail("creature requires dedicated index, inventory and hatch handlers")
				}

			}
			if v.Section != "[item]" && v.Section != "[item etc]" && v.Section != "[item second]" && v.Section != "[item event]" && v.Section != "[package related]" {
				if v.Section != "[creature]" || !isCreatureEgg(v) {
					return fail("unimplemented shop family")
				}
			}
		}
		// 当前目录的 r[3]、r[4]、r[7] 没有数据，r[6] 仅一行有值；r[10] 则在
		// petit_friends/luckybag 的 120 行中保存连续的类型引用（70383..70423），
		// 实际点券价格仍在 r[5]。发布目录只放宽 r[10]，所有货币列继续受限，
		// 且 r[5] 仍必须大于零。
		alt := []int{3, 4, 6, 7}
		if !c.Release {
			alt = append(alt, 10)
		}
		for _, i := range alt {
			if r[i].Value != 0 {
				return fail("alternate currency or special price policy")
			}
		}
		if !c.Release {
			if r[12].Type != 6 || (r[12].Text != "" && !((v.Section == "[item event]" || v.Section == "[package related]") && isSaleWindowDate(r[12].Text))) || r[13].Value != -1 {
				return fail("sale condition or date policy requires handler")
			}
			if r[9].Value != 0 && r[9].Value != 4 && !((v.Section == "[item event]" || v.Section == "[package related]") && (r[9].Value == 1 || r[9].Value == 90)) {
				return fail("hidden or unverified display policy")
			}
		}
		if !digestValid(v.Item.SHA256) || (!strings.HasPrefix(v.IndexPath, "stackable/") && !strings.HasPrefix(v.IndexPath, "equipment/creature/")) {
			return fail("missing script/index provenance")
		}
		alternate := path.Join(path.Dir(v.IndexPath), "(r)"+path.Base(v.IndexPath))
		if v.Item.Path != v.IndexPath && v.Item.Path != alternate {
			return fail("script does not match index path")
		}
		if e := c.checkPolicies(r[0].Value, r[1].Value); e != nil {
			return fail(e.Error())
		}
		// An immediately-applied item remains special when another SKU sells a
		// multipack of the same template (for example account counters).
		immediate := c.immediateTemplates
		if immediate == nil {
			immediate = c.deriveImmediateTemplates()
		}
		if r[1].Value != 1 && immediate[r[1].Value] {
			return fail("template requires immediate-effect delivery")
		}
	}
	var e error
	if v.Section == "[package related]" {
		// The related family only sells boxes and packages: the delivery is
		// either the purchase-time [package data] expansion or the closed box,
		// never an ordinary stackable projection of the section row.
		h, e = packageHandler(v.Item)
	} else {
		h, e = ordinaryHandler(v.Item, c.Release)
	}
	if e != nil {
		return p, h, e
	}
	if r[1].Value == 1 {
		h = deliveryType{
			Kind:  "[coin]",
			Slots: [2]uint16{1, 1},
			Limit: math.MaxUint32,
		}
	}
	p = Product{ID: uint32(r[0].Value), Template: uint32(r[1].Value), Units: uint32(r[2].Value), Cera: ceraPrice, Enabled: true}
	return p, h, nil
}

type ShopReportRow struct {
	Product     int32  `json:"product"`
	Template    int32  `json:"template"`
	Name        string `json:"name"`
	Section     string `json:"section"`
	Kind        string `json:"kind"`
	Enabled     bool   `json:"enabled"`
	Reason      string `json:"reason,omitempty"`
	Units       int32  `json:"units"`
	Cera        int32  `json:"cera"`
	ImportError string `json:"import_error,omitempty"`
}

func (c PilotConfig) Report() []ShopReportRow {
	out := []ShopReportRow{}
	for _, v := range c.Entries {
		if width := len(v.Row); width != 13 && width != 14 {
			continue
		}
		_, h, e := c.classify(v)
		row := ShopReportRow{Product: v.Row[0].Value, Template: v.Row[1].Value, Name: v.Row[8].Text, Section: v.Section, Kind: h.Kind, Enabled: e == nil, Units: v.Row[2].Value, Cera: v.Row[5].Value}
		row.ImportError = v.ImportError
		if e != nil {
			row.Reason = e.Error()
		}
		out = append(out, row)
	}
	return out
}
