package gamedata

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestCumulativeRestorationAllReleaseDomains(t *testing.T) {
	basePath, restoredPath := os.Getenv("US115_RESTORATION_BASE_PVF"), os.Getenv("US115_RESTORATION_PVF")
	if basePath == "" || restoredPath == "" {
		t.Skip("explicit frozen R2 and restored archives required")
	}
	type profile struct {
		Environment map[string]string `json:"environment"`
	}
	var p profile
	raw, err := os.ReadFile("../../configs/pvf-default.json")
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &p))
	restoredSelection := p.Environment["DFO_PVF_CATALOGS"]
	added := map[string]bool{"attendance-online": true, "attendance-daily": true, "membership": true}
	var oldDomains []string
	for _, d := range strings.Split(restoredSelection, ",") {
		if !added[d] {
			oldDomains = append(oldDomains, d)
		}
	}
	require.Len(t, oldDomains, 58)
	require.Len(t, strings.Split(restoredSelection, ","), 61)
	prepare := func(archive, selection string, restored bool) map[string]int {
		c, err := prepareCatalogsForTest(t, selection, archive, "", "missing-characters.json", "missing-quests.json", "missing-progression.json", "missing-world.json", CatalogInputs{
			DerivedCacheDir: "-",
			ItemShopPath:    "missing-item-shops.json", ItemShopPolicyPath: "../../configs/pvf-item-shop-policy.json", BoxesPath: "missing-boxes.json", BoxPolicyPath: "../../configs/pvf-box-policy.json", CashshopRelease: true, CharacterPolicyPath: "../../configs/pvf-character-policy.json", LayerRevisitPolicyPath: "../../configs/pvf-layer-revisit-policy.json", ScriptWarpPolicyPath: "../../configs/pvf-script-warp-policy.json", LotteryPolicyPath: "missing-lottery-policy.json", VerifyBaselines: false, MinePath: "missing-mine.json", BlackPurgatoryPath: "missing-black-purgatory.json", ClearCubePath: "missing-cube.json", OdysseyGrowthPath: "missing-growth.json", OdysseyChapterPath: "missing-chapters.json", OdysseyDropPath: "missing-odyssey-drop.json", OdysseyCurrencyPath: "missing-coins.json", OdysseyWeaponPath: "missing-weapons.json", ApocalypsePath: "missing-apocalypse.json", AttunementPath: "missing-attunement.json", ContentPolicyPath: "../../configs/pvf-mine-policy.json", IndexPath: "missing/items.index.json", FullPrefix: "missing/equipment-full", JournalPath: "missing-journal.json", CreateCostPath: "missing-create-cost.json", MaterialsPath: "missing-materials.json", TutorialPath: "missing-tutorial.json", EnhancementPolicyPath: "../../configs/pvf-enhancement-policy.json",
			RandomOptionPath: "missing-options.json", ShieldPath: "missing-shields.json", OathPath: "missing-oath.json", VaultPath: "missing-vault.json", WearRulesPath: "../../configs/equipment-wear.current35.json", VaultPolicyPath: "../../configs/pvf-vault-policy.json", LootPath: "missing-loot.json", EquipmentPath: "missing-equipment.json", QuestEquipmentPath: "missing-quest-equipment.json", DropPolicyPath: "../../configs/pvf-drop-policy.json",
			TownPath: "missing-town.json", DungeonPath: "missing-dungeons.json", TrainingDungeonPath: "missing-training.json", TutorialDungeonPath: "missing-tutorial-dungeons.json", ScenePolicyPath: "../../configs/pvf-scene-policy.json",
		})
		require.NoError(t, err)
		_, err = c.CheckReport(selection)
		require.NoError(t, err)
		defer c.Equipment.Close()
		defer c.Learning.Close()
		defer c.Quests.Close()
		defer c.Loot.CloseDetails()
		defer c.Dungeons.CloseMapSource()
		c.CollectImportMemory()
		require.NotNil(t, c.Contracts)
		require.NotNil(t, c.SymbolSockets)
		require.NotNil(t, c.EmblemInlay)
		shops, err := c.LoadItemShops("missing-item-shops.json", c.Items.Source.Checksum)
		require.NoError(t, err)
		cash, err := c.LoadCashShop(c.Items.Source.Checksum, true)
		require.NoError(t, err)
		boxes, err := c.LoadBoxes("missing-boxes.json", c.Items.Source.Checksum)
		require.NoError(t, err)
		scope := map[string]int{"shops": len(shops.Shops), "cash_offers": cash.EnabledCount(), "items": len(c.Items.Items), "boosters": len(c.Boosters), "boxes": boxes.TableCount(), "box_rewards": boxes.RewardCount(), "dungeons": len(c.Dungeons.Dungeons), "training": len(c.TrainingDungeons.Dungeons), "tutorial": len(c.TutorialDungeons.Dungeons), "loot": len(c.Loot.Items), "selection": len(c.Selection.Rows), "drop_pool": len(c.Selection.DropPool()), "skins": len(c.Skins), "periods": len(c.Periods), "script_warps": len(c.ScriptWarps), "login_days": len(c.AttendanceLogin.Days), "weekly_periods": len(c.AttendanceWeekly.Weekly)}
		if restored {
			require.NotEmpty(t, c.World.NPCPlaces[400001880])
			require.NotNil(t, c.AttendanceOnline)
			require.NotNil(t, c.AttendanceDaily)
			require.NotNil(t, c.Membership)
			require.Equal(t, c.SourceChecksum, c.AttendanceOnline.SourceChecksum)
			require.Equal(t, c.SourceChecksum, c.AttendanceDaily.SourceChecksum)
			require.Equal(t, c.SourceChecksum, c.Membership.SourceChecksum)
			require.Len(t, c.AttendanceDaily.Daily, 10)
			require.Len(t, c.Membership.Grades, 11)
		} else {
			require.Empty(t, c.World.NPCPlaces[400001880])
			require.Nil(t, c.AttendanceOnline)
			require.Nil(t, c.AttendanceDaily)
			require.Nil(t, c.Membership)
		}
		return scope
	}
	baseline := prepare(basePath, strings.Join(oldDomains, ","), false)
	runtime.GC()
	restored := prepare(restoredPath, restoredSelection, true)
	require.Equal(t, baseline, restored, "all retained R2 source scopes must survive")
	raw, err = json.Marshal(restored)
	require.NoError(t, err)
	t.Logf("R2 58 domains and restored 61 domains checked; retained scopes=%s", raw)
}
