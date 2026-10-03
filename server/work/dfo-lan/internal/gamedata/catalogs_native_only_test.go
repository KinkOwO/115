package gamedata

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Even a valid old export cannot re-enable the retired runtime content path.
func TestRetiredCommerceJSONCannotSupplyRuntimeContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old-export.json")
	if err := os.WriteFile(path, []byte(`{"items":{},"boxes":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	c := &Catalogs{}
	if _, err := c.LoadBooster(path, path); err == nil {
		t.Fatal("booster JSON fallback was accepted")
	}
	if _, err := c.LoadShopPrices(path, "old"); err == nil {
		t.Fatal("price JSON fallback was accepted")
	}
	if _, err := c.LoadSelectionBoxes(path); err == nil {
		t.Fatal("selection JSON fallback was accepted")
	}
	c.Boosters = map[uint32]catalog.BoosterDefinition{7: {Template: 7}}
	c.Prices = &catalog.ShopPrices{Source: "native"}
	if b, err := c.LoadBooster(path, path); err != nil || b.Definitions[7].Template != 7 {
		t.Fatal("prepared native booster was not used", err)
	}
	if _, err := c.LoadShopPrices(path, "foreign"); err == nil {
		t.Fatal("foreign price source accepted")
	}
}

// Full effective-map fingerprints captured before removing the JSON branches.
// Source changes require a fresh read-only audit, not an edited game export.
func TestNativeCommerceCurrentArchiveFingerprint(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete native content fingerprints")
	}
	c, err := PrepareCatalogs(CatalogInputs{Selection: "items,boosters,prices,selection-boxes", ArchivePath: path, ArchiveChecksum: "8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934", DerivedCacheDir: "-"}, CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name  string
		value any
		want  string
	}{
		{"boosters", c.Boosters, "5c4c1981108135963854088455784e5e1efb942ad4a05aa93aebba06b29be073"},
		{"prices", c.Prices.Items, "369d46db21a22bbb0e4bcec96f8a8b86375be1293c40e64b6fbb16cc2d80d16c"},
		{"selection", c.SelectionBoxes.Boxes, "e39da3de6fc00f056bffdff99f1bd560e8f1613b5d86fe1a84b9eed8cc32445c"},
		{"fixed", c.SelectionBoxes.Fixed, "7258bbb97413befee6167c7ee617029713cfb9beb1b492cd6fe175724aacc220"},
		{"unparsed", c.SelectionBoxes.Unparsed, "d46e7f427e6acc454407bfa406e7e3bac6a28c025cca55e3649c43a5736e9780"},
		{"rejected", c.SelectionBoxes.Rejected, "e720151097748fe344478aa60cf26af7243813ed087572863484c91b03afbb0e"},
	} {
		raw, err := json.Marshal(row.value)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != row.want {
			t.Fatalf("%s effective content changed: %s", row.name, got)
		}
	}
}
