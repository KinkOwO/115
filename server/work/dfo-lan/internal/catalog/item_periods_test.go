package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentItemPeriodCatalogMatchesSourceAndKnownTemplates(t *testing.T) {
	const source = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	file := filepath.Join("..", "..", "configs", "item-period-tags.json")
	templates, err := LoadItemPeriods(file, source)
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) < 100000 {
		t.Fatalf("only %d time-limited templates exported", len(templates))
	}
	contains := func(id uint32) bool {
		for _, template := range templates {
			if template == id {
				return true
			}
		}
		return false
	}
	if !contains(590012183) || !contains(100991331) || contains(10000660) {
		t.Fatal("known expiring/non-expiring templates classified incorrectly")
	}
	if _, err := LoadItemPeriods(file, "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Fatal("catalog from another PVF source accepted")
	}
}

func TestItemPeriodCatalogRejectsRepeatedTemplates(t *testing.T) {
	const source = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	data := ItemPeriodCatalog{Schema: ItemPeriodCatalogSchema, Templates: []uint32{12, 12}}
	data.Source.Checksum = source
	b, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(t.TempDir(), "periods.json")
	if err := os.WriteFile(file, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadItemPeriods(file, source); err == nil {
		t.Fatal("duplicate template accepted")
	}
}
