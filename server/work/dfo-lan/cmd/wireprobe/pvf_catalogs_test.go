package main

import (
	"dfolan/internal/catalog"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestPVFCatalogCandidateSelectionPreservesDefaultAndBlocksUnverifiedDomains(t *testing.T) {
	result, err := preparePVFCoreCatalogs("", "missing", "wrong", "missing", "", "", "")
	if err != nil || result.quests != nil || result.progression != nil || result.world != nil {
		t.Fatalf("default changed: %+v %v", result, err)
	}
	for _, selection := range []string{"characterz", "shops", "all", "quests,quests", "quests,"} {
		if _, err := parsePVFCatalogSelection(selection); err == nil {
			t.Fatalf("unverified selection accepted: %s", selection)
		}
	}
	selected, err := parsePVFCatalogSelection("quests, progression, world")
	if err != nil || !selected["quests"] || !selected["progression"] || !selected["world"] {
		t.Fatal(selected, err)
	}
}

// Explicit local integration check; ordinary tests do not load a multi-GB
// archive. This invokes the exact startup gate without opening any database.
func TestPVFCoreCatalogsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE and DFO_PVF_CORE_TEST_SHA256 for the read-only local check")
	}
	c, err := preparePVFCoreCatalogs("quests,progression,world", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "../../configs/quests.generated.json", "../../configs/progression.next25.json", "../../configs/world.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	collectPVFImportMemory(c)
	quests, err := c.loadQuests("missing-after-preparation")
	if err != nil || len(quests.Quests) == 0 {
		t.Fatal("prepared quests were not reused", err)
	}
	progression, err := c.loadProgression("missing-after-preparation")
	if err != nil || len(progression.Thresholds) == 0 {
		t.Fatal("prepared progression was not reused", err)
	}
	world, err := c.loadWorld("missing-after-preparation")
	if err != nil || len(world.Areas) == 0 || len(world.NPCMoves) == 0 || len(world.EpisodeReturns) == 0 {
		t.Fatal("prepared world dependencies were not reused", err)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	t.Logf("quests=%d thresholds=%d world_areas=%d NPC_moves=%d episode_returns=%d retained_heap_bytes=%d source=%s", len(quests.Quests), len(progression.Thresholds), len(world.Areas), len(world.NPCMoves), len(world.EpisodeReturns), memory.HeapAlloc, quests.Source.Checksum)
}

func TestPVFCatalogGateRefusesRewardChangesAndDoesNotFallback(t *testing.T) {
	legacy := catalog.QuestCatalog{Quests: map[uint32]catalog.QuestDefinition{7: {ID: 7, MinimumLevel: 10}}}
	direct := catalog.QuestCatalog{Quests: map[uint32]catalog.QuestDefinition{7: {ID: 7, MinimumLevel: 20}}}
	if err := verifyPVFCatalog(legacy, direct); err == nil {
		t.Fatal("changed source semantics accepted")
	}
	if err := verifyPVFCatalog(legacy, legacy); err != nil {
		t.Fatal(err)
	}
	if _, err := preparePVFCoreCatalogs("quests", "missing", "wrong", "missing", "", "", ""); err == nil {
		t.Fatal("missing baseline accepted")
	}
	if _, err := preparePVFCoreCatalogs("world", "missing", "wrong", "missing", "", "", ""); err == nil {
		t.Fatal("missing world baseline accepted")
	}
}

func TestPVFWorldRefusesJSONDiagnosticOverrideBeforeArchiveOrStorage(t *testing.T) {
	t.Setenv("DFO_NPC_PRESENCE_WORLD", "missing-shadow.json")
	_, err := preparePVFCoreCatalogs("world", "missing", "wrong", "missing", "", "", "baseline.json")
	if err == nil || !strings.Contains(err.Error(), "DFO_NPC_PRESENCE_WORLD") {
		t.Fatal(err)
	}
}

func TestPVFItemCatalogsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for prepared item catalogs")
	}
	c, err := preparePVFCoreCatalogs("world,quests,progression,items,equipment,periods,skins,journal,create-cost", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "../../configs/quests.generated.json", "../../configs/progression.next25.json", "../../configs/world.generated.json", pvfItemInputs{
		indexPath: "../../configs/items.index.json", fullPrefix: "../../configs/equipment-full",
		journalPath: "../../configs/equipment-journal.generated.json", createCostPath: "../../configs/equipment-create-cost.generated.json"})
	if err != nil {
		t.Fatal(err)
	}
	collectPVFImportMemory(c)
	index, err := c.loadBooster("", "missing-after-preparation.json")
	if err != nil || len(index.Items) != 599771 {
		t.Fatal("index not reused", err)
	}
	full, err := c.openFullEquipment("missing-after-preparation", c.items.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	if _, err = full.Definition(101001153); err != nil {
		t.Fatal("lazy definition unavailable after preparation", err)
	}
	periods, err := c.loadItemPeriods("missing", c.items.Source.Checksum)
	if err != nil || len(periods) == 0 {
		t.Fatal("periods not reused", err)
	}
	skins, err := c.loadSkinStorage("missing", c.items.Source.Checksum)
	if err != nil || len(skins) == 0 {
		t.Fatal("skins not reused", err)
	}
	journal, err := c.loadEquipmentJournal("missing", c.items.Source.Checksum)
	if err != nil || len(journal.Categories) == 0 {
		t.Fatal("journal not reused", err)
	}
	cost, err := c.loadEquipmentCreateCost("missing", c.items.Source.Checksum)
	if err != nil || len(cost.Groups) == 0 {
		t.Fatal("costs not reused", err)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	t.Logf("items=%d equipment=%d periods=%d skins=%d costs=%d retained_heap_bytes=%d", len(index.Items), full.RecordCount(), len(periods), len(skins), len(cost.Groups), memory.HeapAlloc)
}
