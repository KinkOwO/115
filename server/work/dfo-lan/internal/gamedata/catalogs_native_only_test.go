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

func TestRetiredItemAndEquipmentJSONCannotSupplyRuntimeContent(t *testing.T) {
	c := &Catalogs{}
	if _, err := c.OpenFullEquipment("old-export", "old"); err == nil {
		t.Fatal("equipment export fallback accepted")
	}
	loot := &catalog.LootCatalog{}
	if err := c.SupplementStackables(loot, "old-export.json"); err == nil {
		t.Fatal("item export fallback accepted")
	}
	s := &Source{mode: JSON}
	if _, err := s.ItemIndex("old-export.json"); err == nil {
		t.Fatal("JSON source supplied item bindings")
	}
}

func TestRetiredWorldAndQuestJSONCannotSupplyRuntimeContent(t *testing.T) {
	c := &Catalogs{}
	s := &Source{mode: JSON}
	if _, err := c.LoadWorld("old-world.json"); err == nil {
		t.Fatal("world JSON runtime fallback accepted")
	}
	if _, err := c.LoadQuests("old-quests.json"); err == nil {
		t.Fatal("quest JSON runtime fallback accepted")
	}
	if _, err := s.World("old-world.json"); err == nil {
		t.Fatal("JSON source supplied world content")
	}
	if _, err := s.Quests("old-quests.json"); err == nil {
		t.Fatal("JSON source supplied quest content")
	}
	c.World = &catalog.WorldCatalog{}
	c.Quests = &catalog.QuestCatalog{}
	if _, err := c.LoadWorld("missing.json"); err != nil {
		t.Fatal("prepared native world ignored", err)
	}
	if _, err := c.LoadQuests("missing.json"); err != nil {
		t.Fatal("prepared native quests ignored", err)
	}
}

func TestNativeWorldQuestsCurrentArchiveFingerprint(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete native world/quest fingerprints")
	}
	// Baseline verification must no longer discover these retired exports.
	c, err := PrepareCatalogs(CatalogInputs{Selection: "world,quests", ArchivePath: path, ArchiveChecksum: "8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934", DerivedCacheDir: "-", VerifyBaselines: true, WorldPath: "missing-world.json", QuestPath: "missing-quests.json"}, CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Quests.Close()
	w, q := c.World, c.Quests
	if len(w.Areas) != 694 || len(w.Towns) != 175 || len(q.Quests) != 2844 || len(w.NPCMoves) != 236 || len(w.NPCPlaces) != 1080 || len(w.EpisodeReturns) != 4 {
		t.Fatal("complete native world/quest scope changed")
	}
	for _, row := range []struct {
		name  string
		value any
		want  string
	}{
		{"world areas", w.Areas, "30bc07131ac38fab97f08439436e798a2717d03e2850938cf1a792f5dc4bd7d7"},
		{"world towns", w.Towns, "c2e08064dffe83229d4a5671862801a4bd4d5c45f91ac6262350b1060b27d50a"},
		{"town index", w.TownIndex, "42375fcfed4aab954035793f01a5954f31d8037ebe5f633ed1f11e3ab4500fcc"},
		{"world dungeons", w.Dungeons, "a97a0c344cf59235875f90b0671824aae5be4c2214eee346e1e772d08e10cf71"},
		{"dungeon index", w.DungeonIndex, "2e6eeda5f03eef08b34c7bcf65d5b8ec29ee39b25cf6b22b459b0777922a57ce"},
		{"NPC moves", w.NPCMoves, "53004164be53054b86eb375700de0f5dd7da393df291f272cd0c9842c1b719cc"},
		{"NPC places", w.NPCPlaces, "fb68caa0f400aee53c46eb3f99c32cba7da96ea15b843d1620c9bfc923e999fa"},
		{"episode returns", w.EpisodeReturns, "1e79e6876b58f0fe2485119f2fab3a0ed2f2a65f9a9669d42aef5f3725e923e5"},
		{"quests", q.Quests, "2eafa456b584acb4b8137e93d914183266aac8eb2e9c90bab6141eb89c70e711"},
		{"quest index", q.Index, "ff6cecfbdf1b0f296aeb629ce086e803da8ffb0570a9e864042f0a9a721ca84e"},
	} {
		raw, err := json.Marshal(row.value)
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != row.want {
			t.Fatalf("%s content changed: %s", row.name, got)
		}
	}
}

func TestNativeItemEquipmentCurrentArchiveFingerprint(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete item/equipment binding fingerprints")
	}
	c, err := PrepareCatalogs(CatalogInputs{Selection: "items,equipment", ArchivePath: path, ArchiveChecksum: "8b2a9f83247e000a28acd5134b616da725f46def39980b5373030e8cbc5d0934", DerivedCacheDir: "-"}, CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Equipment.Close()
	raw, err := json.Marshal(c.Items.Items)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != "a8c866c2e8c396ec86ae2290619b65c36f4e4980a26447f263c682eae2e3a5e5" || len(c.Items.Items) != 599771 {
		t.Fatal("native item metadata changed", got, len(c.Items.Items))
	}
	if c.Equipment.RecordCount() != 424216 || c.Equipment.IndexSHA256 != "7ffc480eee357d1dbd507de2c75621e0fca3a209dac76e64254bc00bc0557b71" {
		t.Fatal("complete equipment LIST binding changed")
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
