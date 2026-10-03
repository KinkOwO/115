package cashshop

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
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
	err := c.resolveEquipmentEntries(index, func(name string) (catalog.ScriptRecord, error) {
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
			err := c.resolveEquipmentEntries(idx, func(name string) (catalog.ScriptRecord, error) {
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
	p := nativePilot(t, false)
	if !digestValid(p.Config.EquipmentIndexHash) {
		t.Fatal("equipment index hash missing")
	}
	eggs := 0
	for _, e := range p.Config.Entries {
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
		prod, dt, err := p.Config.classify(e)
		if err != nil {
			t.Fatal("egg rejected despite dedicated delivery", err)
		}
		if dt.Kind != "[creature]" || dt.Slots != [2]uint16{0, 139} {
			t.Fatal("egg delivery type mismatch", dt)
		}
		_ = prod
	}
	if eggs != 12 {
		t.Fatal("expected twelve source eggs", eggs)
	}
	if p.EnabledCount() == 0 {
		t.Fatal("no ordinary products enabled from the native source")
	}
	t.Log("12 eggs resolve with creature equipment type, egg subtype and hatch output; dedicated delivery enabled")
}

func TestShopPilotCreatureEggPurchase(t *testing.T) {
	p := nativePilot(t, true)
	l := &packLedger{state: json.RawMessage(`{}`)}
	_, _, err := p.Purchase(context.Background(), l, 1, 1, "egg-purchase-test-0001", []protocol.CeraCartItem{{Product: 3300000, Quantity: 1}})
	if err != nil {
		t.Fatal("egg purchase failed:", err)
	}
	bag, err := inventory.ReadBag(l.state)
	if err != nil {
		t.Fatal(err)
	}
	if len(bag.Items) != 0 {
		t.Fatalf("egg leaked to ordinary items: %+v", bag.Items)
	}
	if len(bag.Special[7]) != 1 {
		t.Fatalf("expected 1 creature item in space 7, got %d", len(bag.Special[7]))
	}
	if bag.Special[7][0].Slot != 0 || bag.Special[7][0].Template != 63006 {
		t.Fatalf("unexpected creature item: %+v", bag.Special[7][0])
	}

	// Purchase a second egg and verify it occupies next available slot
	_, _, err = p.Purchase(context.Background(), l, 1, 1, "egg-purchase-test-0002", []protocol.CeraCartItem{{Product: 3300001, Quantity: 1}})
	if err != nil {
		t.Fatal("second egg purchase failed:", err)
	}
	bag, err = inventory.ReadBag(l.state)
	if err != nil {
		t.Fatal(err)
	}
	if len(bag.Special[7]) != 2 {
		t.Fatalf("expected 2 creature items, got %d", len(bag.Special[7]))
	}
	if bag.Special[7][1].Slot != 1 || bag.Special[7][1].Template != 63007 {
		t.Fatalf("unexpected second creature item: %+v", bag.Special[7][1])
	}
}
