package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

func TestSupplementStackables(t *testing.T) {
	c := LootCatalog{
		Source: pvf.ArchiveSnapshot{
			Checksum: "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80",
		},
		Items: map[uint32]LootItem{
			1000: {ID: 1000, Kind: "stackable", StackableType: "[material]", Grade: 5},
		},
	}
	indexPath := "../../configs/items.index.json"
	if _, err := os.Stat(indexPath); err != nil {
		t.Skip("items.index.json missing, skipping")
	}

	if err := c.SupplementStackables(indexPath); err != nil {
		t.Fatalf("SupplementStackables failed: %v", err)
	}

	// Original monster drop item must remain intact
	it1000, ok := c.Items[1000]
	if !ok || it1000.Grade != 5 {
		t.Fatalf("original item corrupted: %+v", it1000)
	}

	// Consumables must now be present
	for _, id := range []uint32{1106, 1112, 2660671, 10000541} {
		item, found := c.Items[id]
		if !found {
			t.Errorf("expected item %d to be supplemented, but not found", id)
		}
		if item.Kind != "stackable" {
			t.Errorf("item %d kind: got %s, want stackable", id, item.Kind)
		}
		if item.StackableType == "" {
			t.Errorf("item %d stackable_type is empty", id)
		}
	}
}
