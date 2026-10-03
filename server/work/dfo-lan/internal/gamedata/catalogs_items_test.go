package gamedata

import (
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"os"
	"runtime"
	"testing"
)

func TestPVFBoxesSourceOnlyImportsItsOwnItemDependency(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for independent box source preparation")
	}
	verify := false
	c, err := prepareCatalogsForTest(t, "characters,boxes", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "missing-characters.json", "", "", "", CatalogInputs{VerifyBaselines: verify, CharacterPolicyPath: "../../configs/pvf-character-policy.json", BoxPolicyPath: "../../configs/pvf-box-policy.json", IndexPath: "missing-items.json", BoxesPath: "missing-boxes.json"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Items == nil || c.Boxes == nil || c.Boxes.TableCount() != 2 {
		t.Fatal("implicit native item dependency missing")
	}
	if _, err := c.LoadBoxes("missing-boxes.json", c.Items.Source.Checksum); err != nil {
		t.Fatal(err)
	}
}

func TestPVFBoxesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native COS material binding parity")
	}
	c, err := prepareCatalogsForTest(t, "items,boxes", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../catalog/testdata/item-flow.json", BoxesPath: "../../configs/boxes.json", BoxPolicyPath: "../../configs/pvf-box-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.LoadBoxes("missing-boxes.json", c.Items.Source.Checksum)
	if err != nil || b.TableCount() != 2 || b.RewardCount() != 54 {
		t.Fatal("native boxes unavailable", err)
	}
	if b.Tables["590712474"].PointStacks[1].SectionReward[0].Template != 590722560 || b.Tables["590719043"].PointStacks[1].SectionReward[0].Template != 590719045 {
		t.Fatal("same-name COS material owners confused")
	}
	if _, err := c.LoadBoxes("missing", "foreign"); err == nil {
		t.Fatal("foreign save identity accepted")
	}
	t.Logf("2 exact native material owners, 54 rewards, %d raw hashes; COS hashes %s / %s", len(b.Sources), b.Sources["live/else/univ/2024/0514_radianttreasurebox/radianttreasurebox.cos"], b.Sources["live/else/univ/2025/0318_newrandombox/radianttreasurebox.cos"])
}

func TestPVFCashShopLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete cashshop parity")
	}
	c, err := prepareCatalogsForTest(t, "cashshop", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{CashshopRelease: true})
	if err != nil {
		t.Fatal(err)
	}
	p, err := c.LoadCashShop(c.CashShop.Config.Source.Checksum, true)
	if err != nil || p.EnabledCount() == 0 {
		t.Fatal("prepared shop not reused", err)
	}
	if _, err := c.LoadCashShop("foreign", true); err == nil {
		t.Fatal("foreign save source accepted")
	}
	if _, err := c.LoadCashShop(p.Config.Source.Checksum, false); err == nil {
		t.Fatal("release behavior silently changed")
	}
	t.Logf("complete native rows=%d products=%d policies=%d priceSHA=%s indexSHA=%s equipmentSHA=%s", len(p.Config.Entries), p.EnabledCount(), len(p.Config.Policies), p.Config.PriceScriptHash, p.Config.IndexHash, p.Config.EquipmentIndexHash)
}

const pvfNextDomains = "world,quests,progression,items,equipment,periods,skins,journal,create-cost,skills,prices,materials,boosters,tutorial"

func TestPVFCommerceLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for the read-only complete source comparison")
	}
	c, err := prepareCatalogsForTest(t, pvfNextDomains, path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "retired-quests.json", "../../configs/progression.next25.json", "retired-world.json", CatalogInputs{
		IndexPath: "../catalog/testdata/item-flow.json", LearningPath: "../../configs/skills.next27.json", FullPrefix: "../../configs/equipment-full", JournalPath: "../../configs/equipment-journal.generated.json", CreateCostPath: "../../configs/equipment-create-cost.generated.json", TutorialPath: "../../configs/tutorial-routes.current35.json"})
	if err != nil {
		t.Fatal(err)
	}
	checksum := c.Items.Source.Checksum
	defer c.Equipment.Close()
	c.CollectImportMemory()
	if learning, err := c.LoadLearning("missing-after-preparation", checksum); err != nil || len(learning.Rows) == 0 {
		t.Fatal("learning not reused", err)
	}
	if prices, err := c.LoadShopPrices("missing-after-preparation", checksum); err != nil || len(prices.Items) == 0 {
		t.Fatal("prices not reused", err)
	}
	if _, err := c.LoadShopPrices("", "wrong"); err == nil {
		t.Fatal("wrong prepared source accepted")
	}
	materials, err := c.LoadItemMaterials("missing-after-preparation")
	if err != nil {
		t.Fatal(err)
	}
	if costs, ok := materials.Materials(3242); !ok || len(costs) != 1 || costs[0].Template != 3037 || costs[0].Count != 1000 {
		t.Fatal("material runtime index", costs)
	}
	if boosters, err := c.LoadBooster("missing-after-preparation", "missing-after-preparation"); err != nil || len(boosters.Definitions) == 0 {
		t.Fatal("booster definitions not reused", err)
	}
	if routes, err := c.LoadTutorialRoutes("missing-after-preparation", checksum); err != nil || len(routes.Flows) == 0 {
		t.Fatal("tutorial not reused", err)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	t.Logf("tutorial_flows=%d retained_heap_bytes=%d", len(c.Tutorial.Flows), memory.HeapAlloc)
	t.Logf("skills=%d prices=%d materials=%d boosters=%d", len(c.Learning.Rows), len(c.Prices.Items), len(c.Materials.Items), len(c.Boosters))
}

func TestPVFSourceOnlyLocalArchiveDoesNotReadSelectedJSON(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for the source-only startup check")
	}
	verify := false
	c, err := prepareCatalogsForTest(t, pvfNextDomains, path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "missing-quests.json", "missing-progression.json", "missing-world.json", CatalogInputs{
		VerifyBaselines: verify, IndexPath: "missing/items.index.json", FullPrefix: "missing/equipment-full", JournalPath: "missing-journal.json", CreateCostPath: "missing-create-cost.json", LearningPath: "missing-skills.json", MaterialsPath: "missing-materials.json", TutorialPath: "missing-tutorial.json"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Equipment.Close()
	c.CollectImportMemory()
	if len(c.Learning.Rows) != 3224 || len(c.Prices.Items) != 599682 || len(c.Materials.Items) != 14211 || len(c.Boosters) != 42504 || len(c.Tutorial.Flows) == 0 {
		t.Fatal("incomplete direct projection")
	}
	if _, err := c.Equipment.Definition(101001153); err != nil {
		t.Fatal(err)
	}
	t.Logf("all 14 domains prepared with nonexistent selected JSON paths; player source anchor unchanged")
}

func TestPVFDropLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for full loot and equipment selection parity")
	}
	c, err := prepareCatalogsForTest(t, "loot,equipment-selection", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../catalog/testdata/item-flow.json", DropPolicyPath: "../../configs/pvf-drop-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Loot.Items) != 1022 || len(c.Selection.Rows) != 3174 {
		t.Fatal("existing runtime pool changed", len(c.Loot.Items), len(c.Selection.Rows))
	}
	if _, err := c.LoadLoot("missing.json"); err != nil {
		t.Fatal(err)
	}
	a, err := c.LoadEquipmentSelection("missing.json", c.Items.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.LoadEquipmentSelection("missing.json", c.Items.Source.Checksum)
	if err != nil || a == b {
		t.Fatal("shared mutable wrapper", err)
	}
	t.Logf("loot and equipment full effective parity; drop pool=%d", len(a.DropPool()))
}

func TestPVFEquipmentSelectionLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip()
	}
	c, err := prepareCatalogsForTest(t, "equipment-selection", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../catalog/testdata/item-flow.json", DropPolicyPath: "../../configs/pvf-drop-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(len(c.Selection.Rows), len(c.Selection.DropPool()))
}

func TestPVFEnhancementsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for all six enhancement families")
	}
	c, err := prepareCatalogsForTest(t, "enhancements", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../catalog/testdata/item-flow.json", EnhancementPolicyPath: "../../configs/pvf-enhancement-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	if err = c.LoadEnhancements("missing-selected-JSON-directory"); err != nil {
		t.Fatal(err)
	}
	if card, ok := inventory.EnchantCardForBead(2600294); !ok || card != 3600 {
		t.Fatal("bead runtime index", card)
	}
	if !inventory.IsPureGrimoire(1286) {
		t.Fatal("server pure grimoire policy lost")
	}
	if count, ok := inventory.GoldMaterialCount(0); !ok || count != 10 {
		t.Fatal("reinforcement material", count)
	}
	if count, ok := inventory.AmplifyMaterialCount(9); !ok || count != 10 {
		t.Fatal("amplify material", count)
	}
	rate, ok := inventory.GoldSuccessPercent(10)
	if !ok || rate != 25 || inventory.AmplifySuccessPercent(4) != 70 {
		t.Fatal("server success policy changed")
	}
	t.Logf("six enhancement families passed complete effective parity and runtime activation")
}

func TestPVFEnhancementSourceOnlyLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for combined source-only startup")
	}
	verify := false
	c, err := prepareCatalogsForTest(t, pvfNextDomains+",enhancements", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "missing-quests.json", "missing-progression.json", "missing-world.json", CatalogInputs{
		VerifyBaselines: verify, IndexPath: "missing/items.index.json", FullPrefix: "missing/equipment-full", JournalPath: "missing-journal.json", CreateCostPath: "missing-create-cost.json", LearningPath: "missing-skills.json", MaterialsPath: "missing-materials.json", TutorialPath: "missing-tutorial.json", EnhancementPolicyPath: "../../configs/pvf-enhancement-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Equipment.Close()
	c.CollectImportMemory()
	if err := c.LoadEnhancements("missing-selected-JSON-directory"); err != nil {
		t.Fatal(err)
	}
	if len(c.Enhancements.ReinforcementTickets) != 1196 || len(c.Enhancements.AmplifyTickets) != 1629 || len(c.Enhancements.Grimoires.Grimoires) != 433 || len(c.Enhancements.Enchant.Beads) != 4846 || len(c.Enhancements.Gold.Levels) != 255 || len(c.Enhancements.Amplify.Levels) != 255 {
		t.Fatal("incomplete enhancements")
	}
	for _, test := range []struct {
		level  int
		weapon bool
		want   uint32
	}{{0, false, 147750}, {0, true, 177300}, {15, true, 8155800}} {
		got, err := inventory.GoldCost(115, 4, test.level, test.weapon)
		if err != nil || got != test.want {
			t.Fatal("reinforcement live cost anchor", got, err)
		}
	}
	if gold, ok := inventory.AmplifyGold(9); !ok || gold != 50000 {
		t.Fatal("amplify cost", gold)
	}
	if _, err := c.Equipment.Definition(101001153); err != nil {
		t.Fatal(err)
	}
	t.Log("15 selectors / 20 source families prepared with nonexistent selected JSON paths; character anchor and enhancement policy retained")
}

func TestPVFEquipmentRulesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for four equipment/vault rule families")
	}
	c, err := prepareCatalogsForTest(t, "random-options,shields,oath-grades,vault", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../catalog/testdata/item-flow.json", WearRulesPath: "../../configs/equipment-wear.current35.json", VaultPolicyPath: "../../configs/pvf-vault-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	verifyEquipmentRuleReuse(t, c)
}

func TestPVFEquipmentRulesSourceOnlyLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for direct preparation without JSON baselines")
	}
	verify := false
	c, err := prepareCatalogsForTest(t, "random-options,shields,oath-grades,vault", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{VerifyBaselines: verify, IndexPath: "missing/items.json", RandomOptionPath: "missing-options.json", ShieldPath: "missing-shields.json", OathPath: "missing-oath.json", VaultPath: "missing-vault.json", WearRulesPath: "../../configs/equipment-wear.current35.json", VaultPolicyPath: "../../configs/pvf-vault-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	verifyEquipmentRuleReuse(t, c)
}

func verifyEquipmentRuleReuse(t *testing.T, c *Catalogs) {
	t.Helper()
	source := c.Items.Source.Checksum
	if x, err := c.LoadRandomOptions("missing.json", source); err != nil || x.GroupCount() == 0 {
		t.Fatal("random options not reused", err)
	}
	if _, err := c.LoadRandomOptions("missing.json", "wrong"); err == nil {
		t.Fatal("wrong random option source accepted")
	}
	shields, err := c.LoadShields("missing.json", source)
	if err != nil || len(shields.Rows) != 25 {
		t.Fatal("shields not reused", err)
	}
	for _, row := range shields.Rows {
		if row.Condition == "quest" {
			if ok, _ := shields.Allowed(row.Item, 115); ok {
				t.Fatal("unverified quest shield enabled")
			}
		}
	}
	if oath, err := c.LoadOathGrades("missing.json"); err != nil || oath.Len() == 0 {
		t.Fatal("oath table not reused", err)
	}
	vault, err := c.LoadVaultRules("missing.json")
	if err != nil {
		t.Fatal(err)
	}
	if vault.SourceSHA256 != "fda6c33f2a155124a4262f3e7cd6f7a18d4e2191e5f147aec5754716afe5aa7c" || vault.InitialSlots != 8 || vault.InitialSecondarySlots != 24 || vault.Account.RequiredLevel != 60 || len(vault.Account.Upgrades) != 40 {
		t.Fatal("vault capacity/save or source table changed")
	}
	if limit, err := vault.Account.GoldLimit(64); err != nil || limit != 500000000 {
		t.Fatal(limit, err)
	}
	primer, oath := c.Oath.Grades([]inventory.BagEquipment{})
	if primer != 40 || oath != 40 {
		t.Fatal("diagnostic grades default changed")
	}
	t.Logf("four source projections and runtime indexes ready; groups=%d shields=%d oath=%d vault=%d", c.RandomOptions.GroupCount(), len(c.Shields.Rows), c.Oath.Len(), len(vault.Account.Upgrades))
}

func TestPVFFameLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete fame rule parity")
	}
	c, err := prepareCatalogsForTest(t, "fame", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{IndexPath: "../catalog/testdata/item-flow.json"})
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.InstallFameRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := character.CurrentFameRules()
	if err != nil {
		t.Fatal(err)
	}
	if err := auditPVFCatalog(c.FameRules, current); err != nil {
		t.Fatal(err)
	}
	if len(current.Tables) != 9 || len(current.Items) != 8100 || len(current.Sets) != 13 || len(current.ItemPoints) != 1054 || len(current.Sources) != 8411 {
		t.Fatal("fame source scope changed")
	}
	t.Log("native fame tables", len(current.Tables), "items", len(current.Items), "sets", len(current.Sets), "source hashes", len(current.Sources))
}

func TestPVFItemShopsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete SHP/runtime routing audit")
	}
	c, err := prepareCatalogsForTest(t, "items,item-shops", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", CatalogInputs{ItemShopPath: "../../configs/itemshop-candidate.json", ItemShopPolicyPath: "../../configs/pvf-item-shop-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	s, err := c.LoadItemShops("missing-item-shops.json", c.Items.Source.Checksum)
	if err != nil || len(s.Shops) != 527 {
		t.Fatal("native shop lookup unavailable", err)
	}
	if _, err := c.LoadItemShops("missing", "foreign"); err == nil {
		t.Fatal("foreign save source accepted")
	}
	if s.Shops["100000375"].Path != "itemshop/100000375_global_6th_seria.shp" || s.Shops["100001019"].Npc != 100003035 {
		t.Fatal("compatibility route changed")
	}
	count := 0
	for _, shop := range s.Shops {
		count += len(shop.Offers)
	}
	t.Logf("%d shops / %d source offers and complete first-payable/amount/limit lookup match", len(s.Shops), count)
}
