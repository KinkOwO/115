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
	if e = c.resolveCreatureEquipment(equipment, resolve); e != nil {
		return c, e
	}
	return c, c.validate()
}

// Eggs use equipment.lst, while creature food and rename cards use
// stackable.lst. Never replace a successfully resolved stackable definition.
func (c *PilotConfig) resolveCreatureEquipment(index catalog.ScriptRecord, resolve func(string) (catalog.ScriptRecord, error)) error {
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
		if e.Section != "[creature]" || e.ImportError != "template missing from stackable.lst" {
			continue
		}
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
	return c, c.validate()
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

func (c PilotConfig) classify(v OrdinaryProduct) (Product, deliveryType, error) {
	var p Product
	var h deliveryType
	fail := func(reason string) (Product, deliveryType, error) { return p, h, fmt.Errorf("%s", reason) }
	openAll := os.Getenv("DFO_SHOP_OPEN_ALL") == "1"
	if v.ImportError != "" {
		return fail(v.ImportError)
	}
	if len(v.Row) != 14 {
		return fail("invalid price row width")
	}
	r := v.Row
	for _, i := range []int{0, 1, 2, 3, 4, 5, 6, 7, 9, 10, 11, 13} {
		if r[i].Type != 0 {
			return fail("invalid price cell type")
		}
	}
	if r[0].Value <= 0 || r[1].Value <= 0 || r[2].Value <= 0 {
		return fail("invalid product, units or template")
	}
	if !openAll && (r[2].Value > 112000 || r[5].Value <= 0) {
		return fail("invalid product, units or Cera price")
	}
	ceraPrice := uint32(0)
	if r[5].Value > 0 {
		ceraPrice = uint32(r[5].Value)
	}
	if !openAll {
		_, isContract, err := entryContract(v)
		if err != nil {
			return fail(err.Error())
		}
		if isContract {
			return fail("premium contract requires account activation")
		}

		switch v.Section {
		case "[item mod or ext]":
			return fail("expansion requires capacity state, upgrade prerequisites and refresh handler")
		case "[item period or contract]":
			return fail("contract family requires account activation")
		case "[creature]":
			if !isCreatureEgg(v) {
				return fail("creature requires dedicated index, inventory and hatch handlers")
			}
		case "[package related]":
			return fail("package family requires sale policy and reward delivery handlers")
		}
		if v.Section != "[item]" && v.Section != "[item etc]" && v.Section != "[item second]" && v.Section != "[item event]" {
			if v.Section != "[creature]" || !isCreatureEgg(v) {
				return fail("unimplemented shop family")
			}
		}
		for _, i := range []int{3, 4, 6, 7, 10} {
			if r[i].Value != 0 {
				return fail("alternate currency or special price policy")
			}
		}
		if r[12].Type != 6 || r[12].Text != "" || r[13].Value != -1 {
			return fail("sale condition or date policy requires handler")
		}
		if r[9].Value != 0 && r[9].Value != 4 {
			return fail("hidden or unverified display policy")
		}
		if !digestValid(v.Item.SHA256) || (!strings.HasPrefix(v.IndexPath, "stackable/") && !strings.HasPrefix(v.IndexPath, "equipment/creature/")) {
			return fail("missing script/index provenance")
		}
		alternate := path.Join(path.Dir(v.IndexPath), "(r)"+path.Base(v.IndexPath))
		if v.Item.Path != v.IndexPath && v.Item.Path != alternate {
			return fail("script does not match index path")
		}
		for _, name := range []string{"[purchasing limit]", "[not stackable buy]", "[immediately adaptive product]", "[specific product mileage]", "[auto open booster item]"} {
			for _, t := range c.Policies[name] {
				if t.Type == 0 && t.Value == r[0].Value {
					if r[1].Value == 1 && (name == "[not stackable buy]" || name == "[immediately adaptive product]") {
						continue
					}
					return fail("unimplemented purchase policy " + name)
				}
			}
		}
		// An immediately-applied item remains special when another SKU sells a
		// multipack of the same template (for example account counters).
		for _, t := range c.Policies["[immediately adaptive product]"] {
			if t.Type != 0 {
				continue
			}
			for _, other := range c.Entries {
				if len(other.Row) == 14 && other.Row[0].Value == t.Value && other.Row[1].Value == r[1].Value {
					if r[1].Value == 1 {
						continue
					}
					return fail("template requires immediate-effect delivery")
				}
			}
		}
	}
	var e error
	h, e = ordinaryHandler(v.Item)
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
		if len(v.Row) != 14 {
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
