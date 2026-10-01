package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/json"
	"testing"
)

func prewarmRole(t *testing.T, bag inventory.Bag) storage.Character {
	t.Helper()
	raw, err := inventory.SaveBag(json.RawMessage(`{"other_saved_field":123}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	return storage.Character{State: raw}
}

func TestRolePVFPrewarmPreservesUnknownInventoryTemplates(t *testing.T) {
	role := prewarmRole(t, inventory.Bag{Version: "ordinary-bag-v1",
		Items: []inventory.BagItem{{Slot: 65, Template: 10310180, Amount: 1}},
		Worn:  []inventory.BagEquipment{{Slot: 12, Template: 4294967295}},
	})
	original := append([]byte(nil), role.State...)
	s := &character.Service{Equipment: &inventory.EquipmentCatalog{Full: &inventory.FullEquipmentCatalog{}}}
	if err := prepareRolePVFDetails(context.Background(), s, nil, nil, role); err != nil {
		t.Fatal("optional prefetch changed saved-template login admission", err)
	}
	if !bytes.Equal(original, role.State) {
		t.Fatal("prefetch changed inventory or unrelated saved fields")
	}
}

// Called by the native combined-startup test after its projection checks.
// The fixture includes the actual failing template and equipment in legacy
// Bag.Items, so a source-enabled catalog exercises the reported failure.
func verifyRolePVFPrewarmNative(t *testing.T, c pvfCoreCatalogs) {
	t.Helper()
	l := *c.loot
	l.Items = make(map[uint32]catalog.LootItem, len(c.loot.Items))
	for id, item := range c.loot.Items {
		l.Items[id] = item
	}
	if err := l.SupplementItemIndex(*c.items); err != nil {
		t.Fatal(err)
	}
	if !l.HasRuntimeDetails() {
		t.Fatal("fixture did not exercise source-backed prefetch")
	}
	var stackable, equipment uint32
	for id, item := range c.items.Items {
		if id > 1 && item.Kind == "stackable" && stackable == 0 {
			stackable = id
		}
		if item.Kind == "equipment" && equipment == 0 {
			equipment = id
		}
	}
	if stackable == 0 || equipment == 0 {
		t.Fatal("missing native prefetch fixtures")
	}
	binding, present := c.items.Items[10310180]
	t.Logf("reported saved template 10310180: source present=%t kind=%s path=%s", present, binding.Kind, binding.Path)
	role := prewarmRole(t, inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{
		{Slot: 65, Template: 10310180, Amount: 1},
		{Slot: 66, Template: stackable, Amount: 1},
		{Slot: 67, Template: equipment, Amount: 1},
		{Slot: 68, Template: 4294967295, Amount: 1},
	}})
	original := append([]byte(nil), role.State...)
	s := &character.Service{Equipment: &inventory.EquipmentCatalog{Full: c.equipment}}
	items := &loot.Service{Catalog: l, Equipment: s.Equipment}
	if err := prepareRolePVFDetails(context.Background(), s, nil, items, role); err != nil {
		t.Fatal("native saved-template prefetch rejected selection", err)
	}
	if !bytes.Equal(original, role.State) {
		t.Fatal("native prefetch changed persisted state")
	}
	if err := l.CloseDetails(); err != nil {
		t.Fatal(err)
	}
	known := prewarmRole(t, inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: stackable, Amount: 1}}})
	if err := prepareRolePVFDetails(context.Background(), s, nil, items, known); err == nil {
		t.Fatal("known stackable source failure was ignored")
	}
	if err := c.equipment.Close(); err != nil {
		t.Fatal(err)
	}
	known = prewarmRole(t, inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: equipment, Amount: 1}}})
	if err := prepareRolePVFDetails(context.Background(), s, nil, items, known); err == nil {
		t.Fatal("legacy Bag.Items equipment did not use its native detail source")
	}
}
