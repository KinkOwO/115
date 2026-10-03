package gamedata

import (
	"dfolan/internal/adventure"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/quest"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// prepareCatalogsForTest keeps the local-archive fixtures compact while still
// exercising the public preparation facade and its adapter hooks.
func prepareCatalogsForTest(t *testing.T, selection, archive, checksum, characters, quests, progression, world string, itemInputs ...CatalogInputs) (*Catalogs, error) {
	t.Helper()
	// Restore the runtime source identity to the checksum actually used by this
	// preparation, never the stale historical 7ef2 constant: writing that back
	// would poison the source gate for the next test.
	usedIdentity := ""
	t.Cleanup(func() {
		catalog.SetOdysseySource(usedIdentity)
		inventory.SetClearCubeSource(usedIdentity)
		quest.SetImageCommunicationSource(usedIdentity)
	})
	inputs := CatalogInputs{
		Selection: selection, ArchivePath: archive, ArchiveChecksum: checksum,
		CharacterPath: characters, QuestPath: quests, ProgressionPath: progression, WorldPath: world,
	}
	if len(itemInputs) != 0 {
		provided := itemInputs[0]
		provided.Selection, provided.ArchivePath, provided.ArchiveChecksum = selection, archive, checksum
		provided.CharacterPath, provided.QuestPath, provided.ProgressionPath, provided.WorldPath = characters, quests, progression, world
		inputs = provided
	}
	// Native PVF domains other than characters ignore the JSON character anchor:
	// the archive identity is already enforced by Open(ExpectedChecksum). Drop the
	// anchor path so the historical 7ef2 baseline cannot fail the source gate,
	// while preserving the IndexPath inference production derives from it.
	if selected, err := parsePVFCatalogSelection(selection); err == nil && !selected["characters"] {
		if inputs.IndexPath == "" && strings.TrimSpace(inputs.CharacterPath) != "" {
			inputs.IndexPath = filepath.Join(filepath.Dir(inputs.CharacterPath), "items.index.json")
		}
		inputs.CharacterPath = ""
	}
	// Default all domains onto the shared derived-projection cache so repeated
	// imports of the same archive inside this package reuse one projection
	// instead of recomputing it per test. Callers that pass DerivedCacheDir keep
	// their explicit directory (for example the cold/hot cache regression).
	if inputs.DerivedCacheDir == "" {
		inputs.DerivedCacheDir = testPVFCacheDir()
	}
	result, err := PrepareCatalogs(inputs, testCatalogAdapters())
	if result != nil {
		usedIdentity = result.SourceChecksum
	}
	return result, err
}

// testModuleRoot locates the module root by walking up from the test working
// directory until go.mod is found, so relative cache paths resolve to one
// shared directory regardless of which package is running.
var testModuleRoot = sync.OnceValues(func() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
})

// testPVFCacheDir resolves the shared derived-projection cache, mirroring the
// production DFO_PVF_CACHE_DIR behavior: default runtime/pvf-cache, "-"
// disables caching, and relative paths resolve against the module root.
func testPVFCacheDir() string {
	dir := strings.TrimSpace(os.Getenv("DFO_PVF_CACHE_DIR"))
	if dir == "" {
		dir = "runtime/pvf-cache"
	}
	if dir == "-" || filepath.IsAbs(dir) {
		return dir
	}
	root, err := testModuleRoot()
	if err != nil {
		return dir
	}
	return filepath.Join(root, dir)
}

func testCatalogAdapters() CatalogAdapters {
	return CatalogAdapters{
		ValidatedSource: func(checksum string) {
			catalog.SetOdysseySource(checksum)
			inventory.SetClearCubeSource(checksum)
			quest.SetImageCommunicationSource(checksum)
		},
		ValidateLottery: func(tables catalog.LotteryTables, index catalog.ItemIndex, _ string, _ bool) error {
			if tables.Items.SourcePVFSHA256 == "" || tables.Equipment.SourcePVFSHA256 == "" || index.Source.Checksum != tables.Items.SourcePVFSHA256 {
				return fmt.Errorf("lottery source identity is incomplete")
			}
			return nil
		},
		RewardBoxes: func(definitions map[uint32]catalog.BoosterDefinition, items map[uint32]catalog.ItemIndexEntry) loot.RewardBoxSource {
			return testRewardBoxSource{definitions: definitions, items: items}
		},
	}
}

type testRewardBoxSource struct {
	definitions map[uint32]catalog.BoosterDefinition
	items       map[uint32]catalog.ItemIndexEntry
}

func (s testRewardBoxSource) RewardBox(template uint32) (loot.RewardBox, bool) {
	definition, ok := s.definitions[template]
	if !ok || template == 10415192 || len(definition.Pools) == 0 {
		return loot.RewardBox{}, false
	}
	box := loot.RewardBox{}
	for _, pool := range definition.Pools {
		converted := loot.RewardBoxPool{Draws: pool.DrawCount}
		for _, candidate := range pool.Candidates {
			converted.Candidates = append(converted.Candidates, loot.RewardBoxCandidate{Template: candidate.Template, Weight: candidate.Weight, Count: candidate.Count})
		}
		box.Pools = append(box.Pools, converted)
	}
	return box, true
}

func (s testRewardBoxSource) Item(template uint32) bool {
	item, ok := s.items[template]
	return ok && (item.Kind == "equipment" || item.Kind == "stackable")
}

func (s testRewardBoxSource) Container(template uint32) bool {
	if template == 10415192 {
		return false
	}
	if _, ok := s.definitions[template]; ok {
		return true
	}
	item, ok := s.items[template]
	return ok && item.StackableType != "[booster selection]" && strings.Contains(strings.ToLower(item.StackableType), "booster")
}

func TestPVFCatalogCandidateSelectionPreservesDefaultAndBlocksUnverifiedDomains(t *testing.T) {
	result, err := prepareCatalogsForTest(t, "", "missing", "wrong", "missing", "", "", "")
	if err != nil || result.Quests != nil || result.Progression != nil || result.World != nil {
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
	c, err := prepareCatalogsForTest(t, "quests,progression,world", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "../../configs/quests.generated.json", "../../configs/progression.next25.json", "../../configs/world.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	c.CollectImportMemory()
	quests, err := c.LoadQuests("missing-after-preparation")
	if err != nil || len(quests.Quests) == 0 {
		t.Fatal("prepared quests were not reused", err)
	}
	progression, err := c.LoadProgression("missing-after-preparation")
	if err != nil || len(progression.Thresholds) == 0 {
		t.Fatal("prepared progression was not reused", err)
	}
	world, err := c.LoadWorld("missing-after-preparation")
	if err != nil || len(world.Areas) == 0 || len(world.NPCMoves) == 0 || len(world.EpisodeReturns) == 0 {
		t.Fatal("prepared world dependencies were not reused", err)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	t.Logf("quests=%d thresholds=%d world_areas=%d NPC_moves=%d episode_returns=%d retained_heap_bytes=%d source=%s", len(quests.Quests), len(progression.Thresholds), len(world.Areas), len(world.NPCMoves), len(world.EpisodeReturns), memory.HeapAlloc, quests.Source.Checksum)
}

func TestCatalogBaselineVerificationDefaultsOff(t *testing.T) {
	if (CatalogInputs{}).checksBaselines() {
		t.Fatal("zero-value catalog inputs must not read historical JSON baselines")
	}
	if !(CatalogInputs{VerifyBaselines: true}).checksBaselines() {
		t.Fatal("explicit baseline verification was ignored")
	}
}

func TestSelectedNativeReadFailureNeverFallsBackToJSON(t *testing.T) {
	c := &Catalogs{selected: map[string]bool{"quests": true}}
	if _, err := c.LoadQuests("../../configs/quests.generated.json"); err == nil {
		t.Fatal("selected but unprepared PVF quests silently consumed JSON")
	}
}

func TestPreparedNativeEmptyResultsDoNotFallBackToJSON(t *testing.T) {
	ready := map[string]bool{"periods": true, "skins": true, "boosters": true, "script-warps": true}
	c := &Catalogs{selected: ready, prepared: ready, Items: &catalog.ItemIndex{}}
	periods, err := c.LoadItemPeriods("missing-periods.json", "")
	if err != nil || len(periods) != 0 {
		t.Fatalf("valid empty periods changed source: periods=%v err=%v", periods, err)
	}
	skins, err := c.LoadSkinStorage("missing-skins.json", "")
	if err != nil || len(skins) != 0 {
		t.Fatalf("valid empty skins changed source: skins=%v err=%v", skins, err)
	}
	boosters, err := c.LoadBooster("missing-boosters.json", "missing-index.json")
	if err != nil || boosters == nil || len(boosters.Definitions) != 0 {
		t.Fatalf("valid empty boosters changed source: boosters=%v err=%v", boosters, err)
	}
	if _, err := c.InstallScriptWarps(); err == nil || !strings.Contains(err.Error(), "empty native script warp routes") {
		t.Fatalf("empty native routes bypassed domain validation: %v", err)
	}
}

func TestCatalogPreparationCallbacksWaitForValidatedSource(t *testing.T) {
	called := false
	_, err := PrepareCatalogs(CatalogInputs{
		Selection: "characters", ArchivePath: "missing-inner.pvf", CharacterPath: "missing-characters.json",
		CharacterPolicyPath: "missing-character-policy.json",
	}, CatalogAdapters{ValidatedSource: func(string) { called = true }})
	if err == nil || called {
		t.Fatalf("source callback ran before source validation: err=%v called=%v", err, called)
	}
}

func TestPVFWorldRefusesJSONDiagnosticOverrideBeforeArchiveOrStorage(t *testing.T) {
	t.Setenv("DFO_NPC_PRESENCE_WORLD", "missing-shadow.json")
	_, err := prepareCatalogsForTest(t, "world", "missing", "wrong", "missing", "", "", "baseline.json")
	if err == nil || !strings.Contains(err.Error(), "DFO_NPC_PRESENCE_WORLD") {
		t.Fatal(err)
	}
}

func TestPVFItemCatalogsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for prepared item catalogs")
	}
	c, err := prepareCatalogsForTest(t, "world,quests,progression,items,equipment,periods,skins,journal,create-cost", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "../../configs/quests.generated.json", "../../configs/progression.next25.json", "../../configs/world.generated.json", CatalogInputs{
		IndexPath: "../../configs/items.index.json", FullPrefix: "../../configs/equipment-full",
		JournalPath: "../../configs/equipment-journal.generated.json", CreateCostPath: "../../configs/equipment-create-cost.generated.json"})
	if err != nil {
		t.Fatal(err)
	}
	c.CollectImportMemory()
	if len(c.Items.Items) != 599771 {
		t.Fatal("index not reused", err)
	}
	full, err := c.OpenFullEquipment("missing-after-preparation", c.Items.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	if _, err = full.Definition(101001153); err != nil {
		t.Fatal("lazy definition unavailable after preparation", err)
	}
	periods, err := c.LoadItemPeriods("missing", c.Items.Source.Checksum)
	if err != nil || len(periods) == 0 {
		t.Fatal("periods not reused", err)
	}
	skins, err := c.LoadSkinStorage("missing", c.Items.Source.Checksum)
	if err != nil || len(skins) == 0 {
		t.Fatal("skins not reused", err)
	}
	journal, err := c.LoadEquipmentJournal("missing", c.Items.Source.Checksum)
	if err != nil || len(journal.Categories) == 0 {
		t.Fatal("journal not reused", err)
	}
	cost, err := c.LoadEquipmentCreateCost("missing", c.Items.Source.Checksum)
	if err != nil || len(cost.Groups) == 0 {
		t.Fatal("costs not reused", err)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	t.Logf("items=%d equipment=%d periods=%d skins=%d costs=%d retained_heap_bytes=%d", len(c.Items.Items), full.RecordCount(), len(periods), len(skins), len(cost.Groups), memory.HeapAlloc)
}

func TestPVFCharactersLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete character runtime parity")
	}
	c, err := prepareCatalogsForTest(t, "characters", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{CharacterPolicyPath: "../../configs/pvf-character-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := c.LoadCharacters("missing-characters.json")
	if err != nil || len(direct.Professions) != 17 {
		t.Fatal("native professions missing", err)
	}
	if len(c.SourceCharacters.Professions[0].Growth) != 6 || len(direct.Professions[0].Growth) != 0 || direct.Source.Checksum != c.SourceCharacters.Source.Checksum {
		t.Fatal("raw source view or save identity changed")
	}
	t.Log("17 complete runtime professions match; 341 raw differences resolved through shortcut/command policy and duplicate source-view projection")
}

func TestPVFMigrationSourceOnlyLocalArchive(t *testing.T) {
	verifyPVFMigrationSourceOnly(t, "")
}

func verifyPVFMigrationSourceOnly(t *testing.T, cacheDir string) {
	t.Helper()
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for combined direct startup without export JSON")
	}
	verify := false
	c, err := prepareCatalogsForTest(t, SupportedDomains, path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "missing-characters.json", "missing-quests.json", "missing-progression.json", "missing-world.json", CatalogInputs{
		DerivedCacheDir: cacheDir,
		ItemShopPath:    "missing-item-shops.json", ItemShopPolicyPath: "../../configs/pvf-item-shop-policy.json", BoxesPath: "missing-boxes.json", BoxPolicyPath: "../../configs/pvf-box-policy.json", CashshopRelease: true, CharacterPolicyPath: "../../configs/pvf-character-policy.json", LayerRevisitPolicyPath: "../../configs/pvf-layer-revisit-policy.json", ScriptWarpPolicyPath: "../../configs/pvf-script-warp-policy.json", LotteryPolicyPath: "missing-lottery-policy.json", VerifyBaselines: verify, MinePath: "missing-mine.json", BlackPurgatoryPath: "missing-black-purgatory.json", ClearCubePath: "missing-cube.json", OdysseyGrowthPath: "missing-growth.json", OdysseyChapterPath: "missing-chapters.json", OdysseyDropPath: "missing-odyssey-drop.json", OdysseyCurrencyPath: "missing-coins.json", OdysseyWeaponPath: "missing-weapons.json", ApocalypsePath: "missing-apocalypse.json", AttunementPath: "missing-attunement.json", ContentPolicyPath: "../../configs/pvf-mine-policy.json", IndexPath: "missing/items.index.json", FullPrefix: "missing/equipment-full", JournalPath: "missing-journal.json", CreateCostPath: "missing-create-cost.json", LearningPath: "missing-skills.json", MaterialsPath: "missing-materials.json", TutorialPath: "missing-tutorial.json", EnhancementPolicyPath: "../../configs/pvf-enhancement-policy.json",
		RandomOptionPath: "missing-options.json", ShieldPath: "missing-shields.json", OathPath: "missing-oath.json", VaultPath: "missing-vault.json", WearRulesPath: "../../configs/equipment-wear.current35.json", VaultPolicyPath: "../../configs/pvf-vault-policy.json", LootPath: "missing-loot.json", EquipmentPath: "missing-equipment.json", QuestEquipmentPath: "missing-quest-equipment.json", DropPolicyPath: "../../configs/pvf-drop-policy.json",
		TownPath: "missing-town.json", DungeonPath: "missing-dungeons.json", TrainingDungeonPath: "missing-training.json", TutorialDungeonPath: "missing-tutorial-dungeons.json", ScenePolicyPath: "../../configs/pvf-scene-policy.json",
	})
	if err != nil {
		t.Fatal(err)
	}
	if shop, err := c.LoadCashShop(c.Items.Source.Checksum, true); err != nil || shop.EnabledCount() == 0 {
		t.Fatal("native cashshop unavailable", err)
	}
	if boxes, err := c.LoadBoxes("missing-boxes.json", c.Items.Source.Checksum); err != nil || boxes.TableCount() != 2 || boxes.RewardCount() != 54 {
		t.Fatal("native COS boxes unavailable", err)
	}
	if shops, err := c.LoadItemShops("missing-item-shops.json", c.Items.Source.Checksum); err != nil || len(shops.Shops) != 527 {
		t.Fatal("native item shops unavailable", err)
	}
	defer c.Equipment.Close()
	defer c.Learning.Close()
	defer c.Quests.Close()
	defer c.Loot.CloseDetails()
	defer c.Dungeons.CloseMapSource()
	c.CollectImportMemory()
	verifyEquipmentRuleReuse(t, c)
	if len(c.Loot.Items) != 1022 || len(c.Selection.Rows) != 3174 || len(c.Selection.DropPool()) != 2794 {
		t.Fatal("existing loot scope changed")
	}
	if err := c.LoadEnhancements("missing-enhancements"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Equipment.Definition(101001153); err != nil {
		t.Fatal(err)
	}
	if !c.Town.Allows(1, 561, 234) || len(c.Dungeons.Dungeons) != 3200 || len(c.TrainingDungeons.Dungeons) != 4 || len(c.TutorialDungeons.Dungeons) != 15 {
		t.Fatal("scene selection changed")
	}
	if c.Grief == nil || c.Dazzlement == nil || c.HellMaps == nil || c.MazeRates == nil {
		t.Fatal("source overlays missing")
	}
	base, err := c.LoadDungeons("missing-dungeons.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.AttachTerminalScenes(&base, "missing-terminal.json"); err != nil {
		t.Fatal(err)
	}
	if err := c.AttachLayerRevisits(&base, "missing-layer-revisits.json"); err != nil {
		t.Fatal(err)
	}
	if len(base.LayerRevisits) != 1 || base.LayerRevisits[0].ResumeMap != 100004325 {
		t.Fatal("native layer revisit base missing")
	}
	if err := c.AttachTournamentMaps(&base, "missing-tournament.json"); err != nil {
		t.Fatal(err)
	}
	if len(base.TerminalScenes) != 7 || len(c.TournamentMaps.Maps) != 2 {
		t.Fatal("closing scene source scope changed")
	}

	boxes, err := c.LoadSelectionBoxes("missing-selection-boxes.json")
	if err != nil || len(boxes.Boxes) != 16749 || len(boxes.Fixed) != 2 || len(boxes.Unparsed) != 3 || len(boxes.Rejected) != 3 {
		t.Fatal("selection source scope changed", err)
	}

	restore, err := c.InstallAdventureRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	rules, err := adventure.Current()
	if err != nil || len(rules.Experience) != 60 || len(rules.Shops) != 3 {
		t.Fatal("native embedded adventure rules missing", err)
	}
	restoreRecommended, err := c.InstallRecommendedRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restoreRecommended()
	recommended, err := adventure.CurrentRecommendedRules()
	if err != nil || len(recommended.Ranges) == 0 {
		t.Fatal("native embedded recommended rules missing", err)
	}
	restoreSeason, err := c.InstallSeasonRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restoreSeason()
	season, err := adventure.CurrentSeason()
	if err != nil || len(season.Levels) != 120 || len(season.Capsules) != 40 {
		t.Fatal("native embedded season rules missing", err)
	}
	var progression character.ProgressionService
	if err := c.BindOdysseyRoutes(&progression); err != nil {
		t.Fatal(err)
	}
	if progression.JournalRoutes == nil || len(progression.JournalRoutes.Nodes) != 29 {
		t.Fatal("native embedded journal routes missing")
	}
	restoreBackgrounds, err := c.InstallRosterBackgrounds()
	if err != nil {
		t.Fatal(err)
	}
	defer restoreBackgrounds()
	backgrounds, err := character.CurrentRosterBackgroundTickets()
	if err != nil || len(backgrounds.Items) != 95 || len(backgrounds.Backgrounds) != 63 {
		t.Fatal("native background tickets or resources missing", err)
	}
	restoreFame, err := c.InstallFameRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restoreFame()
	fame, err := character.CurrentFameRules()
	if err != nil || len(fame.Tables) != 9 || len(fame.Items) != 8100 || len(fame.Sources) != 8411 {
		t.Fatal("native fame rules missing", err)
	}
	restoreWarps, err := c.InstallScriptWarps()
	if err != nil {
		t.Fatal(err)
	}
	defer restoreWarps()
	if len(c.ScriptWarps) != 12 {
		t.Fatal("native script warp scope missing")
	}
	characters, err := c.LoadCharacters("missing-characters.json")
	if err != nil || len(characters.Professions) != 17 || c.SourceCharacters == nil || len(c.SourceCharacters.Professions[0].Growth) != 6 {
		t.Fatal("native character source or runtime projection missing", err)
	}
	t.Log("54 selectors / 63 source families prepared with all selected export JSON paths absent")

}

func TestCatalogCheckRequiresPreparedSourceAndExplicitDomains(t *testing.T) {
	if _, err := (&Catalogs{}).CheckReport("items"); err == nil {
		t.Fatal("unprepared source accepted")
	}
	c := Catalogs{SourceChecksum: strings.Repeat("a", 64)}
	if _, err := c.CheckReport(""); err == nil {
		t.Fatal("empty selection accepted")
	}
	if _, err := c.CheckReport("unverified"); err == nil {
		t.Fatal("unknown source domain accepted")
	}
	got, err := c.CheckReport("items,characters")
	if err != nil || got["domain_count"] != 2 || got["storage_accessed"] != false || got["runtime_started"] != false {
		t.Fatal(got, err)
	}
}
