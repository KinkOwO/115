package cashshop

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
	"testing"
)

func TestCreatureEquipmentResolution(t *testing.T) {
	c := syntheticCatalog(t, "[etc]")
	food := c.Entries[0]
	food.Section = "[creature]"
	egg := food
	egg.Row = append([]pvf.Token(nil), food.Row...)
	egg.Row[0].Value = 3300000
	egg.Row[1].Value = 63006
	egg.Item = catalog.ScriptRecord{}
	egg.IndexPath = ""
	egg.ImportError = "template missing from stackable.lst"
	c.Entries = []OrdinaryProduct{food, egg}
	index := catalog.ScriptRecord{SHA256: strings.Repeat("b", 64), Cells: []pvf.Token{{Value: 63006}, {Type: 6, Text: "equipment/creature/egg_faras.equ"}, {Value: food.Row[1].Value}, {Type: 6, Text: "equipment/creature/not_food.equ"}}}
	calls := 0
	err := c.resolveCreatureEquipment(index, func(name string) (catalog.ScriptRecord, error) {
		calls++
		return catalog.ScriptRecord{Path: name, SHA256: strings.Repeat("c", 64)}, nil
	})
	if err != nil || calls != 1 || c.EquipmentIndexHash != index.SHA256 || c.Entries[1].ImportError != "" || c.Entries[0].Item.SHA256 != food.Item.SHA256 {
		t.Fatal("namespace resolution", err)
	}
	if _, _, err = c.classify(c.Entries[1]); err == nil {
		t.Fatal("egg admitted to ordinary bag")
	}
	for _, failure := range []string{"missing", "wrong family", "read error", "wrong script"} {
		t.Run(failure, func(t *testing.T) {
			c.Entries[1] = egg
			idx := index
			idx.Cells = append([]pvf.Token(nil), index.Cells...)
			if failure == "missing" {
				idx.Cells[0].Value = 1
			}
			if failure == "wrong family" {
				idx.Cells[1].Text = "equipment/character/weapon.equ"
			}
			err := c.resolveCreatureEquipment(idx, func(name string) (catalog.ScriptRecord, error) {
				if failure == "read error" {
					return catalog.ScriptRecord{}, fmt.Errorf("missing script")
				}
				return catalog.ScriptRecord{Path: "equipment/creature/wrong.equ", SHA256: strings.Repeat("c", 64)}, nil
			})
			if err != nil || c.Entries[1].ImportError == "" || c.Entries[1].Item.SHA256 != "" {
				t.Fatal("bad egg definition accepted", err)
			}
		})
	}
}

func TestCreatureCurrentSourceCatalog(t *testing.T) {
	source := "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	next, err := LoadPilot("../../configs/shop-next-candidate.json", source)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := LoadPilot("../../configs/shop-vault-release.json", source)
	if err != nil {
		t.Fatal(err)
	}
	if !digestValid(next.Config.EquipmentIndexHash) {
		t.Fatal("equipment index hash missing")
	}
	eggs := 0
	for _, e := range next.Config.Entries {
		if e.Section != "[creature]" {
			continue
		}
		if e.ImportError != "" {
			t.Fatal(e.Row[0].Value, e.ImportError)
		}
		if !strings.HasPrefix(e.IndexPath, "equipment/creature/") {
			continue
		}
		eggs++
		fields := shopSections(e.Item.Cells)
		kind, subtype, output := fields["[equipment type]"], fields["[sub type]"], fields["[output index]"]
		if len(kind) != 2 || kind[0].Text != "[creature]" || len(subtype) != 1 || subtype[0].Value != 1 || len(output) != 1 || output[0].Value <= 0 {
			t.Fatal("egg definition missing", e.Row[0].Value)
		}
		if _, _, err := next.Config.classify(e); err == nil {
			t.Fatal("egg admitted without dedicated delivery")
		}
	}
	if eggs != 12 {
		t.Fatal("expected twelve source eggs", eggs)
	}
	before, err := accepted.products()
	if err != nil {
		t.Fatal(err)
	}
	after, err := next.products()
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) {
		t.Fatal("ordinary product set changed")
	}
	for id, product := range before {
		if after[id] != product {
			t.Fatal("ordinary product changed", id)
		}
	}
	t.Log("12 eggs resolve with creature equipment type, egg subtype and hatch output; ordinary products unchanged; no incomplete delivery enabled")
}
