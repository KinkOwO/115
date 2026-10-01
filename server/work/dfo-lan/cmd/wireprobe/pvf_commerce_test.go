package main

import (
	"os"
	"runtime"
	"testing"
)

const pvfNextDomains = "world,quests,progression,items,equipment,periods,skins,journal,create-cost,skills,prices,materials,boosters,tutorial"

func TestPVFCommerceLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for the read-only complete source comparison")
	}
	c, err := preparePVFCoreCatalogs(pvfNextDomains, path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "../../configs/quests.generated.json", "../../configs/progression.next25.json", "../../configs/world.generated.json", pvfItemInputs{
		indexPath: "../../configs/items.index.json", learningPath: "../../configs/skills.next27.json", fullPrefix: "../../configs/equipment-full", journalPath: "../../configs/equipment-journal.generated.json", createCostPath: "../../configs/equipment-create-cost.generated.json", tutorialPath: "../../configs/tutorial-routes.current35.json"})
	if err != nil {
		t.Fatal(err)
	}
	checksum := c.items.Source.Checksum
	defer c.equipment.Close()
	collectPVFImportMemory(c)
	if learning, err := c.loadLearning("missing-after-preparation", checksum); err != nil || len(learning.Rows) == 0 {
		t.Fatal("learning not reused", err)
	}
	if prices, err := c.loadShopPrices("missing-after-preparation", checksum); err != nil || len(prices.Items) == 0 {
		t.Fatal("prices not reused", err)
	}
	if _, err := c.loadShopPrices("", "wrong"); err == nil {
		t.Fatal("wrong prepared source accepted")
	}
	materials, err := c.loadItemMaterials("missing-after-preparation")
	if err != nil {
		t.Fatal(err)
	}
	if costs, ok := materials.Materials(3242); !ok || len(costs) != 1 || costs[0].Template != 3037 || costs[0].Count != 1000 {
		t.Fatal("material runtime index", costs)
	}
	if boosters, err := c.loadBooster("missing-after-preparation", "missing-after-preparation"); err != nil || len(boosters.Definitions) == 0 {
		t.Fatal("booster definitions not reused", err)
	}
	if routes, err := c.loadTutorialRoutes("missing-after-preparation", checksum); err != nil || len(routes.Flows) == 0 {
		t.Fatal("tutorial not reused", err)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	t.Logf("tutorial_flows=%d retained_heap_bytes=%d", len(c.tutorial.Flows), memory.HeapAlloc)
	t.Logf("skills=%d prices=%d materials=%d boosters=%d", len(c.learning.Rows), len(c.prices.Items), len(c.materials.Items), len(c.boosters))
}

func TestPVFSourceOnlyLocalArchiveDoesNotReadSelectedJSON(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for the source-only startup check")
	}
	verify := false
	c, err := preparePVFCoreCatalogs(pvfNextDomains, path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "missing-quests.json", "missing-progression.json", "missing-world.json", pvfItemInputs{
		verifyBaselines: &verify, indexPath: "missing/items.index.json", fullPrefix: "missing/equipment-full", journalPath: "missing-journal.json", createCostPath: "missing-create-cost.json", learningPath: "missing-skills.json", pricesPath: "missing-prices.json", materialsPath: "missing-materials.json", boosterPath: "missing-boosters.json", tutorialPath: "missing-tutorial.json"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.equipment.Close()
	collectPVFImportMemory(c)
	if len(c.learning.Rows) != 3224 || len(c.prices.Items) != 599682 || len(c.materials.Items) != 14211 || len(c.boosters) != 42504 || len(c.tutorial.Flows) == 0 {
		t.Fatal("incomplete direct projection")
	}
	if _, err := c.equipment.Definition(101001153); err != nil {
		t.Fatal(err)
	}
	t.Logf("all 14 domains prepared with nonexistent selected JSON paths; player source anchor unchanged")
}
