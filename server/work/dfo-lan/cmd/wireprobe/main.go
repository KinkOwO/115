// wireprobe is a protocol experiment gateway. It binds the game port on the
// host named by -game-listen, which accepts a wildcard (0.0.0.0:PORT) so
// clients on other machines can reach it, and publishes the dialable address
// through the channel directory via -advertise-host.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"dfolan/internal/progression"
	"dfolan/internal/quest"
	"dfolan/internal/storage"
	"dfolan/internal/world"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// envStrOr 读一个字符串环境变量；缺失时返回 fallback。
func envStrOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

// envByteOr 读一个 0..255 的环境变量；缺失或非法时返回 fallback。
// 装备库制作的应答里有几个单字节开关，做成 env 就能不改代码切换。
func envByteOr(name string, fallback int) int {
	v := os.Getenv(name)
	if v == "" {
		return fallback
	}
	n, e := strconv.Atoi(v)
	if e != nil || n < 0 || n > 255 {
		return fallback
	}
	return n
}

func main() {
	moonConfigFile := flag.String("moon-solo-config", os.Getenv("DFO_MOON_SOLO_CONFIG"), "opt-in Moon Lake solo candidate, explicitly validated 2.38.3.25 profile")
	fixture := flag.String("fixture", "", "verified-format server fixture to send after accept")
	dir := flag.String("output", "runtime/wireprobe", "capture directory")
	gameListen := flag.String("game-listen", "127.0.0.1:0", "game endpoint; use 0.0.0.0:PORT to accept clients from other machines")
	advertiseHost := flag.String("advertise-host", os.Getenv("DFO_ADVERTISE_HOST"), "host the client dials for the game and channel directory; empty reuses the bound address, or auto-detects the LAN IPv4 when game-listen is a wildcard")
	responseFile := flag.String("responses", "", "JSON mapping command IDs to response fixture paths")
	characterStorage := flag.String("character-storage", "", "enable experimental persisted character handling with this local storage config")
	characterCatalog := flag.String("character-catalog", "configs/characters.generated.json", "PVF-derived profession catalog")
	pvfCatalogSelection := flag.String("pvf-catalogs", os.Getenv("DFO_PVF_CATALOGS"), "candidate direct-read domains: quests,progression,world,items,equipment,periods,skins,journal,create-cost,skills,prices,materials,boosters,tutorial,enhancements,random-options,shields,oath-grades,vault,loot,equipment-selection; empty keeps JSON")
	pvfVerifyBaselines := flag.Bool("pvf-verify-baselines", os.Getenv("DFO_PVF_VERIFY_BASELINES") != "0", "compare selected PVF domains with JSON baselines before storage; false removes the selected JSON startup dependency")
	pvfEnhancementPolicy := flag.String("pvf-enhancement-policy", envStrOr("DFO_PVF_ENHANCEMENT_POLICY", "configs/pvf-enhancement-policy.json"), "independent enhancement server policies; required only for PVF enhancements")
	pvfVaultPolicy := flag.String("pvf-vault-policy", envStrOr("DFO_PVF_VAULT_POLICY", "configs/pvf-vault-policy.json"), "client capacity/save policy independent of PVF account vault table")
	pvfContentPolicyPath := flag.String("pvf-content-policy", envStrOr("DFO_PVF_CONTENT_POLICY", "configs/pvf-content-policy.json"), "independent enabled special-content selection; source tables from PVF")
	pvfSelectionPolicyPath := flag.String("pvf-selection-policy", envStrOr("DFO_PVF_SELECTION_POLICY", "configs/pvf-selection-policy.json"), "bounded selection box templates; source categories from PVF")
	pvfItemShopPolicyPath := flag.String("pvf-item-shop-policy", envStrOr("DFO_PVF_ITEM_SHOP_POLICY", "configs/pvf-item-shop-policy.json"), "existing service shop routes and purchase-limit policy; offers from native SHP")
	pvfBoxPolicyPath := flag.String("pvf-box-policy", envStrOr("DFO_PVF_BOX_POLICY", "configs/pvf-box-policy.json"), "enabled COS/box scope and existing placement defaults; rewards from native material bindings")
	pvfCharacterPolicyPath := flag.String("pvf-character-policy", envStrOr("DFO_PVF_CHARACTER_POLICY", "configs/pvf-character-policy.json"), "saved source identity and default shortcut behavior; profession source fields from PVF")
	pvfLayerRevisitPolicyPath := flag.String("pvf-layer-revisit-policy", envStrOr("DFO_PVF_LAYER_REVISIT_POLICY", "configs/pvf-layer-revisit-policy.json"), "verified layer revisit scope, record and cache restoration; source maps and landing from PVF")
	pvfScriptWarpPolicyPath := flag.String("pvf-script-warp-policy", envStrOr("DFO_PVF_SCRIPT_WARP_POLICY", "configs/pvf-script-warp-policy.json"), "verified script warp scope and witnessed transition records; source routes from PVF")
	pvfLotteryPolicyPath := flag.String("pvf-lottery-policy", envStrOr("DFO_PVF_LOTTERY_POLICY", "configs/pvf-lottery-policy.json"), "enabled and grantable lottery pool templates; source odds from PVF")
	pvfScenePolicyPath := flag.String("pvf-scene-policy", envStrOr("DFO_PVF_SCENE_POLICY", "configs/pvf-scene-policy.json"), "independent entry town and training dungeon selection; source rules from PVF")
	pvfDropPolicy := flag.String("pvf-drop-policy", envStrOr("DFO_PVF_DROP_POLICY", "configs/pvf-drop-policy.json"), "existing basic equipment allowlist and maximum loot grade; source rules from PVF")
	pvfArchivePath := flag.String("pvf-archive", os.Getenv("DFO_PVF_ARCHIVE"), "explicit inner PVF path for candidate domains")
	pvfArchiveChecksum := flag.String("pvf-sha256", os.Getenv("DFO_PVF_SHA256"), "expected inner PVF SHA256; must match existing character source")
	characterRules := flag.String("character-rules", "configs/character-probe.json", "explicit local bootstrap settings")
	selectProbeConfig := flag.String("select-probe-config", "", "opt in to current-build SELECT parser experiment; does not initialize a town")
	entryBasicProbe := flag.Bool("entry-basic-probe", false, "send experimental current-build minimum actor info after SELECT; does not initialize a town")
	townCatalogFile := flag.String("town-catalog", "", "PVF-derived town-area catalog for the entry experiment")
	townProbeFile := flag.String("town-entry-probe", "", "opt in to experimental town entry using this separate spawn policy")
	worldCatalogFile := flag.String("world-catalog", "", "enable source-backed town transitions and saved positions")
	worldRulesFile := flag.String("world-rules", "configs/world-probe.json", "separate world movement policy")
	entryAdditionProbe := flag.Bool("entry-addition-probe", false, "send current-build source attributes; optional inventory and skills remain pending")
	questCatalogFile := flag.String("quest-catalog", "", "enable source quest accept/abandon persistence; objectives and rewards are separate")
	vaultRulesFile := flag.String("vault-rules", "", "source vault capacity and empty-state initialization")
	fatigueRulesFile := flag.String("fatigue-rules", "", "separate persisted fatigue and rollover policy")
	dungeonCatalogFile := flag.String("dungeon-catalog", "", "source dungeon layouts and first-room loading experiment")
	progressionCatalogFile := flag.String("progression-catalog", "", "current-source experience and growth catalog")
	progressionRulesFile := flag.String("progression-rules", "configs/experience.compat90.json", "separate reference compatibility formula settings")
	lootCatalogFile := flag.String("loot-catalog", "", "current gold/ordinary stackable source projection; equipment pending")
	lootRulesFile := flag.String("loot-rules", "configs/drop.compat90.json", "explicit reference drop formula policy")
	equipmentCatalogFile := flag.String("equipment-catalog", os.Getenv("DFO_EQUIPMENT_CATALOG"), "source equipment catalog a run selects gear from; required whenever loot is enabled")
	// 装备库（装备图鉴）规则表：cmd/equipmentjournalimport 的产物。留空 = 不登记装备库，
	// 分解保持原行为（只扣来源、发材料、写回执）。
	equipmentJournalRulesFile := flag.String("equipment-journal-rules", os.Getenv("DFO_EQUIPMENT_JOURNAL_RULES"), "装备库规则表（equipmentsetjournal.cos 的导出物）；留空则不登记")
	// 装备库「装备生成」的成本表（同一份源的 [create cost] 段）。留空 = 不做生成。
	equipmentCreateCostFile := flag.String("equipment-create-cost", os.Getenv("DFO_EQUIPMENT_CREATE_COST"), "装备生成成本表（[create cost] 的导出物）；留空则第二步只回窗口")
	// 装备库「制作 / 变换」（CMD2259）应答里的窗口选择字节：非 0 → 打开窗口 3937，
	// 0 → 打开窗口 2145。哪个才是"制作界面"尚未定案，故做成 flag/env 以便不改代码切换。
	equipmentCraftWindowFlag := flag.Int("equipment-craft-window", envByteOr("DFO_EQUIPMENT_CRAFT_WINDOW", 1), "CMD2259 应答的窗口选择字节（非 0 → 窗口 3937；0 → 窗口 2145）")
	equipmentCraftVariantFlag := flag.Int("equipment-craft-variant", envByteOr("DFO_EQUIPMENT_CRAFT_VARIANT", 0), "CMD2259 应答的子分支字节（仅当窗口字节为 0 时生效）")
	// 第二步（"确定"）用另一组参数：默认 u8@4 = 0 → 窗口 2145。
	equipmentCraftConfirmWindowFlag := flag.Int("equipment-craft-confirm-window", envByteOr("DFO_EQUIPMENT_CRAFT_CONFIRM_WINDOW", 0), "CMD2259 第二步（确定）应答的窗口选择字节")
	equipmentCraftConfirmVariantFlag := flag.Int("equipment-craft-confirm-variant", envByteOr("DFO_EQUIPMENT_CRAFT_CONFIRM_VARIANT", 0), "CMD2259 第二步应答的子分支字节")
	// 装备生成（请求头 [12] == 0）走另一扇窗：u8@4 = 0 → 窗口 2145。
	equipmentCraftGenerateWindowFlag := flag.Int("equipment-craft-generate-window", envByteOr("DFO_EQUIPMENT_CRAFT_GENERATE_WINDOW", 0), "CMD2259 装备生成（[12]=0）应答的窗口选择字节")
	equipmentCraftGenerateVariantFlag := flag.Int("equipment-craft-generate-variant", envByteOr("DFO_EQUIPMENT_CRAFT_GENERATE_VARIANT", 1), "CMD2259 装备生成应答的子分支字节（1 = 只落成功标志、不动窗口状态，默认；0 = 强制 setState 到状态 3，会让材料切换按钮失灵）")
	// 是否**真的执行**装备生成（扣料 + 发装备）。默认开；关掉则只回窗口、不动存档。
	equipmentCraftExecuteFlag := flag.Bool("equipment-craft-execute", os.Getenv("DFO_EQUIPMENT_CRAFT_EXECUTE") != "0", "CMD2259 是否执行装备生成（扣成本 + 发装备）")
	// 在哪一次请求上执行：confirm（同指纹第二次）/ first（第一次就执行）/ never。
	equipmentCraftExecuteOnFlag := flag.String("equipment-craft-execute-on", envStrOr("DFO_EQUIPMENT_CRAFT_EXECUTE_ON", "confirm"), "CMD2259 何时执行装备生成：confirm / first / never")
	// 装备变换（CMD2259 action=1）怎么执行：apply = 真的换装；observe = 只记日志、不动存档。
	equipmentTransformApplyFlag := flag.String("equipment-transform", envStrOr("DFO_EQUIPMENT_TRANSFORM_APPLY", "apply"), "CMD2259 action=1（装备变换）如何执行：apply（真的换装）/ observe（只记日志）")
	bagRulesFile := flag.String("bag-rules", "configs/inventory.compat90.json", "separate bag slot and missing stack limit policy")
	boxesFile := flag.String("boxes", "", "imported open-box content tables; empty resolves boxes.json beside the bag rules")
	cardRulesFile := flag.String("card-rules", "configs/cards.compat90.json", "separate compatible free-card policy")
	learningFile := flag.String("skill-catalog", "", "current PVF learning metadata; enables manual learning and persisted skill slots")
	channelRefreshFile := flag.String("channel-refresh-config", "", "separate local channel directory service for native refresh")
	channelIdentityEnabled := flag.Bool("channel-identity", false, "candidate: synchronize NOTI2435 and all actor contexts with the connected channel")
	equipmentRewardFile := flag.String("quest-equipment-catalog", "", "source basic-equipment metadata for atomic quest rewards")
	wearRulesFile := flag.String("equipment-wear-rules", "", "current-client equipment slots and persistent wear handling")
	knightShieldFile := flag.String("knight-shield-catalog", "equipment-knight-shield.full-candidate.json", "optional source-verified shield window side-car; relative to wear rules directory, empty disables")
	fullEquipmentFile := flag.String("equipment-full-catalog", os.Getenv("DFO_EQUIPMENT_FULL_CATALOG"), "separate indexed wear catalog prefix; does not widen drops")
	itemIndexFile := flag.String("item-index", os.Getenv("DFO_ITEM_INDEX"), "full stackable item index JSON (e.g. configs/items.index.json)")
	boosterCatalogFile := flag.String("booster-catalog", os.Getenv("DFO_BOOSTER_CATALOG"), "booster definitions JSON")
	selectionBoxFile := flag.String("selection-boxes", os.Getenv("DFO_SELECTION_BOXES"), "source selection box JSON ([booster select category] boxes)")
	itemShopFile := flag.String("item-shop", os.Getenv("DFO_ITEM_SHOP"), "source item shop JSON (itemshop/**.shp; prices goods with [need material], e.g. the Odyssey shop's silver coins)")
	shopPricesFile := flag.String("shop-prices", os.Getenv("DFO_SHOP_PRICES"), "source NPC prices; empty resolves shop-prices.json beside the loot catalog")
	bleedingMineRewardsFile := flag.String("bleeding-mine-rewards", "", "赤红铁矿原版奖励表；默认读取掉落目录旁的 bleeding-mine-rewards.json")
	soloPartyBootstrap := flag.Bool("solo-party-bootstrap", false, "initialize the owned actor in the current solo party roster")
	accountOptionsFile := flag.String("account-options", "", "sparse current-client account option overrides; other defaults remain client-owned")
	unifiedCharacFile := flag.String("unified-charac-template", "", "override the built-in 3539 byte character option block sent as NOTI2827 (different client build only)")
	skillLockOffset := flag.Int("skill-lock-offset", -1, "override the subtype 19 skill lock offset inside the character option block (default 2736)")
	tutorialRoutesFile := flag.String("tutorial-routes", "", "source per-job starting route table")
	tutorialDungeonsFile := flag.String("tutorial-dungeons", "", "source starting-route dungeon catalog")
	shopPilotFile := flag.String("shop-purchase-pilot", os.Getenv("DFO_SHOP_PURCHASE_PILOT"), "isolated single-item cash purchase pilot catalog")
	shopRelease := flag.Bool("shop-release", os.Getenv("DFO_SHOP_RELEASE") == "1", "enable accepted ordinary shop in release profile")
	vaultPurchase := flag.Bool("vault-purchase-candidate", os.Getenv("DFO_VAULT_PURCHASE_CANDIDATE") == "1", "enable isolated vault purchase candidate")
	vaultRelease := flag.Bool("vault-purchase-release", os.Getenv("DFO_VAULT_PURCHASE_RELEASE") == "1", "enable accepted personal vault purchases in release profile")
	randomOptionFile := flag.String("random-option-catalog", os.Getenv("DFO_RANDOM_OPTION_CATALOG"), "current-client magic-seal random option rules; enables CMD393 unsealing")
	apocalypseCatalogFile := flag.String("apocalypse-catalog", "configs/apocalypse.generated.json", "compiled apocalypse.ctp table (phase clock, operations, gates, rewards, duty skills)")
	attunementRewardsFile := flag.String("attunement-rewards", os.Getenv("DFO_ATTUNEMENT_REWARDS"), "boundary-of-attunement reward table generated from the source rewardboostinfo CTPs")
	attunementRebalanceOn := flag.Bool("attunement-rebalance", os.Getenv("DFO_ATTUNEMENT_REBALANCE") == "1", "本私服的掉落调参（**与官服的显式差异**）：征兆「无事发生」减半、fixed 池低档按比例向高档倾斜。见 internal/loot/attunement_rebalance.go")
	attunementFixedTiltDefault := 25
	if v := os.Getenv("DFO_ATTUNEMENT_FIXED_TILT"); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil {
			attunementFixedTiltDefault = n
		}
	}
	attunementFixedTilt := flag.Int("attunement-fixed-tilt", attunementFixedTiltDefault, "固定池倾斜幅度：普通/稀有各减这么多百分比权重，减掉的按高档现有比例补（1..99）。0 = 不动固定池；只在 -attunement-rebalance 打开时生效")
	boosterGageHide := flag.Bool("booster-gage-hide", os.Getenv("DFO_BOOSTER_GAGE") != "0", "send NOTI398 booster-gage with displayValue=0 on town entry to hide the top-left Liberation Trace panel; disable with -booster-gage-hide=false or DFO_BOOSTER_GAGE=0")
	oathGrades := flag.String("oath-grades", os.Getenv("DFO_OATH_GRADES"), "诊断覆盖：固定下发的引子/誓约档位 primer,oath（见 oath_info.go）。留空 = 按角色穿戴的誓约/引子装备算，这是正常路径")
	oathGradesTable := flag.String("oath-grades-table", os.Getenv("DFO_OATH_GRADES_TABLE"), "誓约/引子装备稀有度表（cmd/oathgradeimport 生成）；只在 -oath-grades-from-gear 打开时用")
	oathFromGear := flag.Bool("oath-grades-from-gear", os.Getenv("DFO_OATH_GRADES_FROM_GEAR") == "1", "诊断：按角色穿戴的誓约/引子装备算档位（旧规则）。默认关 —— 客户端脱不下誓约槽，穿上 primeval 就永久 oath=45")
	oathProgressClearsDefault := oathDefaultProgressClears
	if v := os.Getenv("DFO_OATH_PROGRESS_CLEARS"); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil {
			oathProgressClearsDefault = n
		}
	}
	oathProgressDungeonSpec := os.Getenv("DFO_OATH_PROGRESS_DUNGEONS")
	if oathProgressDungeonSpec == "" {
		oathProgressDungeonSpec = oathDefaultProgressDungeons
	}
	oathProgressClears := flag.Int("oath-progress-clears", oathProgressClearsDefault, "隐藏 BOSS 的保底场次：-oath-progress-dungeons 里的副本通关这么多场后，下一场下发 oath=45（必出一次）并在通关时归零；<=0 关闭保底")
	oathProgressDungeons := flag.String("oath-progress-dungeons", oathProgressDungeonSpec, "计入保底的副本号，逗号分隔（默认只有小深渊 100005014）")
	oathInject := flag.String("oath-inject", os.Getenv("DFO_OATH_INJECT"), "诊断用：向客户端注入任意 noti 的候选列表，形式 id:size:fill;off:val,...（见 oath_probe.go）；默认空 = 关闭")
	omenHoldDefault := -1
	if v := os.Getenv("DFO_OMEN_HOLD"); v != "" {
		if n, convErr := strconv.Atoi(v); convErr == nil {
			omenHoldDefault = n
		}
	}
	omenHold := flag.Int("omen-hold", omenHoldDefault, "诊断：把玩家直接放到指定征兆阶段(0-4)，-1 = 不动；-omen-state 打开时会写回角色存档")
	omenRewards := flag.Bool("omen-rewards", os.Getenv("DFO_OMEN_REWARDS") == "1", "千海之空深渊的征兆系统：通关时按 [coupon drop table] 的阶段表累积并结算（见 internal/loot/omen.go）。默认关闭")
	omenInfo := flag.String("omen-info", os.Getenv("DFO_OMEN_INFO"), "诊断：直接指定 noti 2836「征兆队伍状态」的 69 字节载荷，用来点亮征兆 UI 并实测字段语义。写法见 cmd/wireprobe/omen_info.go；留空 = 按角色存档里的真实档数生成（需 -omen-state）")
	omenState := flag.Bool("omen-state", os.Getenv("DFO_OMEN_STATE") == "1", "征兆的正式状态：持有档数存进角色存档、进本按真实状态下发 noti 2836，并让隐藏 BOSS 由「满档结算」驱动（见 cmd/wireprobe/omen_state.go）。默认关闭")
	scaleDeathFromHP := flag.Bool("scale-death-from-hp", os.Getenv("DFO_SCALE_DEATH_FROM_HP") == "1", "boundary-of-attunement 定盘机关(109019266)的兜底判死：它血量触底时服务端合成一条死亡上报，不再依赖引擎那两个恒为 72 的 rarity 天花板；默认关闭")
	flag.Parse()
	pvfCatalogs, pvfCatalogErr := preparePVFCoreCatalogs(*pvfCatalogSelection, *pvfArchivePath, *pvfArchiveChecksum, *characterCatalog, *questCatalogFile, *progressionCatalogFile, *worldCatalogFile, pvfItemInputs{itemShopPath: *itemShopFile, itemShopPolicyPath: *pvfItemShopPolicyPath, boxesPath: *boxesFile, boxPolicyPath: *pvfBoxPolicyPath, cashshopPath: *shopPilotFile, cashshopRelease: *shopRelease, characterPolicyPath: *pvfCharacterPolicyPath, layerRevisitPolicyPath: *pvfLayerRevisitPolicyPath, scriptWarpPolicyPath: *pvfScriptWarpPolicyPath, lotteryPolicyPath: *pvfLotteryPolicyPath, selectionBoxesPath: *selectionBoxFile, selectionPolicyPath: *pvfSelectionPolicyPath, minePath: *bleedingMineRewardsFile, indexPath: *itemIndexFile, fullPrefix: *fullEquipmentFile, journalPath: *equipmentJournalRulesFile, createCostPath: *equipmentCreateCostFile, learningPath: *learningFile, pricesPath: *shopPricesFile, boosterPath: *boosterCatalogFile, tutorialPath: *tutorialRoutesFile, verifyBaselines: pvfVerifyBaselines, enhancementPolicyPath: *pvfEnhancementPolicy, randomOptionPath: *randomOptionFile, shieldPath: *knightShieldFile, wearRulesPath: *wearRulesFile, oathPath: *oathGradesTable, vaultPath: *vaultRulesFile, vaultPolicyPath: *pvfVaultPolicy, lootPath: *lootCatalogFile, equipmentPath: *equipmentCatalogFile, questEquipmentPath: *equipmentRewardFile, dropPolicyPath: *pvfDropPolicy, townPath: *townCatalogFile, dungeonPath: *dungeonCatalogFile, tutorialDungeonPath: *tutorialDungeonsFile, scenePolicyPath: *pvfScenePolicyPath, apocalypsePath: *apocalypseCatalogFile, attunementPath: *attunementRewardsFile, contentPolicyPath: *pvfContentPolicyPath})
	if pvfCatalogErr != nil {
		log.Fatalf("PVF candidate catalogs: %v", pvfCatalogErr)
	}
	if _, err := pvfCatalogs.installAdventureRules(); err != nil {
		log.Fatalf("PVF adventure runtime rules: %v", err)
	}
	if _, err := pvfCatalogs.installRecommendedRules(); err != nil {
		log.Fatalf("PVF recommended dungeon runtime rules: %v", err)
	}
	if _, err := pvfCatalogs.installSeasonRules(); err != nil {
		log.Fatalf("PVF season runtime rules: %v", err)
	}
	if _, err := pvfCatalogs.installRosterBackgrounds(); err != nil {
		log.Fatalf("PVF roster background runtime rules: %v", err)
	}
	if _, err := pvfCatalogs.installFameRules(); err != nil {
		log.Fatalf("PVF fame runtime rules: %v", err)
	}
	if _, err := pvfCatalogs.installScriptWarps(); err != nil {
		log.Fatalf("PVF script warp runtime routes: %v", err)
	}
	collectPVFImportMemory(pvfCatalogs)
	equipmentCraftWindow = byte(*equipmentCraftWindowFlag)
	equipmentCraftVariant = byte(*equipmentCraftVariantFlag)
	equipmentCraftConfirmWindow = byte(*equipmentCraftConfirmWindowFlag)
	equipmentCraftConfirmVariant = byte(*equipmentCraftConfirmVariantFlag)
	equipmentCraftExecute = *equipmentCraftExecuteFlag
	equipmentCraftGenerateWindow = byte(*equipmentCraftGenerateWindowFlag)
	equipmentCraftGenerateVariant = byte(*equipmentCraftGenerateVariantFlag)
	switch *equipmentCraftExecuteOnFlag {
	case "confirm", "first", "never":
		equipmentCraftExecuteOn = *equipmentCraftExecuteOnFlag
	default:
		log.Fatalf("invalid -equipment-craft-execute-on %q (want confirm/first/never)", *equipmentCraftExecuteOnFlag)
	}
	switch *equipmentTransformApplyFlag {
	case "apply", "observe":
		equipmentTransformApply = *equipmentTransformApplyFlag
	default:
		log.Fatalf("invalid -equipment-transform %q (want apply/observe)", *equipmentTransformApplyFlag)
	}
	oathGradePair, oathGradesErr := parseOathGrades(*oathGrades)
	if oathGradesErr != nil {
		log.Fatalf("bad -oath-grades: %v", oathGradesErr)
	}
	// 档位表只服务「按穿戴装备算档位」这条诊断路径（-oath-grades-from-gear）。
	// 默认的保底路径不需要它，所以默认配置下**不加载、也不会因为缺表拒绝启动**。
	var oathGradeTable *inventory.OathGradeTable
	if *oathFromGear {
		table, tableErr := pvfCatalogs.loadOathGrades(*oathGradesTable)
		if tableErr != nil {
			log.Fatalf("bad -oath-grades-table: %v", tableErr)
		}
		oathGradeTable = table
	}
	oathProgressSet, oathProgressErr := parseOathProgressDungeons(*oathProgressDungeons)
	if oathProgressErr != nil {
		log.Fatalf("bad -oath-progress-dungeons: %v", oathProgressErr)
	}
	switch {
	case len(oathGradePair) == 2 && (oathGradePair[0] != 0 || oathGradePair[1] != 0):
		log.Printf("oath grades: overridden to primer=%d oath=%d (diagnostic)", oathGradePair[0], oathGradePair[1])
	case *oathFromGear:
		log.Printf("oath grades: derived from worn oath/primer gear (%d known items, diagnostic)", oathGradeTable.Len())
	case *omenState:
		log.Printf("oath grades: hidden boss driven by an omen full settlement on %s", *oathProgressDungeons)
	case *oathProgressClears > 0:
		log.Printf("oath grades: hidden-boss pity every %d clear(s) of %s", *oathProgressClears, *oathProgressDungeons)
	default:
		log.Printf("oath grades: always normal (pity disabled)")
	}
	oathInjectSpecs, oathInjectErr := parseOathInject(*oathInject)
	if oathInjectErr != nil {
		log.Fatalf("bad -oath-inject: %v", oathInjectErr)
	}
	if len(oathInjectSpecs) > 0 {
		log.Printf("oath injector armed: %d candidate notification(s)", len(oathInjectSpecs))
	}
	// 征兆队伍状态（noti 2836）的载荷。**在启动期校验**：以前这段在频道会话建立时
	// （每个频道一次）才解析，写错一个字符就会在玩家"进频道"的那一刻 log.Fatalf，
	// 现象是"启动游戏进不去频道"，而且加载日志已经刷完、错误行在最底下，极难定位。
	omenInfoBytes, omenInfoErr := parseOmenInfo(*omenInfo)
	if omenInfoErr != nil {
		log.Fatalf("bad -omen-info: %v", omenInfoErr)
	}
	if len(omenInfoBytes) > 0 {
		log.Printf("omen info (noti 2836): injecting %d bytes: %s", len(omenInfoBytes), hex.EncodeToString(omenInfoBytes))
	}
	// -omen-state 单独打开是**静默坏掉**的配置：征兆阶段表才是推进持有数的那台机器，
	// 关掉它之后存档会永远停在 0（既不涨、也永远不会满档结算），而 UI 会一直显示
	// 空格子 —— 现象是「征兆系统上线了但什么都没发生」。宁可启动就报错。
	if *omenState && !*omenRewards {
		log.Fatal("-omen-state needs -omen-rewards: the [coupon drop table] roll is what advances the omen, " +
			"so a state-only run would sit at stage 0 forever")
	}
	// 掉落调参（与官服的显式差异）。开关关着时两个参数都不参与，表保持官方原值。
	attunementRebalance := loot.Rebalance{}
	if *attunementRebalanceOn {
		if *attunementFixedTilt < 0 || *attunementFixedTilt >= 100 {
			log.Fatalf("bad -attunement-fixed-tilt: %d is outside 0..99 (100 would empty the common tiers)", *attunementFixedTilt)
		}
		attunementRebalance = loot.Rebalance{
			OmenHalveIdle:    true,
			FixedTiltPercent: uint32(*attunementFixedTilt),
		}
	}
	if *fullEquipmentFile == "" {
		for _, cand := range []string{
			"configs/equipment-full",
			"cmd/wireprobe/testdata/odyssey-equipment",
		} {
			if _, err := os.Stat(cand + ".index.json"); err == nil {
				if _, err := os.Stat(cand + ".data"); err == nil {
					*fullEquipmentFile = cand
					break
				}
			}
		}
	}
	if *randomOptionFile == "" && pvfCatalogs.randomOptions == nil {
		if _, err := os.Stat("configs/randomoption.current37.json"); err == nil {
			*randomOptionFile = "configs/randomoption.current37.json"
		}
	}
	if *shopPilotFile == "" && pvfCatalogs.cashshop == nil {
		for _, cand := range []string{
			"configs/shop-vault-release.json",
			"configs/shop-purchase-pilot.json",
		} {
			if _, err := os.Stat(cand); err == nil {
				*shopPilotFile = cand
				*shopRelease = true
				break
			}
		}
	}
	if *boosterCatalogFile == "" {
		for _, cand := range []string{
			"configs/booster-catalog.json",
			"server/work/dfo-lan/configs/booster-catalog.json",
		} {
			if _, err := os.Stat(cand); err == nil {
				*boosterCatalogFile = cand
				break
			}
		}
	}
	if *selectionBoxFile == "" {
		candidates := []string{
			"configs/selection-boxes-release.json",
			"configs/selection-boxes-candidate.json",
			"server/work/dfo-lan/configs/selection-boxes-candidate.json",
		}
		// 网关通常不是从模块根启动的（启动器的工作目录是 server/），所以再按
		// "与已经显式给出的目录同目录"推导一次——那些路径是绝对路径。
		for _, base := range []string{*boosterCatalogFile, *itemIndexFile} {
			if base == "" {
				continue
			}
			dir := filepath.Dir(base)
			candidates = append(candidates,
				filepath.Join(dir, "selection-boxes-release.json"),
				filepath.Join(dir, "selection-boxes-candidate.json"))
		}
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				*selectionBoxFile = cand
				break
			}
		}
	}
	if *itemShopFile == "" && pvfCatalogs.itemShops == nil {
		candidates := []string{
			"configs/itemshop-release.json",
			"configs/itemshop-candidate.json",
			"server/work/dfo-lan/configs/itemshop-candidate.json",
		}
		for _, base := range []string{*boosterCatalogFile, *itemIndexFile} {
			if base == "" {
				continue
			}
			dir := filepath.Dir(base)
			candidates = append(candidates,
				filepath.Join(dir, "itemshop-release.json"),
				filepath.Join(dir, "itemshop-candidate.json"))
		}
		for _, cand := range candidates {
			if _, err := os.Stat(cand); err == nil {
				*itemShopFile = cand
				break
			}
		}
	}
	if *itemIndexFile == "" {
		for _, cand := range []string{
			"configs/items.index.json",
			"server/work/dfo-lan/configs/items.index.json",
		} {
			if _, err := os.Stat(cand); err == nil {
				*itemIndexFile = cand
				break
			}
		}
	}
	if *boosterCatalogFile == "" && *itemIndexFile != "" {
		cand := filepath.Join(filepath.Dir(*itemIndexFile), "booster-catalog.json")
		if _, err := os.Stat(cand); err == nil {
			*boosterCatalogFile = cand
		}
	}
	skillRelease := os.Getenv("DFO_SKILL_RELEASE") == "1"
	if candidateSkills := os.Getenv("DFO_SKILL_CATALOG"); candidateSkills != "" {
		*learningFile = candidateSkills
	}
	// NOTI2827 restores locked skills from the client's own character option
	// block. The built-in block is the same version as this client, so the
	// template file and the offset override are escapes for a different build.
	var unifiedCharacTemplate []byte
	if *unifiedCharacFile != "" {
		data, err := os.ReadFile(*unifiedCharacFile)
		if err != nil {
			log.Fatal(err)
		}
		if len(data) == 0 {
			log.Fatal("empty character option template")
		}
		unifiedCharacTemplate = data
		log.Printf("NOTI2827 character option block overridden by %s (%d bytes)", *unifiedCharacFile, len(data))
	}
	if *skillLockOffset >= 0 {
		log.Printf("NOTI2827 skill lock offset overridden to %d", *skillLockOffset)
	}
	var accountOptionsPayload []byte
	if *accountOptionsFile != "" {
		data, err := os.ReadFile(*accountOptionsFile)
		if err != nil {
			log.Fatal(err)
		}
		var overrides map[uint16]uint16
		if err = json.Unmarshal(data, &overrides); err != nil {
			log.Fatal(err)
		}
		accountOptionsPayload, err = protocol.AccountOptions(overrides)
		if err != nil {
			log.Fatal(err)
		}
	}
	gameHost, _, gameListenError := net.SplitHostPort(*gameListen)
	// A wildcard bind is what makes the gateway reachable from other machines;
	// an explicit host still has to be a numeric address rather than a name.
	if gameListenError != nil || (gameHost != "" && net.ParseIP(gameHost) == nil) {
		log.Fatal("game-listen must be host:port with a numeric IP host")
	}
	if *entryBasicProbe && (*selectProbeConfig == "" || *characterStorage == "") {
		log.Fatal("entry basic probe requires persisted characters and SELECT probe configuration")
	}
	var townCatalog catalog.TownArea
	var townPolicy struct {
		X     uint16  `json:"x"`
		Y     uint16  `json:"y"`
		Flags [3]byte `json:"flags"`
	}
	if *townProbeFile != "" {
		if !*entryBasicProbe || (*townCatalogFile == "" && pvfCatalogs.town == nil) {
			log.Fatal("town probe requires basic actor and town catalog")
		}
		var e error
		townCatalog, e = pvfCatalogs.loadTown(*townCatalogFile)
		if e != nil {
			log.Fatal(e)
		}
		b, e := os.ReadFile(*townProbeFile)
		if e != nil {
			log.Fatal(e)
		}
		if e = json.Unmarshal(b, &townPolicy); e != nil {
			log.Fatal(e)
		}
		if !townCatalog.Allows(255, townPolicy.X, townPolicy.Y) {
			log.Fatal("spawn policy lies outside source walkable rectangles")
		}
	}
	var selectProbe *protocol.SelectProbeState
	if *selectProbeConfig != "" {
		b, e := os.ReadFile(*selectProbeConfig)
		if e != nil {
			log.Fatal(e)
		}
		selectProbe = new(protocol.SelectProbeState)
		if e = json.Unmarshal(b, selectProbe); e != nil {
			log.Fatal(e)
		}
		if _, e = protocol.SelectProbeSuccess(*selectProbe); e != nil {
			log.Fatal(e)
		}
	}
	var characters *character.Service
	var worldService *world.Service
	var wearService *inventory.WearService
	var questService *quest.Service
	var townArrivalScenes map[uint32]catalog.TownArrivalScene
	var vaultService *inventory.VaultService
	var fatigueService *character.FatigueService
	var developmentAccount int64
	var dungeonCatalog *catalog.DungeonCatalog
	var progressionService *character.ProgressionService
	var lootService *loot.Service
	// journalRules 是装备库规则（nil = 不登记）。它同时被 CMD26 的事务与入场 2610 用到，
	// 所以在这里声明、在 loot 块里装载。
	var journalRules *catalog.EquipmentJournalRules
	// equipmentCreateCost 是「装备生成」成本表（nil = 第二步只回窗口、不生成）。
	var equipmentCreateCost *catalog.EquipmentCreateCost
	var shopPilot *cashshop.Pilot
	var unsealService *inventory.UnsealService
	// skinCatalog maps an `[add skin storage]` stackable template to its PVF
	// facts, driving CMD507 action 169 (damage font) registration. Nil when no
	// item index is configured, which disables the skin flow.
	var skinCatalog map[uint32]catalog.SkinStorageEntry
	if *characterStorage != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cfg, e := storage.LoadConfig(*characterStorage)
		if e != nil {
			log.Fatal(e)
		}
		s, e := storage.Open(ctx, cfg)
		if e != nil {
			log.Fatal(e)
		}
		defer s.Close()
		releaseAdminGuard, e := s.HoldAdminGuard(ctx)
		if e != nil {
			log.Fatal(e)
		}
		defer releaseAdminGuard()
		if e = s.Migrate(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigrateAdventure(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigrateBleedingMine(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigrateTutorial(ctx); e != nil {
			log.Fatal(e)
		}
		// Account/character unified options (CMD2377 0x01/0x05) persist here;
		// the account block restores through NOTI2826, character settings are
		// stored until the NOTI2827 layout is reversed.
		if e = s.MigrateUnifiedOptions(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigrateGamepad(ctx); e != nil {
			log.Fatal(e)
		}
		// The account cera ledger backs the balance sent in SELECT.
		if e = s.MigrateGrants(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigratePremiums(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigrateCharacterEvents(ctx); e != nil {
			log.Fatal(e)
		}
		// NPC 商店限购流水（`[purchase limit]`）。新表而不是复用 character_events：
		// 那张表主键是 (character_id, event_key)，同一 key 只能一行，而限购要可累加的行。
		if e = s.MigrateShopPurchases(ctx); e != nil {
			log.Fatal(e)
		}
		// Per-character read-notice ledger (NOTI402/426) backs the teaching
		// frame suppression for third-awakened characters.
		if e = s.MigrateCharacterNotices(ctx); e != nil {
			log.Fatal(e)
		}
		// Per-character profile skin snapshot (NOTI1545/1546) backs the
		// category-0 owned/selected state sent on character entry.
		if e = s.MigrateProfileSkins(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigrateRosterBackgrounds(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigrateMailbox(ctx); e != nil {
			log.Fatal(e)
		}
		// Per-(character,dungeon) hidden-boss pity counter. The client's tier
		// ladder has no roll, so this table is the only place "rare" can live.
		if e = s.MigrateOathProgress(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigrateOathOptions(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigrateEquipmentSkill(ctx); e != nil {
			log.Fatal(e)
		}
		// Per-(character,dungeon) omen save slot. The omen is not an item: it is a
		// character-save marker the client reads out of NOTI2836 (see omen_state.go).
		if e = s.MigrateOmenState(ctx); e != nil {
			log.Fatal(e)
		}
		data, e := pvfCatalogs.loadCharacters(*characterCatalog)
		if e != nil {
			log.Fatal(e)
		}
		b, e := os.ReadFile(*characterRules)
		if e != nil {
			log.Fatal(e)
		}
		var rules character.Rules
		if e = json.Unmarshal(b, &rules); e != nil {
			log.Fatal(e)
		}
		characters, e = character.New(s, data, rules)
		if characters != nil {
			characters.DisableActorAppearance = skillRelease
			characters.DetailedWornCandidate = !skillRelease
		}
		if e != nil {
			log.Fatal(e)
		}
		// 带期限物品一律按「永不过期」下发。**默认开启**（DFO_MAX_ITEM_PERIOD=0 才关）：
		// 三个 .cmd 入口都设了这个变量，但一键启动器自己拉起 launch_local.py、
		// 从不设置它 ⇒ 走一键启动器时整条兜底不生效，脚本声明过期限的模板
		// （银增幅书到期日 2022-11-08 之类）就会带着 0 下发，客户端显示
		// 「剩余期限已过」并拒绝使用（错误码 31730）。
		if os.Getenv("DFO_MAX_ITEM_PERIOD") != "0" {
			protocol.ConfigureStoredPeriodLifting(true)
			if *itemIndexFile == "" && pvfCatalogs.periods == nil {
				log.Printf("maximum item period: no item index (-item-index), lifting stored periods only")
			} else {
				periodFile := filepath.Join(filepath.Dir(*itemIndexFile), "item-period-tags.json")
				templates, periodErr := pvfCatalogs.loadItemPeriods(periodFile, data.Source.Checksum)
				if periodErr != nil {
					if pvfCatalogs.periods != nil {
						log.Fatal(periodErr)
					}
					// 表读不到不再致命：退化为「存档里已有的非零期限一律抬到最大值」，
					// 至少不会把已过期的旧道具重新判成过期。
					log.Printf("maximum item period table unavailable (%v), lifting stored periods only", periodErr)
				} else {
					protocol.ConfigureMaxItemPeriods(templates)
					log.Printf("maximum item period enabled for %d PVF templates", len(templates))
				}
			}
		}
		// Skin-cargo registration (CMD507 action 169, `[add skin storage]`) reads
		// the skin key straight from PVF and persists the unlock per account.
		if *itemIndexFile != "" || pvfCatalogs.skins != nil {
			skinFile := filepath.Join(filepath.Dir(*itemIndexFile), "skin-storage-items.json")
			entries, skinErr := pvfCatalogs.loadSkinStorage(skinFile, data.Source.Checksum)
			if skinErr != nil {
				log.Printf("skin storage registration disabled: %v", skinErr)
			} else if e = s.MigrateSkinCargo(ctx); e != nil {
				log.Fatal(e)
			} else if e = s.MigrateSkinSelection(ctx); e != nil {
				log.Fatal(e)
			} else if e = s.MigrateSkinSelectionList(ctx); e != nil {
				log.Fatal(e)
			} else {
				skinCatalog = entries
				templates := make([]uint32, 0, len(entries))
				for template := range entries {
					templates = append(templates, template)
				}
				protocol.ConfigureSkinStoragePeriods(templates)
				log.Printf("skin storage registration armed for %d PVF templates", len(entries))
			}
		}
		if *shopPilotFile != "" || pvfCatalogs.cashshop != nil {
			var database string
			if e = s.DB.QueryRow(ctx, "SELECT current_database()").Scan(&database); e == nil {
				if database != "dfo_swordmaster_pilot_20260916" && !*shopRelease {
					log.Printf("shop purchase pilot running on database: %s", database)
				}
			}
			shopPilot, e = pvfCatalogs.loadCashShop(*shopPilotFile, data.Source.Checksum, *shopRelease)
			if e != nil {
				log.Fatal(e)
			}
			if e = s.MigrateCashShop(ctx); e != nil {
				log.Fatal(e)
			}
			log.Printf("PVF shop enabled: %d ordinary products", shopPilot.EnabledCount())
			log.Printf("商城配置：%s，发布模式：%t", *shopPilotFile, shopPilot.Config.Release)
		}
		if *learningFile != "" || pvfCatalogs.learning != nil {
			characters.Learning, e = pvfCatalogs.loadLearning(*learningFile, data.Source.Checksum)
			if e != nil {
				log.Fatal(e)
			}
			if e = s.MigrateCharacterEvents(ctx); e != nil {
				log.Fatal(e)
			}
			if e = s.MigrateCharacterNotices(ctx); e != nil {
				log.Fatal(e)
			}
			if e = s.MigrateSkillLocks(ctx); e != nil {
				log.Fatal(e)
			}
		}
		developmentAccount, e = s.DevelopmentAccount(ctx, "probe")
		if e != nil {
			log.Fatal(e)
		}
	}
	if *entryAdditionProbe && !*entryBasicProbe {
		log.Fatal("addition requires a basic actor")
	}
	if *fatigueRulesFile != "" {
		if characters == nil || selectProbe == nil {
			log.Fatal("fatigue requires persisted characters and SELECT")
		}
		var e error
		fatiguePath := *fatigueRulesFile
		if path := os.Getenv("DFO_FATIGUE_RULES"); path != "" {
			fatiguePath = path
		}
		fatigueService, e = character.LoadFatigueService(characters.Store, fatiguePath)
		if e != nil {
			log.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = characters.Store.MigrateFatigue(ctx)
		cancel()
		if e != nil {
			log.Fatal(e)
		}
	}
	if *worldCatalogFile != "" || pvfCatalogs.world != nil {
		if characters == nil || *townProbeFile == "" {
			log.Fatal("world requires persisted characters and a spawn policy")
		}
		data, e := pvfCatalogs.loadWorld(*worldCatalogFile)
		if e != nil {
			log.Fatal(e)
		}
		b, e := os.ReadFile(*worldRulesFile)
		if e != nil {
			log.Fatal(e)
		}
		var rules world.Rules
		if e = json.Unmarshal(b, &rules); e != nil {
			log.Fatal(e)
		}
		if data.Source.Checksum != characters.Catalog.Source.Checksum {
			log.Fatal("world/character source versions differ")
		}
		worldService = &world.Service{Store: characters.Store, Catalog: data, Rules: rules}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = characters.Store.MigrateWorld(ctx)
		cancel()
		if e != nil {
			log.Fatal(e)
		}
	}
	if *dungeonCatalogFile != "" || pvfCatalogs.dungeons != nil {
		if candidate := os.Getenv("DFO_ODYSSEY_DUNGEON_CATALOG"); candidate != "" {
			*dungeonCatalogFile = candidate
		}
		if worldService == nil {
			log.Fatal("dungeons require world sessions")
		}
		data, e := pvfCatalogs.loadDungeons(*dungeonCatalogFile)
		if e != nil {
			log.Fatal(e)
		}
		trainingRoomPath := os.Getenv("DFO_TRAINING_ROOM_CATALOG")
		if trainingRoomPath == "" {
			trainingRoomPath = filepath.Join(filepath.Dir(*dungeonCatalogFile), "dungeons.training-room.json")
		}
		trainingRooms, e := pvfCatalogs.loadTrainingDungeons(trainingRoomPath)
		if e != nil {
			log.Fatal(e)
		}
		if e = catalog.MergeDungeonCatalog(&data, trainingRooms); e != nil {
			log.Fatal(e)
		}
		if filepath.Base(*dungeonCatalogFile) == "dungeons.full.json" || pvfCatalogs.dungeons != nil {
			overlayDirectory := filepath.Dir(*dungeonCatalogFile)
			if pvfCatalogs.dungeons != nil {
				overlayDirectory = filepath.Dir(*characterCatalog)
			}
			path := filepath.Join(overlayDirectory, "dungeons.terminal-scenes.json")
			if e = pvfCatalogs.attachTerminalScenes(&data, path); e != nil {
				log.Fatal(e)
			}
			path = filepath.Join(overlayDirectory, "dungeons.layer-revisits.json")
			if e = pvfCatalogs.attachLayerRevisits(&data, path); e != nil {
				log.Fatal(e)
			}
			path = filepath.Join(overlayDirectory, "dungeons.tournament-quest-maps.json")
			if e = pvfCatalogs.attachTournamentMaps(&data, path); e != nil {
				log.Fatal(e)
			}
			path = filepath.Join(overlayDirectory, "dungeons.tower-of-grief-maps.json")
			if e = pvfCatalogs.attachTowerGrief(&data, path); e != nil {
				log.Fatal(e)
			}
			path = filepath.Join(overlayDirectory, "dungeons.tower-of-dazzlement-maps.json")
			if e = pvfCatalogs.attachTowerDazzlement(&data, path); e != nil {
				log.Fatal(e)
			}
			path = filepath.Join(overlayDirectory, "dungeons.maze-chance-rates.json")
			if e = pvfCatalogs.attachMazeRates(&data, path); e != nil {
				log.Fatal(e)
			}
			path = filepath.Join(overlayDirectory, "dungeons.hell-party-maps.json")
			if e = pvfCatalogs.attachHellMaps(&data, path); e != nil {
				log.Fatal(e)
			}
		}
		if data.Source.Checksum != worldService.Catalog.Source.Checksum {
			log.Fatal("dungeon/world source versions differ")
		}
		// 「哪些副本按权重掷骰选图」念出来（权重是我们改写过的，见 §41）。
		// 强制选图放在念完之后：日志先反映配置，再反映这次的诊断覆盖。
		logMazeChance(&data)
		if e = forceMaze(&data, os.Getenv("DFO_MAZE_FORCE")); e != nil {
			log.Fatal(e)
		}
		dungeonCatalog = &data
	}
	if *progressionCatalogFile != "" || pvfCatalogs.progression != nil {
		if characters == nil || dungeonCatalog == nil {
			log.Fatal("progression requires source characters and dungeon sessions")
		}
		data, e := pvfCatalogs.loadProgression(*progressionCatalogFile)
		if e != nil {
			log.Fatal(e)
		}
		rules, e := progression.LoadRules(*progressionRulesFile)
		if e != nil {
			log.Fatal(e)
		}
		if data.Source.Checksum != characters.Catalog.Source.Checksum || data.Source.Checksum != dungeonCatalog.Source.Checksum {
			log.Fatal("progression source version mismatch")
		}
		progressionService = &character.ProgressionService{Store: characters.Store, Catalog: data, Professions: characters.Catalog, Rules: rules}
		if path := os.Getenv("DFO_ODYSSEY_GROWTH"); path != "" || pvfCatalogs.odysseyGrowth != nil {
			progressionService.Odyssey, e = pvfCatalogs.loadOdysseyGrowth(path)
			if e != nil {
				log.Fatal(e)
			}
		}
		if path := os.Getenv("DFO_ODYSSEY_CHAPTERS"); path != "" || pvfCatalogs.odysseyChapters != nil {
			progressionService.Chapters, e = pvfCatalogs.loadOdysseyChapters(path)
			if e != nil {
				log.Fatal(e)
			}
		}
		if e = pvfCatalogs.bindOdysseyRoutes(progressionService); e != nil {
			log.Fatal(e)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = characters.Store.MigrateCharacterEvents(ctx)
		if e == nil {
			e = characters.Store.MigrateCharacterNotices(ctx)
		}
		if e == nil {
			e = characters.Store.MigrateSkillLocks(ctx)
		}
		if e == nil {
			e = characters.Store.MigrateTowerProgress(ctx)
		}
		cancel()
		if e != nil {
			log.Fatal(e)
		}
	}
	var tutorialRoutes *catalog.TutorialCatalog
	var tutorialDungeons *catalog.DungeonCatalog
	if *tutorialRoutesFile != "" {
		if dungeonCatalog == nil || characters == nil {
			log.Fatal("starting routes require source dungeons and persisted characters")
		}
		if *tutorialDungeonsFile == "" && pvfCatalogs.tutorialDungeons == nil {
			log.Fatal("starting routes require their own dungeon catalog")
		}
		routes, e := pvfCatalogs.loadTutorialRoutes(*tutorialRoutesFile, characters.Catalog.Source.Checksum)
		if e != nil {
			log.Fatal(e)
		}
		data, e := pvfCatalogs.loadTutorialDungeons(*tutorialDungeonsFile)
		if e != nil {
			log.Fatal(e)
		}
		if data.Source.Checksum != characters.Catalog.Source.Checksum {
			log.Fatal("starting-route dungeon source version differs")
		}
		tutorialRoutes, tutorialDungeons = routes, &data
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		e = characters.Store.MigrateBirth(ctx)
		cancel()
		if e != nil {
			log.Fatal(e)
		}
	}
	if *lootCatalogFile != "" {
		if progressionService == nil {
			log.Fatal("loot requires progression and owned dungeon sessions")
		}
		lootPath := *lootCatalogFile
		if path := os.Getenv("DFO_LOOT_CATALOG"); path != "" {
			lootPath = path
		}
		if err := pvfCatalogs.loadEnhancements(filepath.Dir(lootPath)); err != nil {
			log.Fatal(err)
		}
		// 锻造（CMD430 / Refine）的武器限制、成功率表与材料消耗。
		// 成功率由服主提供（115 版本），材料消耗 PVF 无表、走配置默认值。
		if err := inventory.LoadRefineRules(filepath.Join(filepath.Dir(lootPath), "refine.json")); err != nil {
			log.Fatal(err)
		}
		// 物品脚本自带的 [need material]（商店表 itemshop/**.shp 没有价格字段）：
		// 商店里「用材料交换」的商品，材料成本只写在物品脚本里（3242=1000×3037 等）。
		itemMaterials, matErr := pvfCatalogs.loadItemMaterials(filepath.Join(filepath.Dir(lootPath), "item-materials.json"))
		if matErr != nil {
			log.Fatal(matErr)
		}
		c, e := pvfCatalogs.loadLoot(lootPath)
		if e != nil {
			log.Fatal(e)
		}
		r, e := loot.LoadRules(*lootRulesFile)
		if e != nil {
			log.Fatal(e)
		}
		bag, e := inventory.LoadBagRules(*bagRulesFile)
		if e != nil {
			log.Fatal(e)
		}
		tables, e := loot.Parse(c)
		if e != nil {
			log.Fatal(e)
		}
		if c.Source.Checksum != characters.Catalog.Source.Checksum || bag.Source != c.Source.Checksum {
			log.Fatal("loot source mismatch")
		}
		if *equipmentCatalogFile == "" {
			log.Fatal("loot requires -equipment-catalog or DFO_EQUIPMENT_CATALOG: without it every equipment award is silently dropped")
		}
		gear, e := pvfCatalogs.loadEquipmentSelection(*equipmentCatalogFile, c.Source.Checksum)
		if e != nil {
			log.Fatal(e)
		}
		log.Printf("loaded equipment catalog: %d rows, %d droppable, from %s",
			len(gear.Rows), len(gear.DropPool()), *equipmentCatalogFile)
		dropCatalog := c
		itemIndexPath := *itemIndexFile
		if itemIndexPath == "" {
			cand := filepath.Join(filepath.Dir(lootPath), "items.index.json")
			if _, err := os.Stat(cand); err == nil {
				itemIndexPath = cand
			} else if _, err := os.Stat("configs/items.index.json"); err == nil {
				itemIndexPath = "configs/items.index.json"
			}
		}
		if itemIndexPath != "" || pvfCatalogs.items != nil {
			if err := pvfCatalogs.supplementStackables(&c, itemIndexPath); err != nil {
				if pvfCatalogs.items != nil {
					log.Fatal(err)
				}
				log.Printf("warning: supplement stackables from %s: %v", itemIndexPath, err)
			} else {
				log.Printf("supplemented stackable catalog from %s (total items: %d)", itemIndexPath, len(c.Items))
			}
		}
		if *equipmentJournalRulesFile != "" || pvfCatalogs.journal != nil {
			jr, e := pvfCatalogs.loadEquipmentJournal(*equipmentJournalRulesFile, c.Source.Checksum)
			if e != nil {
				log.Fatal(e)
			}
			journalRules = &jr
			log.Printf("loaded equipment journal rules: max=%d limits=%d categories=%d groups=%d/%d",
				jr.Maximum, len(jr.MaximumByType), len(jr.Categories), len(jr.WeaponGroups), len(jr.PeculiarGroups))
		}
		if *equipmentCreateCostFile != "" || pvfCatalogs.createCost != nil {
			cc, e := pvfCatalogs.loadEquipmentCreateCost(*equipmentCreateCostFile, c.Source.Checksum)
			if e != nil {
				log.Fatal(e)
			}
			equipmentCreateCost = &cc
			items := 0
			for _, g := range cc.Groups {
				items += len(g.Items)
			}
			log.Printf("loaded equipment create cost: groups=%d itemRows=%d templates=%d",
				len(cc.Groups), items, len(cc.Templates()))
		}
		lootService = &loot.Service{Store: characters.Store, Catalog: c, DropCatalog: dropCatalog, Rules: r, BagRules: bag, Tables: tables, Equipment: gear, Journal: journalRules, CreateCost: equipmentCreateCost, ItemMaterials: itemMaterials}
		minePath := *bleedingMineRewardsFile
		if minePath == "" {
			minePath = filepath.Join(filepath.Dir(lootPath), "bleeding-mine-rewards.json")
		}
		if _, err := os.Stat(minePath); err == nil || pvfCatalogs.mine != nil {
			mine, err := pvfCatalogs.loadMine(minePath)
			if err != nil {
				log.Fatal(err)
			}
			if mine.Source != c.Source.Checksum {
				log.Fatal("赤红铁矿奖励表与当前角色配置版本不一致")
			}
			lootService.BleedingMine = mine
		} else if *bleedingMineRewardsFile != "" {
			log.Fatal(err)
		}
		pricesPath := *shopPricesFile
		if pricesPath == "" {
			pricesPath = filepath.Join(filepath.Dir(lootPath), "shop-prices.json")
		}
		if _, err := os.Stat(pricesPath); err == nil || *shopPricesFile != "" || pvfCatalogs.prices != nil {
			lootService.Prices, e = pvfCatalogs.loadShopPrices(pricesPath, c.Source.Checksum)
			if e != nil {
				log.Fatal(e)
			}
			log.Printf("loaded %d source NPC prices from %s", len(lootService.Prices.Items), pricesPath)
		} else {
			log.Printf("warning: no source NPC prices (%s); gold purchases and sales are refused", pricesPath)
		}
		if path := os.Getenv("DFO_ODYSSEY_COIN_RULES"); path != "" || pvfCatalogs.odysseyCurrency != nil {
			lootService.Currency, e = pvfCatalogs.loadOdysseyCurrency(path)
			if e != nil {
				log.Fatal(e)
			}
		}
		cards, e := loot.LoadCardRules(*cardRulesFile)
		if e != nil {
			log.Fatal(e)
		}
		lootService.CardPolicy = &cards
		// Open-box tables are optional: without them a box is still consumed, it
		// just cannot hand out a prize. The launcher passes every config path
		// absolutely, so an unset -boxes resolves beside the bag rules rather
		// than against the working directory, which is not the project directory.
		boxesPath := *boxesFile
		if boxesPath == "" {
			boxesPath = filepath.Join(filepath.Dir(*bagRulesFile), "boxes.json")
		}
		boxesAvailable := pvfCatalogs.boxes != nil
		if !boxesAvailable {
			_, statErr := os.Stat(boxesPath)
			boxesAvailable = statErr == nil
		}
		if boxesAvailable {
			boxes, boxErr := pvfCatalogs.loadBoxes(boxesPath, lootService.Catalog.Source.Checksum)
			if boxErr != nil {
				log.Fatal(boxErr)
			}
			lootService.Boxes = boxes
			log.Printf("PVF boxes: %d tables, %d prize templates", boxes.TableCount(), boxes.RewardCount())
		} else if *boxesFile != "" {
			log.Fatal("boxes file missing: " + boxesPath)
		} else {
			log.Printf("boxes: %s absent, open-box prizes disabled", boxesPath)
		}
	}
	responses := map[uint16][]byte{}
	if *questCatalogFile != "" || pvfCatalogs.quests != nil {
		if worldService == nil {
			log.Fatal("quests require world character sessions")
		}
		data, e := pvfCatalogs.loadQuests(*questCatalogFile)
		if e != nil {
			log.Fatal(e)
		}
		if data.Source.Checksum != characters.Catalog.Source.Checksum {
			log.Fatal("quest/character source versions differ")
		}
		var odysseyGrowth *catalog.OdysseyGrowth
		if progressionService != nil {
			odysseyGrowth = progressionService.Odyssey
		}
		questService = &quest.Service{Store: characters.Store, Catalog: data, Professions: characters.Catalog, Progression: progressionService, Odyssey: odysseyGrowth, Dungeons: dungeonCatalog}
		var sceneIssues []string
		townArrivalScenes, sceneIssues = catalog.TownArrivalSceneWhitelist(data, worldService.Catalog)
		for _, issue := range sceneIssues {
			log.Printf("town arrival scene excluded: %s", issue)
		}
		log.Printf("PVF town arrival scene whitelist: %d entries", len(townArrivalScenes))
		if *equipmentRewardFile != "" {
			if lootService == nil {
				log.Fatal("quest inventory requires the shared bag catalog")
			}
			equipment, e := pvfCatalogs.loadEquipmentSelection(*equipmentRewardFile, data.Source.Checksum)
			if e != nil {
				log.Fatal(e)
			}
			questService.Inventory = &inventory.Awarder{Catalog: lootService.Catalog, Rules: lootService.BagRules, Equipment: equipment}
			if *wearRulesFile != "" {
				rulesPath := *wearRulesFile
				if override := os.Getenv("DFO_EQUIPMENT_WEAR_RULES"); override != "" {
					rulesPath = override
				}
				rules, err := inventory.LoadWearRules(rulesPath, data.Source.Checksum)
				if err != nil {
					log.Fatal(err)
				}
				wearService = &inventory.WearService{Store: characters.Store, Catalog: equipment, Professions: characters.Catalog, BagRules: lootService.BagRules, Rules: rules}

				// 装备变换要用「部位 → 装备类型」映射去**背包**里找源（客户端允许把背包装备放进
				// 界面「变换前」槽，请求只带部位码），所以把同一份 WearRules 也交给 loot 服务。
				lootService.WearRules = rules

				if *knightShieldFile != "" {
					shieldPath := knightShieldCatalogPath(*knightShieldFile, rulesPath)
					shields, shieldErr := pvfCatalogs.loadShields(shieldPath, data.Source.Checksum)
					if shieldErr != nil && !errors.Is(shieldErr, os.ErrNotExist) {
						log.Fatal(shieldErr)
					}
					wearService.Shields = shields
					if shields == nil {
						log.Printf("knight shield window disabled: catalog absent at %s", shieldPath)
					} else {
						log.Printf("knight shield window enabled: %d source-verified shields from %s", len(shields.Rows), shieldPath)
					}
				}

				// 创建期的初始装备投影共用同一份装备目录与部位槽映射，避免另立编号。
				characters.Equipment = equipment
				characters.WearRules = rules
				if *fullEquipmentFile != "" || pvfCatalogs.equipment != nil {
					full, err := pvfCatalogs.openFullEquipment(*fullEquipmentFile, data.Source.Checksum)
					if err != nil {
						log.Fatal(err)
					}
					defer full.Close()
					wearCatalog := *equipment
					wearCatalog.Full = full
					wearService.Catalog = &wearCatalog
					equipment.Full = full
					// [ALIGN-20260930-DURABILITY] 装备的**耐久上限**（源 `.equ` 的 `[durability]`）。
					// 落库前用它 clamp 超出上限的耐久：实机 2026-09-30 存档里出现过 `100/48`
					// 的武器（上限 48），客户端判定该装备非法 ⇒ 表现是"装备库登记不上 /
					// 分解点不动 / 装备变换界面卡死"。堵在写入端最彻底。
					inventory.SetDurabilityLimit(func(template uint32) (uint16, bool) {
						d, err := equipment.Definition(template)
						if err != nil {
							return 0, false
						}
						v, ok := d.Fields["[durability]"]
						if !ok || len(v) == 0 || v[0].Type != 0 || v[0].Value < 0 {
							return 0, false
						}
						return uint16(v[0].Value), true
					})
					// 宠物行的期限（181 字节行的偏移 56）要按脚本真值给「剩余秒数」：
					// 客户端把它 ÷86400 渲染成「过期时间:N天」，填哨兵值会显示 24856 天。
					inventory.SetCreaturePeriodSource(func(template uint32) (int32, bool) {
						d, err := equipment.Definition(template)
						if err != nil {
							return 0, false
						}
						v, ok := d.Fields["[usable period]"]
						if !ok || len(v) == 0 || v[0].Type != 0 {
							return 0, false
						}
						return v[0].Value, true
					})
					log.Printf("separate wear catalog: %d records; original reward/drop catalog: %d", len(full.Records), len(equipment.Rows))
				}
			}
			// The same source equipment catalog backs quest rewards and
			// monster gear drops; a drop only offers what a bag accepts.
			lootService.Equipment = equipment
			// Magic-seal unsealing (CMD393) rolls from the current random
			// option tables and reads each item's [random option] flag from
			// the full equipment catalog; without the full definitions the
			// sealed state cannot be proven, so the command stays unanswered.
			if equipment.Full != nil && (*randomOptionFile != "" || pvfCatalogs.randomOptions != nil) {
				options, err := pvfCatalogs.loadRandomOptions(*randomOptionFile, data.Source.Checksum)
				if err != nil {
					log.Fatal(err)
				}
				unsealService = &inventory.UnsealService{Store: characters.Store, Equipment: equipment, RandomOptions: options, Model: "current115-randomoption-v1"}
				log.Printf("magic-seal unsealing enabled: %d option groups", options.GroupCount())
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = characters.Store.MigrateQuests(ctx)
		if e == nil {
			e = characters.Store.MigrateQuestObjectives(ctx)
		}
		if e == nil && progressionService != nil {
			e = characters.Store.MigrateQuestRewards(ctx)
		}
		cancel()
		if e != nil {
			log.Fatal(e)
		}
	}
	if *vaultRulesFile != "" {
		if characters == nil {
			log.Fatal("vault initialization requires characters")
		}
		rules, e := pvfCatalogs.loadVaultRules(*vaultRulesFile)
		if e != nil {
			log.Fatal(e)
		}
		vaultService = &inventory.VaultService{Store: characters.Store, Rules: rules}
		if wearService != nil {
			vaultService.Equipment = wearService.Catalog
		}
		if *vaultPurchase || *vaultRelease || *shopRelease {
			for n := uint16(24); n <= 264; n += 16 {
				vaultService.Rules.VerifiedSlots = append(vaultService.Rules.VerifiedSlots, n)
			}
		}
		if lootService != nil {
			vaultService.Catalog = lootService.Catalog
			vaultService.BagRules = lootService.BagRules
			if shopPilot != nil {
				shopPilot.SetItemCatalog(lootService.Catalog.Items)
				vaultService.Catalog, e = shopPilot.StorageCatalog(vaultService.Catalog)
				if e != nil {
					log.Fatal(e)
				}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = characters.Store.MigrateVault(ctx)
		if e == nil {
			e = characters.Store.UpgradeSecondaryVaultCapacity(ctx)
		}
		if e == nil {
			e = characters.Store.MigrateAccountMaterials(ctx)
		}
		if e == nil && rules.Account != nil {
			e = characters.Store.MigrateAccountVault(ctx)
		}
		cancel()
		if e != nil {
			log.Fatal(e)
		}
	}
	var odysseyChoices odysseyWeaponChoices
	if lootService != nil && lootService.Currency != nil && vaultService != nil {
		vaultService.Catalog = lootService.Currency.StorageCatalog(vaultService.Catalog)
		vaultService.BagRules = lootService.Currency.BagRules(vaultService.BagRules)
	}
	if progressionService != nil && progressionService.Odyssey != nil && vaultService != nil {
		items := make(map[uint32]catalog.LootItem, len(vaultService.Catalog.Items)+3)
		for id, item := range vaultService.Catalog.Items {
			items[id] = item
		}
		for id, item := range character.OdysseyGiftCatalog(progressionService.Odyssey).Items {
			items[id] = item
		}
		vaultService.Catalog.Items = items
	}
	if odysseyRewardsEnabled() {
		var e error
		odysseyChoices, e = pvfCatalogs.loadOdysseyWeapons(os.Getenv("DFO_ODYSSEY_WEAPON_BOX"))
		if e != nil {
			log.Fatal(e)
		}
		if vaultService != nil {
			items := make(map[uint32]catalog.LootItem, len(vaultService.Catalog.Items)+1)
			for id, item := range vaultService.Catalog.Items {
				items[id] = item
			}
			items[10417789] = catalog.LootItem{ID: 10417789, Kind: "stackable", Grade: 1, Rarity: 2, StackableType: "[booster selection]", StackLimit: 1, Script: odysseyChoices.Definition}
			vaultService.Catalog.Items = items
		}
	}
	if path := os.Getenv("DFO_CLEAR_CUBE_SOURCE"); (path != "" || pvfCatalogs.clearCube != nil) && vaultService != nil {
		var e error
		vaultService.Catalog, e = pvfCatalogs.withClearCube(vaultService.Catalog, path)
		if e != nil {
			log.Fatal(e)
		}
	}
	var boosterCatalog *BoosterCatalog
	if *boosterCatalogFile != "" || *itemIndexFile != "" || pvfCatalogs.items != nil {
		var err error
		boosterCatalog, err = pvfCatalogs.loadBooster(*boosterCatalogFile, *itemIndexFile)
		if err != nil {
			log.Printf("warning: load booster catalog: %v", err)
		} else {
			log.Printf("loaded booster catalog (%d definitions, %d item index entries)", len(boosterCatalog.Definitions), len(boosterCatalog.Items))
		}
	}
	// 商城发货分类需要完整的物品索引：LootCatalog 只投影 stackable（装备投影
	// 被刻意拒绝），礼包就地展开开出装备时（实机 2026-09-26：称号进消耗品栏）
	// 兜底会把它当 [etc] 发进 Use 区。这里把索引里的 equipment/avatar/creature
	// 分类补进商城目录；已有条目不覆盖，堆叠物仍以 LootCatalog 为准。
	if shopPilot != nil && boosterCatalog != nil {
		kinds := make(map[uint32]cashshop.ItemInfo, len(boosterCatalog.Items))
		for id, it := range boosterCatalog.Items {
			if it.Kind == "equipment" || it.Kind == "avatar" {
				kinds[id] = cashshop.ItemInfo{ID: id, Kind: it.Kind, Path: it.Path}
			}
		}
		shopPilot.SupplementItemKinds(kinds)
		log.Printf("shop delivery: %d equipment/avatar templates classified from item index", len(kinds))
	}
	// 装备耐久与开盒/任务/掉落同一条规则（Catalog.Reward 读源 .equ）。
	if shopPilot != nil && wearService != nil && wearService.Catalog != nil {
		shopPilot.SetEquipmentDurability(func(id uint32) (uint16, error) {
			return wearService.Catalog.Reward(id)
		})
	}
	var lotteryPools *lotteryItemCatalog
	if boosterCatalog != nil && (*itemIndexFile != "" || pvfCatalogs.lotteryTables != nil) {
		lotteryPath := filepath.Join(filepath.Dir(*itemIndexFile), "lottery-item-pools.json")
		var err error
		lotteryPools, err = pvfCatalogs.loadLotteryItems(lotteryPath, boosterCatalog.Items)
		if err != nil {
			log.Printf("warning: lottery item catalog disabled: %v", err)
		} else {
			log.Printf("loaded lottery item catalog (%d verified pools)", len(lotteryPools.Pools))
			if wearService != nil && wearService.Catalog != nil {
				equipmentPath := filepath.Join(filepath.Dir(*itemIndexFile), "lottery-equipment-pools.json")
				if count, loadErr := pvfCatalogs.loadLotteryEquipment(equipmentPath, boosterCatalog.Items, lotteryPools); loadErr != nil {
					log.Printf("warning: equipment lottery pools disabled: %v", loadErr)
				} else {
					log.Printf("loaded equipment lottery pools (%d verified pools)", count)
				}
			}
		}
	}
	// Source selection boxes ([booster select category]) are deliberately absent
	// from the fixed-content booster catalog, so without this table every pick-a-
	// item box falls through to the random-pool branch and the client only ever
	// sees its generic "target inventory is full" notice.
	var selectionBoxes *catalog.SelectionBoxes
	if *selectionBoxFile != "" || pvfCatalogs.selectionBoxes != nil {
		var err error
		selectionBoxes, err = pvfCatalogs.loadSelectionBoxes(*selectionBoxFile)
		if err != nil {
			log.Printf("warning: load selection boxes (%s): %v", *selectionBoxFile, err)
		} else {
			log.Printf("loaded selection boxes (%d boxes, %d mislabeled fixed) from %s", len(selectionBoxes.Boxes), len(selectionBoxes.Fixed), *selectionBoxFile)
		}
	}
	if selectionBoxes == nil {
		log.Printf("warning: no selection box catalog; pick-a-item boxes go down the generic booster path")
	}
	// 末世录/军团编译表（apocalypse.ctp）。阶段时钟、四个作战、门禁时刻、投币开关、
	// 奖励与职责参数全部出自这张表；缺表时军团确认会记一条
	// `legion_catalog_missing` 事件，而不是假装校验通过。
	var apocalypseCatalog *catalog.ApocalypseCatalog
	var apocalypseClock *legion.ApocalypseClock
	if *apocalypseCatalogFile != "" || pvfCatalogs.apocalypse != nil {
		loaded, err := pvfCatalogs.loadApocalypse(*apocalypseCatalogFile)
		if err != nil {
			log.Printf("warning: load apocalypse catalog (%s): %v", *apocalypseCatalogFile, err)
		} else if clock, err := legion.NewApocalypseClock(loaded); err != nil {
			log.Printf("warning: apocalypse clock (%s): %v", *apocalypseCatalogFile, err)
		} else {
			apocalypseCatalog, apocalypseClock = loaded, clock
			log.Printf("loaded apocalypse table (%d records, %d operations, %d phases, %gs total) from %s",
				loaded.RecordCount, len(loaded.Operations), clock.Len(), clock.TotalSeconds(), *apocalypseCatalogFile)
		}
	}
	if apocalypseCatalog == nil {
		log.Printf("warning: no apocalypse catalog; legion operation confirmations are not validated")
	}
	// 物品商店表：源用 [need material] 定价的商品（奥德赛商店的盒子要 100 个银币）
	// 必须按材料扣，否则一律按写死的金币单价白送。
	var itemShops *catalog.ItemShops
	if *itemShopFile != "" || pvfCatalogs.itemShops != nil {
		var err error
		shopSource := ""
		if pvfCatalogs.itemShops != nil {
			shopSource = pvfCatalogs.itemShops.Source.Checksum
		}
		if lootService != nil {
			shopSource = lootService.Catalog.Source.Checksum
		}
		itemShops, err = pvfCatalogs.loadItemShops(*itemShopFile, shopSource)
		if err != nil {
			if pvfCatalogs.itemShops != nil {
				log.Fatal(err)
			}
			log.Printf("warning: load item shops (%s): %v", *itemShopFile, err)
		} else {
			log.Printf("loaded item shops (%d shops) from %s", len(itemShops.Shops), *itemShopFile)
		}
	}
	if itemShops == nil {
		log.Printf("warning: no item shop catalog; material-priced purchases cannot be resolved")
	} else if lootService != nil {
		lootService.ItemShops = itemShops
	}
	// 章节盒掉落（手册 P3 子项 3）。整表默认 enabled=false；只有 profile 显式开启
	// 才会叠加目录与槽位，未开启时连掷骰种子都不消耗。
	if path := os.Getenv("DFO_ODYSSEY_CHAPTER_DROP"); path != "" || pvfCatalogs.odysseyDrop != nil {
		chapterDrop, e := pvfCatalogs.loadOdysseyDrop(path)
		if e != nil {
			log.Fatal(e)
		}
		if e = chapterDrop.ValidateBoxes(selectionBoxes); e != nil {
			log.Fatal(e)
		}
		if lootService != nil {
			lootService.ChapterDrop = chapterDrop
			if chapterDrop.Enabled() {
				lootService.Catalog = chapterDrop.StorageCatalog(lootService.Catalog)
				lootService.BagRules = chapterDrop.BagRules(lootService.BagRules)
				log.Printf("Odyssey chapter drop enabled")
			}
		}
	}
	// 调律之边界（深渊）专属奖励表：按副本声明（[dungeon index]），取自源
	// rewardboostinfo CTP。奖励物全是 [booster] 礼盒，落袋走背包对未知 stackable
	// 类型的既有兜底槽位，开盒走既有的 booster 目录 —— 所以这里只校验、不覆盖
	// 任何目录条目。
	if *attunementRewardsFile != "" || pvfCatalogs.attunement != nil {
		attunement, e := pvfCatalogs.loadAttunement(*attunementRewardsFile)
		if e != nil {
			log.Fatal(e)
		}
		if lootService == nil {
			log.Fatal("attunement rewards need the loot service")
		}
		if e = attunement.ValidateTemplates(lootService.Catalog); e != nil {
			log.Fatal(e)
		}
		// 调参层在**源校验之后**才动手：先证明「表读对了」，再谈「我们想改哪里」。
		// ApplyRebalance 自己会复核权重不变量（每份 drop list 仍恰好 1e6），所以
		// 改完的表与源表在结构上同样合法。
		if _, _, e = attunement.ApplyRebalance(attunementRebalance); e != nil {
			log.Fatal(e)
		}
		logAttunementRebalance(attunement, attunementRebalance)
		// 展开一层要用的礼包目录。缺了它就只能把包装丢在地上，而那正是本功能要
		// 修的那个报告，所以这里硬失败而不是退化成旧行为。
		if boosterCatalog == nil || len(boosterCatalog.Definitions) == 0 {
			log.Fatal("attunement rewards need -booster-catalog: the table pays wrappers, and without the box catalog they cannot be opened at drop time")
		}
		boxes := boosterBoxSource{catalog: boosterCatalog}
		empties, unopenable, e := attunement.ValidateBoxes(boxes)
		if e != nil {
			log.Fatal(e)
		}
		if len(unopenable) > 0 {
			log.Printf("warning: attunement rewards name %d box(es) this build cannot open; they will not be paid: %v", len(unopenable), unopenable)
		}
		lootService.Attunement = attunement
		lootService.RewardBoxes = boxes
		// 征兆系统（omen）。默认关闭：它改变通关的产出，而首次实机验证还没做，
		// 所以打开它必须是显式的一步，而不是跟着奖励表悄悄上线。
		if *omenRewards {
			if e := attunement.ValidateOmen(); e != nil {
				log.Fatal(e)
			}
			lootService.Omen = loot.NewOmenLedger(attunement)
			for _, d := range attunement.Dungeons() {
				if n := attunement.OmenStagesCount(d); n > 0 {
					log.Printf("omen system on dungeon %d: %d stage(s), %d reward template(s)", d, n, len(attunement.OmenTemplates()))
				}
			}
		}
		// 空槽是源的合法面（CTP 用一个没有脚本的保留 id 表示"本次没有"），但
		// "本次没有"和"目录缺了这个物品"在这里长得一模一样，所以把它打出来。
		log.Printf("attunement reward wrappers open one layer; %d empty-face templates: %v", len(empties), empties)
		log.Printf("loaded attunement rewards (%d dungeons %v, %d reward templates) from %s",
			len(attunement.Dungeons()), attunement.Dungeons(), len(attunement.Templates()), *attunementRewardsFile)

		// 幸运事件（小幸运 ×15 / 大幸运 ×50）：它没有任何服务端代码 —— 两个档就落在
		// fixed 池里，倍数写在盒子的 pool 里。这里只是把它念出来，免得它一直是
		// 「看不见的活」。见 internal/loot/attunement_luck.go。
		if luck := attunement.LuckTemplates(); len(luck) > 0 {
			log.Printf("mystical fortune (luck) tiers are live: templates %v rolled straight out of the fixed pool", luck)
			for _, d := range attunement.Dungeons() {
				small, large := attunement.LuckWeights(d, 0)
				if small == 0 && large == 0 {
					continue
				}
				log.Printf("  dungeon %d maze 0 luck: small %d/%d (%.4f%%) · large %d/%d (%.4f%%)",
					d, small, 1000000, float64(small)/10000, large, 1000000, float64(large)/10000)
			}
		}
		// [coupon drop table] 就是征兆系统的阶段表（见 internal/loot/omen.go）。
		// 开关关着时把它明确打出来，让「导入了但没接线」保持可见，而不是让玩家
		// 以为那几行已经在出货。
		if n := attunement.Coupons(); n > 0 && !*omenRewards {
			log.Printf("attunement reward tables carry %d [coupon drop table] row(s) = omen stages; -omen-rewards is off, so no roll uses them", n)
		}
	} else {
		log.Printf("warning: no attunement reward table; boundary-of-attunement clears pay no exclusive reward")
	}
	if lootService != nil && boosterCatalog != nil && (*itemIndexFile != "" || pvfCatalogs.blackPurgatory != nil) {
		path := filepath.Join(filepath.Dir(*itemIndexFile), "black-purgatory-rewards.json")
		rewards, err := pvfCatalogs.loadBlackPurgatory(path, boosterBoxSource{catalog: boosterCatalog}, func(id uint32) (catalog.LootItem, bool) {
			item, ok := boosterCatalog.Items[id]
			return catalog.LootItem{ID: id, Kind: item.Kind, StackableType: item.StackableType, StackLimit: item.StackLimit, Script: catalog.ScriptRecord{Path: item.Path}}, ok
		})
		if err == nil {
			err = rewards.ValidateBossEquipment(lootService.Equipment)
		}
		if err == nil {
			lootService.Catalog, err = rewards.StorageCatalog(lootService.Catalog)
		}
		if err == nil && vaultService != nil {
			vaultService.Catalog, err = rewards.StorageCatalog(vaultService.Catalog)
		}
		if err != nil {
			log.Printf("黑鸦奖励加载失败，暂不允许开始挑战：%v", err)
		} else {
			lootService.BlackPurgatory = rewards
			log.Printf("已加载黑鸦小队翻牌及领主装备奖励；装备概率采用配置中的本服暂定规则")
		}
	}
	if *responseFile != "" {
		b, err := os.ReadFile(*responseFile)
		if err != nil {
			log.Fatal(err)
		}
		paths := map[uint16]string{}
		if err = json.Unmarshal(b, &paths); err != nil {
			log.Fatal(err)
		}
		for id, path := range paths {
			b, err := os.ReadFile(path)
			if err != nil {
				log.Fatal(err)
			}
			if err = wire.ValidateServer(b); err != nil {
				log.Fatal(err)
			}
			responses[id] = b
		}
	}
	if err := os.MkdirAll(*dir, 0700); err != nil {
		log.Fatal(err)
	}
	var raw []byte
	var err error
	if *fixture != "" {
		raw, err = os.ReadFile(*fixture)
		if err != nil {
			log.Fatalf("read fixture %q: %v", *fixture, err)
		}
		if err = wire.ValidateServer(raw); err != nil {
			log.Fatalf("validate fixture %q: %v", *fixture, err)
		}
	}
	hub := newLanHub()
	var moonConfig *moonSoloConfig
	if *moonConfigFile != "" {
		moonConfig, err = loadMoonSoloConfig(*moonConfigFile, lootService)
		if err != nil {
			log.Fatal(err)
		}
		if worldService == nil || characters == nil || dungeonCatalog == nil || *channelRefreshFile == "" || !*entryBasicProbe || !*entryAdditionProbe {
			log.Fatal("Moon requires complete persisted world/entry/dungeon/channel services")
		}
		if err = validateMoonResources(dungeonCatalog, lootService); err != nil {
			log.Fatal(err)
		}
		if err = worldService.ValidatePosition(255, false, moonConfig.Entry); err != nil {
			log.Fatal("Moon source entry: ", err)
		}
	}
	l, err := net.Listen("tcp4", *gameListen)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	advertised, err := advertisedGameAddress(*advertiseHost, gameHost, l.Addr())
	if err != nil {
		log.Fatal(err)
	}
	// One game port per channel. The client dials the port listed for the channel
	// it picked, and the game connection itself never carries a channel number,
	// so the port a client arrives on is the only way to tell channels apart.
	type channelListener struct {
		channel uint32
		ln      net.Listener
	}
	var listeners []channelListener
	var channelCfg channelrefresh.Config
	var endpoints map[uint32]channelrefresh.ChannelEndpoint
	// channelTypes maps a channel id to the Type of its directory row. The game
	// connection carries no channel number, so the port a client dialled is the
	// only channel identity a session has (see the listeners below); this map
	// turns that identity back into the script value handlers report.
	channelTypes := map[uint32]uint32{}
	if *channelRefreshFile != "" {
		channelCfg, err = channelrefresh.Load(*channelRefreshFile)
		if err != nil {
			log.Fatal(err)
		}
		for _, ch := range channelCfg.Channels {
			channelTypes[ch.ID] = ch.Type
		}
		if moonConfig != nil {
			found := false
			for _, ch := range channelCfg.Channels {
				if ch.ID == moonConfig.Channel {
					found = ch.Type == 101
				}
			}
			if !found {
				log.Fatal("Moon channel must exist with source online type 101")
			}
		}
		bindHost, _, _ := net.SplitHostPort(*gameListen)
		_, portText, _ := net.SplitHostPort(l.Addr().String())
		basePort, convErr := strconv.Atoi(portText)
		if convErr != nil {
			log.Fatal(convErr)
		}
		advHost, _, _ := net.SplitHostPort(advertised)
		endpoints = map[uint32]channelrefresh.ChannelEndpoint{}
		for i, ch := range channelCfg.Channels {
			ln := l
			if i > 0 {
				ln, err = net.Listen("tcp4", net.JoinHostPort(bindHost, strconv.Itoa(basePort+i)))
				if err != nil {
					log.Fatal(err)
				}
				defer ln.Close()
			}
			listeners = append(listeners, channelListener{channel: ch.ID, ln: ln})
			endpoints[ch.ID] = channelrefresh.ChannelEndpoint{ID: ch.ID, Host: advHost, Port: uint16(basePort + i)}
		}
	} else {
		listeners = append(listeners, channelListener{channel: 0, ln: l})
	}
	ready, _ := json.Marshal(map[string]any{"address": l.Addr().String(), "advertise": advertised, "pid": os.Getpid(), "fixture_bytes": len(raw), "channels": len(listeners)})
	if err = os.WriteFile(filepath.Join(*dir, "ready.json"), ready, 0600); err != nil {
		log.Fatal(err)
	}
	f, err := os.OpenFile(filepath.Join(*dir, "events.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var mu sync.Mutex
	event := func(v map[string]any) {
		mu.Lock()
		defer mu.Unlock()
		v["time"] = time.Now().UTC().Format(time.RFC3339Nano)
		if err := json.NewEncoder(f).Encode(v); err != nil {
			log.Print(err)
		}
	}
	fmt.Println(string(ready))
	if endpoints != nil {
		cl, e := channelrefresh.Serve(channelCfg, endpoints, event)
		if e != nil {
			log.Fatal(e)
		}
		defer cl.Close()
		event(map[string]any{"kind": "channel_refresh_ready", "address": cl.Addr().String(), "channels": len(endpoints)})
	}
	handleClient := func(c net.Conn, channel uint32) {
		// Every connection runs in its own goroutine and the server had no
		// recover() anywhere, so a single panic took the gateway process down
		// and dropped everyone at once (2026-09-23: the CMD 72 acknowledgement
		// index). Keep it on this connection, with the full stack in the log.
		peer := ""
		defer func() { recoverConnection(peer, channel, c, event) }()
		defer c.Close()
		peer = c.RemoteAddr().String()
		characters := characters // isolate context from simultaneous channel sessions
		var channelNotice []byte
		if *channelIdentityEnabled || channelCfg.SynchronizeIdentity {
			ctx, notice, identityErr := channelIdentity(channelCfg, channel)
			if identityErr != nil || characters == nil {
				event(map[string]any{"kind": "channel_identity_error", "error": fmt.Sprint(identityErr), "characters_present": characters != nil})
				return
			}
			localCharacters := *characters
			localCharacters.ChannelContext = ctx
			characters = &localCharacters
			channelNotice = notice
		}
		keys := make([]byte, wire.SessionKeyBytes)
		for i := range keys {
			keys[i] = byte(i%127 + 1)
		}
		bootstrapped := false
		// Per-command body sample counters for this connection.
		bodySamples := map[uint16]int{}
		var selectedCharacterID int64
		purchaseSession, sessionErr := newShopPilotSession()
		if sessionErr != nil {
			event(map[string]any{"kind": "shop_session_error", "error": sessionErr.Error()})
			return
		}
		purchaseSession.keys = keys
		if (*vaultPurchase || *vaultRelease || *shopRelease) && vaultService != nil {
			purchaseSession.vaultRules = &vaultService.Rules
		}
		var selectedBasic []byte
		var selectedAddition []byte
		var worldState *worldSession
		var comboState comboSkillSession
		var skillState skillSession
		var cubeContractState cubeContractSession
		var equipmentState equipmentSession
		var sortState sortSession
		var legionState legionSession
		legionState.catalog = apocalypseCatalog
		legionState.clock = apocalypseClock
		legionState.channelType = channelTypes[channel]
		if worldService != nil {
			if questService != nil && townArrivalScenes == nil {
				log.Fatal("town arrival scene whitelist was not passed to world sessions")
			}
			worldState = &worldSession{characters: characters, service: worldService, account: developmentAccount, flags: townPolicy.Flags, dungeons: dungeonCatalog, townArrivalScenes: townArrivalScenes, tutorials: tutorialRoutes, tutorialDungeons: tutorialDungeons, professions: characters.Catalog, fatigue: fatigueService, quests: questService, progression: progressionService, loot: lootService, selectionBoxes: selectionBoxes, vault: vaultService, skinCatalog: skinCatalog, soloPartyBootstrap: *soloPartyBootstrap, hub: hub, scaleDeathFromHP: *scaleDeathFromHP, oathGrades: oathGradePair, oathTable: oathGradeTable, oathFromGear: *oathFromGear, oathProgressClears: *oathProgressClears, oathProgressDungeons: oathProgressSet, oathInject: oathInjectSpecs, omenHold: *omenHold, omenState: *omenState, omenInfo: omenInfoBytes}
			worldState.serverID = channelCfg.ServerID
			worldState.channelType = channelTypes[channel]
			if moonConfig != nil && channel == moonConfig.Channel {
				worldState.moonConfig = moonConfig
			}
		}
		if worldState != nil {
			defer worldState.departArea()
		}
		var writeMu sync.Mutex
		sendPayload := func(kind byte, id uint16, payload []byte) error {
			writeMu.Lock()
			defer writeMu.Unlock()
			prepared, e := preparePackets(keys, []outboundPacket{{"response", kind, id, payload}})
			if e != nil {
				event(map[string]any{"kind": "response_encode_error", "peer": peer, "error": e.Error()})
				return e
			}
			c.SetWriteDeadline(time.Now().Add(5 * time.Second))
			e = writePackets(c, prepared, nil)
			if e != nil {
				event(map[string]any{"kind": "response_write_error", "peer": peer, "error": e.Error()})
			}
			return e
		}
		sendServerTime := func(reason string) error {
			now := time.Now()
			payload, err := protocol.ServerTimeSuccess(now)
			if err != nil {
				event(map[string]any{"kind": "server_time_error", "error": err.Error()})
				return err
			}
			if err = sendPayload(1, 1960, payload); err != nil {
				return err
			}
			event(map[string]any{"kind": "server_time_sent", "id": 1960, "reason": reason, "unix_seconds": now.Unix(), "plain_hex": hex.EncodeToString(payload)})
			return nil
		}
		sendRosterBackgrounds := func() error {
			if characters == nil {
				return nil
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			state, e := characters.Store.RosterBackgrounds(ctx, developmentAccount)
			if e != nil {
				event(map[string]any{"kind": "roster_background_restore_error", "error": e.Error()})
				return e
			}
			payload, e := protocol.RosterBackgroundRestore(state)
			if e != nil {
				return e
			}
			if e = sendPayload(0, 1759, payload); e != nil {
				return e
			}
			event(map[string]any{"kind": "roster_background_restored", "selected": state.Selected, "owned_count": len(state.Owned)})
			return nil
		}
		event(map[string]any{"kind": "accept", "peer": peer})
		c.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if _, err := io.Copy(c, bytes.NewReader(raw)); err != nil {
			event(map[string]any{"kind": "write_error", "error": err.Error()})
			return
		}
		event(map[string]any{"kind": "server_frame", "peer": peer, "hex": hex.EncodeToString(raw)})
		done := make(chan struct{})
		defer close(done)
		frames := clientFrames(c, done)
		mailChanges := make(chan struct{}, 1)
		// GM 为独立进程，无法调用本进程的 lanHub；定期从已提交邮件补齐提醒。
		mailTicker := time.NewTicker(2 * time.Second)
		defer mailTicker.Stop()
		var mailAlarmRole, mailDeliveryID int64
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		mineTicker := time.NewTicker(time.Second)
		defer mineTicker.Stop()
		var moonTicks <-chan time.Time
		if worldState != nil && worldState.moonConfig != nil {
			mt := time.NewTicker(250 * time.Millisecond)
			defer mt.Stop()
			moonTicks = mt.C
		}
		for {
			var incoming clientRead
			select {
			case incoming = <-frames:
			case now := <-mineTicker.C:
				if bootstrapped && selectedCharacterID != 0 && worldState != nil {
					cardPackets, cardErr := worldState.autoPickBlackPurgatoryCard(now)
					if cardErr != nil {
						event(map[string]any{"kind": "黑鸦自动翻牌待重试", "character_id": selectedCharacterID, "error": cardErr.Error()})
					}
					for _, packet := range cardPackets {
						if err := sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": selectedCharacterID, "plain_hex": hex.EncodeToString(packet.Payload)})
					}
					quotaPackets, quotaErr := worldState.refreshBlackPurgatoryQuota(now)
					if quotaErr != nil {
						event(map[string]any{"kind": "黑鸦次数同步失败", "error": quotaErr.Error()})
					}
					for _, packet := range quotaPackets {
						if err := sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload), "character_id": selectedCharacterID})
					}
					packets, err := worldState.bleedingMineTimeout(now)
					if err != nil {
						event(map[string]any{"kind": "赤红铁矿超时退出失败", "error": err.Error()})
					}
					for _, packet := range packets {
						if err := sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload), "character_id": selectedCharacterID})
					}
					// 超时只打开矿区失败选项，保留会话供结束探索或放弃处理。
					packets, err = worldState.blackPurgatoryTimeout(now)
					if err != nil {
						event(map[string]any{"kind": "黑鸦超时退出失败", "error": err.Error()})
					}
					for _, packet := range packets {
						if err := sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": selectedCharacterID})
					}
				}
				continue
			case now := <-moonTicks:
				if bootstrapped && selectedCharacterID != 0 {
					packets, e := worldState.moonTick(now)
					if e != nil {
						event(map[string]any{"kind": "moon_tick_error", "error": e.Error()})
					}
					for _, packet := range packets {
						if e := sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "id": packet.ID})
					}
				}
				continue
			case <-mailTicker.C:
				select {
				case mailChanges <- struct{}{}:
				default:
				}
				continue
			case <-mailChanges:
				if bootstrapped && selectedCharacterID != 0 && worldState != nil && worldState.characters != nil {
					adventureCtx, adventureCancel := context.WithTimeout(context.Background(), 5*time.Second)
					adventurePackets, adventureErr := worldState.refreshAdventure(adventureCtx)
					adventureCancel()
					if adventureErr != nil {
						event(map[string]any{"kind": "adventure_refresh_error", "reason": adventureErr.Error()})
					} else {
						for _, packet := range adventurePackets {
							if err := sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
								return
							}
							if packet.ID == 2799 {
								event(map[string]any{"kind": "season_level_synced", "character_id": selectedCharacterID, "attempt": "2/3（源阶段及领奖reader已核实）", "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
							}
						}
					}
					mailCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					alarm, latest, err := worldState.mailboxAlarm(mailCtx)
					cancel()
					if err != nil {
						event(map[string]any{"kind": "mailbox_alarm_error", "character_id": selectedCharacterID, "reason": err.Error()})
					} else if mailAlarmRole != selectedCharacterID || latest > mailDeliveryID {
						// 仅登录和真正的新投递发送 NOTI99，避免读信/领取后销毁详情对象。
						if err = sendPayload(0, 99, alarm); err != nil {
							return
						}
						mailAlarmRole, mailDeliveryID = selectedCharacterID, latest
						event(map[string]any{"kind": "mailbox_delivery_notified", "character_id": selectedCharacterID, "latest_mail_id": latest})
					}
				}
				continue
			case now := <-ticker.C:
				if bootstrapped && selectedCharacterID != 0 && worldState != nil {
					p, e := worldState.refreshDailyFatigue(now)
					if e != nil {
						event(map[string]any{"kind": "fatigue_daily_error", "error": e.Error()})
					}
					if e == nil && p != nil {
						if e = sendPayload(0, 36, p); e != nil {
							return
						}
						event(map[string]any{"kind": "fatigue_daily_refresh", "character_id": selectedCharacterID})
					}
					loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
					loyaltyPackets, loyaltyErr := worldState.refreshCreatureLoyalty(loyaltyCtx, now, worldState.activeDungeon != nil)
					loyaltyCancel()
					if loyaltyErr != nil {
						event(map[string]any{"kind": "creature_loyalty_error", "character_id": selectedCharacterID, "error": loyaltyErr.Error()})
					} else {
						for _, packet := range loyaltyPackets {
							if e := sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
								return
							}
						}
					}
				}
				continue
			}
			frame, err := incoming.frame, incoming.err
			if err != nil {
				event(map[string]any{"kind": "close", "peer": peer, "error": err.Error()})
				return
			}
			entry := map[string]any{"kind": "client_frame", "peer": peer, "type": frame.Type, "id": frame.ID, "bytes": len(frame.Raw)}
			var plaintext []byte
			verified := false
			// An implemented command always retains its body. Everything
			// else is sampled up to a per-command cap, so the wire format
			// of a feature this build does not implement is captured by
			// ordinary play rather than guessed at from another build.
			//
			// A parameterless command is a bare 13-byte header. The client
			// writes no fields, so there is no ciphertext at all; bodies are
			// padded to 16, so any command that carries data arrives as 29
			// bytes or more. command_layouts36.json lists 7 (leave game),
			// 42 and 69 (dungeon exit), 63, 67, 120 and 1301 (village
			// return) as body 0. Demanding 14 bytes before verifying made
			// every one of them fail the checksum gate unread, which is why
			// the escape menu did nothing: the frame is complete, its
			// payload is simply empty, and the checksum then covers only
			// the two header bytes.
			if len(frame.Raw) >= wire.ClientHeaderSize && retainRequestBody(frame.ID, bodySamples) {
				entry["hex"] = hex.EncodeToString(frame.Raw)
				if p, e := wire.DecryptPayload(keys, frame.ID, frame.Raw[13:]); e == nil {
					plaintext = p
					entry["plain_hex"] = hex.EncodeToString(p)
					verified = wire.Checksum(append(append([]byte{}, frame.Raw[11:13]...), p...)) == frame.Raw[7]
					entry["checksum_ok"] = verified
					if !observedGameRequest(frame.ID) {
						entry["unimplemented_sample"] = true
					}
				} else {
					entry["decode_error"] = e.Error()
				}
			}
			event(entry)
			if frame.Type == 1 && bootstrapped && verified && characters != nil && selectedCharacterID != 0 && frame.ID == 1462 {
				// 1402359F0发送无正文请求，实机20260929_005313已确认。
				if len(plaintext) != 0 {
					event(map[string]any{"kind": "账号角色资料请求被拒绝", "error": "请求正文应为空"})
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				payload, roster, err := characters.AllServerRoster(ctx, developmentAccount, fatigueService, time.Now())
				cancel()
				if err != nil {
					event(map[string]any{"kind": "账号角色资料读取失败", "error": err.Error()})
					continue
				}
				// 1444FCE40读取服务器数量u8及各服务器编号u8、角色数u16。
				count := len(roster)
				counts := []byte{1, characters.ChannelContext[0], byte(count), byte(count >> 8)}
				if err = sendPayload(0, 1396, counts); err != nil {
					return
				}
				if err = sendPayload(0, 2, payload); err != nil {
					return
				}
				if worldState != nil && worldState.channelType == 106 {
					worldState.bleedingMineRoster = roster
					// 名单索引与本次下发的账号列表一致，重开编队时也恢复已保存配置。
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					profile, err := worldState.bleedingMineProfile(ctx, roster)
					cancel()
					if err != nil {
						event(map[string]any{"kind": "赤红铁矿编队恢复失败", "error": err.Error()})
					} else if err = sendPayload(profile.Kind, profile.ID, profile.Payload); err != nil {
						return
					}
				}
				event(map[string]any{"kind": "账号编队角色资料已同步", "server": characters.ChannelContext[0], "characters": count, "plain_bytes": len(payload), "attempt": "1/3，CMD1462实机请求及原生读取链已核对"})
				continue
			}
			if frame.Type == 1 && bootstrapped && verified && worldState != nil && frame.ID == 2316 {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				packets, err := worldState.createBleedingMine(ctx, plaintext)
				cancel()
				if err != nil {
					event(map[string]any{"kind": "赤红铁矿创建失败", "id": frame.ID, "error": err.Error()})
					continue
				}
				for _, packet := range packets {
					if err := sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": worldState.role.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			if frame.Type == 1 && bootstrapped && verified && worldState != nil && frame.ID == 2317 {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				packets, err := worldState.saveBleedingMineTeam(ctx, plaintext)
				cancel()
				if err != nil {
					event(map[string]any{"kind": "赤红铁矿编队保存失败", "id": frame.ID, "error": err.Error()})
					// 14073D1E0将错误3映射到通用保存失败提示，不读额外正文。
					if err := sendPayload(1, 2317, []byte{0, 3, 0}); err != nil {
						return
					}
					continue
				}
				for _, packet := range packets {
					if err := sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": worldState.role.ID, "plain_hex": hex.EncodeToString(packet.Payload), "attempt": "1/3，保存请求与原生通知读取链已核对"})
				}
				continue
			}
			if frame.Type == 1 && bootstrapped && verified && worldState != nil && frame.ID == 2318 {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				start, plan, err := worldState.prepareBleedingMineStart(ctx, plaintext)
				cancel()
				if err != nil {
					// 14073D500失败分支解除确认框设置的输入锁。
					event(map[string]any{"kind": "赤红铁矿开战拒绝", "id": frame.ID, "error": err.Error()})
					if sendPayload(1, 2318, []byte{0, 3, 0}) != nil {
						return
					}
					continue
				}
				prepared, err := preparePackets(keys, plan)
				if err != nil {
					event(map[string]any{"kind": "赤红铁矿开战编码失败", "error": err.Error()})
					if sendPayload(1, 2318, []byte{0, 3, 0}) != nil {
						return
					}
					continue
				}
				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if err := writePackets(c, prepared, func(packet preparedPacket) {
					event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": worldState.role.ID,
						"plain_hex": hex.EncodeToString(packet.Payload)})
				}); err != nil {
					return
				}
				worldState.bleedingMineStart = start
				event(map[string]any{"kind": "赤红铁矿等待原生选图", "group": start.Group,
					"dungeon": start.Dungeon, "members": start.Members, "attempt": "1/3，原生开战状态与C15发送链已核对"})
				continue
			}
			if frame.Type == 1 && bootstrapped && verified && worldState != nil && selectedCharacterID != 0 {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				handled, packets, e := worldState.blackPurgatoryHandle(ctx, frame.ID, plaintext)
				cancel()
				if handled {
					if e != nil {
						event(map[string]any{"kind": "黑鸦请求被拒绝", "id": frame.ID, "error": e.Error()})
						packets = []outboundPacket{{"黑鸦请求拒绝应答", 1, frame.ID, protocol.Refusal(8)}}
					}
					prepared, err := preparePackets(keys, packets)
					if err != nil {
						event(map[string]any{"kind": "黑鸦响应编码失败", "error": err.Error()})
						return
					}
					c.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if err := writePackets(c, prepared, func(packet preparedPacket) {
						event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": selectedCharacterID,
							"plain_hex": hex.EncodeToString(packet.Payload)})
					}); err != nil {
						return
					}
					continue
				}
				handled, packets, e = worldState.moonHandle(frame.ID, plaintext, time.Now(), event)
				if handled {
					if e != nil {
						event(map[string]any{"kind": "moon_request_rejected", "id": frame.ID, "error": e.Error()})
						packets = moonRefusal(frame.ID, plaintext)
					}
					for _, packet := range packets {
						if e := sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "id": packet.ID})
					}
					continue
				}
			}
			if frame.Type == 1 && bootstrapped && verified && characters != nil && frame.ID == 63 {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				payload, e := ceraQuery(ctx, characters.Store, developmentAccount, plaintext)
				cancel()
				if e != nil {
					event(map[string]any{"kind": "cera_query_error", "error": e.Error()})
					continue
				}
				if e = sendPayload(0, 53, payload); e != nil {
					return
				}
				event(map[string]any{"kind": "cera_balance_response", "account_id": developmentAccount, "plain_hex": hex.EncodeToString(payload)})
				continue
			}
			if frame.Type == 1 && bootstrapped && verified && frame.ID == 64 {
				if shopPilot != nil && characters != nil && worldState != nil && selectedCharacterID != 0 && worldState.activeDungeon == nil {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					receipt, applied, buyErr := purchaseSession.purchase(ctx, shopPilot, characters.Store, developmentAccount, selectedCharacterID, plaintext, frame.Raw)
					cancel()
					if buyErr == nil {
						worldState.role.State = receipt.CharacterState
						ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
						balance, readErr := characters.Store.AccountCera(ctx, developmentAccount)
						cancel()
						if readErr != nil {
							event(map[string]any{"kind": "cera_committed_sync_error", "order": receipt.Order, "error": readErr.Error()})
							return
						}
						packets, encodeErr := shopPilotSpaces(shopPilot, receipt, balance, applied)
						if encodeErr != nil {
							event(map[string]any{"kind": "cera_committed_sync_error", "order": receipt.Order, "error": encodeErr.Error()})
							return
						}
						event(map[string]any{"kind": "cera_purchase_committed", "order": receipt.Order, "character_id": selectedCharacterID, "applied": applied, "charged": receipt.Charged, "before": receipt.Before, "after": receipt.After, "deliveries": receipt.Deliveries})
						for _, p := range packets {
							if err := sendPayload(p.Kind, p.ID, p.Payload); err != nil {
								return
							}
							event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
						}
						continue
					}
					event(map[string]any{"kind": "cera_purchase_rejected", "error": buyErr.Error(), "charged": false})
				}
				items, e := protocol.DecodeCeraCart(plaintext)
				reason := "delivery_protocol_pending"
				if e != nil {
					reason = e.Error()
				}
				// No ledger mutation occurs on this path. An unsupported buy
				// must finish its native pending state instead of hanging.
				payload := protocol.CeraPurchaseCancelled()
				if e = sendPayload(1, 64, payload); e != nil {
					return
				}
				event(map[string]any{"kind": "cera_purchase_cancelled", "reason": reason, "items": items, "character_id": selectedCharacterID, "charged": false, "plain_hex": hex.EncodeToString(payload)})
				continue
			}
			if frame.Type == 1 && bootstrapped && verified && worldState != nil && lootService != nil && lootService.Boxes != nil &&
				((frame.ID == 2036 && protocol.IsCeraShopDeviceAction(plaintext)) ||
					(frame.ID == 495 && protocol.IsCeraShopDeviceRefresh(plaintext))) {
				blob, stateErr := radiantDeviceWindowState(lootService, worldState.role)
				if stateErr != nil {
					event(map[string]any{"kind": "radiant_device_state_refused", "id": frame.ID, "character_id": selectedCharacterID, "reason": stateErr.Error()})
					continue
				}
				if e := sendPayload(1, 2036, blob); e != nil {
					return
				}
				event(map[string]any{"kind": "radiant_device_state_sent", "id": frame.ID, "character_id": selectedCharacterID, "plain_hex": hex.EncodeToString(blob)})
				continue
			}
			if frame.Type == 1 && bootstrapped && verified && frame.ID == 681 && worldState != nil && lootService != nil && lootService.Boxes != nil {
				request, decodeErr := protocol.DecodeRadiantBoxOpen(plaintext)
				if decodeErr != nil {
					// Other events share this opcode; they stay unanswered as
					// before instead of being answered with a guessed body.
					event(map[string]any{"kind": "event_request_ignored", "id": frame.ID, "reason": decodeErr.Error()})
					continue
				}
				count, countErr := radiantBoxOpens(request.Mode)
				box, boxErr := radiantBoxHeld(lootService, worldState.role)
				if countErr != nil || boxErr != nil {
					reason := countErr
					if reason == nil {
						reason = boxErr
					}
					event(map[string]any{"kind": "event_request_refused", "character_id": selectedCharacterID, "reason": reason.Error()})
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				packets, openErr := worldState.openRadiantBox(ctx, box, count)
				cancel()
				if openErr != nil {
					event(map[string]any{"kind": "event_request_refused", "character_id": selectedCharacterID, "box": box, "mode": request.Mode, "reason": openErr.Error()})
					continue
				}
				event(map[string]any{"kind": "radiant_box_opened", "character_id": selectedCharacterID, "box": box, "mode": request.Mode, "opened": count})
				for _, p := range packets {
					if err := sendPayload(p.Kind, p.ID, p.Payload); err != nil {
						return
					}
					event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
				}
				continue
			}
			if frame.Type == 1 && bootstrapped && verified && characters != nil && worldState != nil && (frame.ID == 102 || frame.ID == 173) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				packets, e := worldState.hatchCreature(ctx, characters.Store, frame.ID, plaintext, frame.Raw)
				cancel()
				if e != nil {
					event(map[string]any{"kind": "creature_hatch_error", "error": e.Error(), "character_id": selectedCharacterID})
					_ = sendPayload(1, frame.ID, []byte{0})
					continue
				}
				for _, p := range packets {
					if err := sendPayload(p.Kind, p.ID, p.Payload); err != nil {
						return
					}
					event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
				}
				event(map[string]any{"kind": "creature_hatch_success", "character_id": selectedCharacterID})
				continue
			}
			if frame.Type == 1 && (frame.ID == 160 || frame.ID == 41) && bootstrapped && verified && characters != nil && worldState != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				var plan []outboundPacket
				var e error
				if frame.ID == 41 {
					pilotEnabled := odysseyTemporaryCreditsEnabled() && isOdysseyRewardRole(worldState.role) && worldState.activeDungeon != nil && worldState.activeDungeon.Definition.Odyssey
					plan, e = worldState.useCoinRevive(ctx, characters.Store, plaintext, frame.Raw, pilotEnabled)
				} else if worldState.activeDungeon != nil || worldState.role.ID == 0 {
					e = fmt.Errorf("booster box use requires selected character in town")
				} else {
					plan, e = worldState.openBoosterItem(ctx, characters.Store, wearService, lootService, boosterCatalog, odysseyChoices, plaintext, frame.Raw)
				}
				cancel()
				if e != nil {
					event(map[string]any{"kind": "booster_action_refused", "id": frame.ID, "reason": e.Error()})
					plan = []outboundPacket{{"booster_action_refused_ack", 1, frame.ID, boosterActionRefusal(frame.ID)}}
				}
				for _, packet := range plan {
					if sendPayload(packet.Kind, packet.ID, packet.Payload) != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": selectedCharacterID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			if frame.Type == 1 && frame.ID == 27 && bootstrapped && verified && characters != nil && worldState != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				var plan []outboundPacket
				var e error
				if lotteryPools == nil || boosterCatalog == nil {
					e = fmt.Errorf("lottery item catalog unavailable")
				} else {
					plan, e = worldState.openLotteryItem(ctx, characters.Store, lotteryPools, boosterCatalog.Items, plaintext, frame.Raw, wearService)
				}
				cancel()
				if e != nil {
					event(map[string]any{"kind": "lottery_item_refused", "character_id": selectedCharacterID, "reason": e.Error()})
					plan = []outboundPacket{{"lottery_item_refused_ack", 1, 27, protocol.Refusal(4)}}
				}
				for _, packet := range plan {
					if sendPayload(packet.Kind, packet.ID, packet.Payload) != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": selectedCharacterID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			if frame.Type == 1 && frame.ID == 806 && bootstrapped && verified && worldState != nil {
				// CMD806 共用送礼(p[0]=0)与剧情角色染色(p[0]=1)。giveFavor
				// 内部按 p[0] 分流：送礼扣材料+加点数；染色回 0x01+请求体
				// 回显，客户端据此本地写角色颜色（不弹好感度窗）。
				plan, favorErr := worldState.giveFavor(plaintext)
				if favorErr != nil {
					event(map[string]any{"kind": "npc_favor_refused", "character_id": selectedCharacterID, "reason": favorErr.Error(), "plain_hex": hex.EncodeToString(plaintext)})
					if e := sendPayload(1, 806, protocol.Refusal(4)); e != nil {
						return
					}
					continue
				}
				for _, packet := range plan {
					if e := sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": selectedCharacterID, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			if frame.Type == 1 && frame.ID == 2079 && bootstrapped && verified && characters != nil {
				id, err := protocol.DecodeSynopsisRead(plaintext)
				var payload []byte
				if err == nil {
					payload, err = saveSynopsisRead(characters.Store, worldState, id)
				}
				if err != nil {
					event(map[string]any{"kind": "synopsis_read_refused", "error": err.Error()})
					continue
				}
				if err = sendPayload(0, 2310, payload); err != nil {
					return
				}
				event(map[string]any{"kind": "synopsis_table_info_sent", "character_id": worldState.role.ID, "synopsis_id": id, "attempt": "2/3", "plain_hex": hex.EncodeToString(payload)})
				continue
			}
			if frame.Type == 1 && frame.ID == 1438 && bootstrapped && verified && characters != nil {
				// CMD1438 STORY_DIGEST_UPDATE: the client reports the opening
				// recap movie finished (empty payload). Must be matched before
				// the 1417 branch: it shares the same request family, and a
				// later placement would let 1417 swallow the report so the
				// digest level never advances.
				if err := saveStoryDigest(characters.Store, worldState, plaintext); err != nil {
					event(map[string]any{"kind": "story_digest_save_error", "error": err.Error()})
				} else {
					event(map[string]any{"kind": "story_digest_saved", "character_id": worldState.role.ID, "level": worldState.level})
				}
				continue
			}
			if frame.Type == 1 && frame.ID == 1417 && bootstrapped && verified && characters != nil {
				if err := cinematicSkip(characters.Store, worldState, plaintext); err != nil {
					event(map[string]any{"kind": "cinematic_skip_refused", "error": err.Error()})
				} else {
					event(map[string]any{"kind": "cinematic_skip_saved", "character_id": worldState.role.ID})
				}
				continue
			}
			if frame.Type == 1 && frame.ID == 2177 && bootstrapped && verified && characters != nil {
				packets, err := awakenCharacter(characters, worldState, plaintext, keys)
				if err != nil {
					event(map[string]any{"kind": "awakening_refused", "error": err.Error()})
					if err = sendPayload(1, 2177, protocol.Refusal(4)); err != nil {
						return
					}
				} else if err = writePackets(c, packets, func(p preparedPacket) {
					event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
				}); err != nil {
					return
				}
				continue
			}
			if frame.Type == 1 && (frame.ID == 1881 || frame.ID == 777) && bootstrapped && verified && characters != nil {
				// 1881 = CHANGE_GROW_TYPE (首次转职), 777 = RE_GROWUP_CHANGE (随时更换职业).
				// 同一 grow-type 家族，请求体与响应格式一致，仅响应 opcode 不同。
				packets, err := changeGrowType(characters, worldState, plaintext, keys, frame.ID)
				if err != nil {
					event(map[string]any{"kind": "advancement_refused", "id": frame.ID, "error": err.Error()})
					if err = sendPayload(1, frame.ID, protocol.Refusal(advancementRefusalCode(err))); err != nil {
						return
					}
				} else if err = writePackets(c, packets, func(p preparedPacket) {
					event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
				}); err != nil {
					return
				}
				continue
			}
			if frame.Type == 1 && frame.ID == 451 && bootstrapped && verified && wearService != nil && wearService.Rules.Special {
				packets, err := avatarOption(wearService, worldState, plaintext, keys)
				if err != nil {
					event(map[string]any{"kind": "avatar_option_refused", "error": err.Error()})
					if err = sendPayload(1, 451, protocol.Refusal(4)); err != nil {
						return
					}
				} else if err = writePackets(c, packets, func(p preparedPacket) {
					event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
				}); err != nil {
					return
				}
				continue
			}
			if frame.Type == 1 && legion.Requests(frame.ID) && bootstrapped && verified && worldState != nil {
				// Legion / apocalypse family. CMD2043/2354/2045 are handled in
				// town and CMD2355 inside the dungeon; the rest of the family is
				// routed here so an unimplemented packet is logged as an
				// explicit refusal instead of vanishing.
				legionPlan, legionErr := legionState.handle(worldState, plaintext, frame.ID)
				// The request body is logged whether or not the opcode is
				// answered. Settling X1 (next64 §6.2) — whether the caller's
				// appended length already contains the 13-byte envelope — is
				// the point of P1's observability, so both the byte count and
				// the raw bytes are kept.
				legionBytes, legionHex := legionRequestBody(plaintext)
				if legionErr != nil {
					event(map[string]any{"kind": "legion_refused", "id": frame.ID, "reason": legionErr.Error(), "request_bytes": legionBytes, "request_hex": legionHex})
					continue
				}
				for _, note := range legionPlan.Events {
					note["id"] = frame.ID
					note["request_bytes"] = legionBytes
					note["request_hex"] = legionHex
					event(note)
				}
				for _, packet := range legionPlan.Packets {
					if err := sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": worldState.role.ID, "request_bytes": legionBytes, "request_hex": legionHex, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			if frame.Type == 1 && bootstrapped && verified && (frame.ID == 305 || frame.ID == 306) && worldState != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				plan, err := worldState.upgradeAccountVault(ctx, frame.ID, plaintext, frame.Raw, keys, purchaseSession.prefix)
				cancel()
				if err != nil {
					event(map[string]any{"kind": "账号金库操作被拒绝", "id": frame.ID, "character_id": selectedCharacterID, "reason": err.Error()})
					if err = sendPayload(1, frame.ID, accountVaultRefusal(err)); err != nil {
						return
					}
					continue
				}
				prepared, err := preparePackets(keys, plan)
				if err != nil {
					event(map[string]any{"kind": "账号金库回包编码失败", "reason": err.Error()})
					return
				}
				if err = writePackets(c, prepared, func(p preparedPacket) {
					event(map[string]any{"kind": p.Name, "id": p.ID, "character_id": selectedCharacterID, "plain_hex": hex.EncodeToString(p.Payload)})
				}); err != nil {
					return
				}
				continue
			}
			if frame.Type == 1 && bootstrapped && verified && (frame.ID == 307 || frame.ID == 308) && worldState != nil {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				plan, err := worldState.changeAccountVaultGold(ctx, frame.ID, plaintext, frame.Raw, keys, purchaseSession.prefix)
				cancel()
				if err != nil {
					event(map[string]any{"kind": "账号金库金币操作被拒", "id": frame.ID, "reason": err.Error(), "plain_hex": hex.EncodeToString(plaintext)})
					if sendPayload(1, frame.ID, accountVaultRefusal(err)) != nil {
						return
					}
					continue
				}
				prepared, err := preparePackets(keys, plan)
				if err != nil {
					return
				}
				if writePackets(c, prepared, func(p preparedPacket) {
					event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
				}) != nil {
					return
				}
				continue
			}
			// 装备技能栏 / 冷却提醒 / 自定义按键（C2S 2254/2256/2257）。
			// 请求侧的 96B 形状由本机实机帧证实（67 个会话各 1 帧 id=2256，全部 96B、[13]=10）。
			// 应答沿用同一作者在 C2S2382 上的约定：先落库，成功后再回 <同一 op> 的 1 字节 ack。
			if equipmentSkillEnabled() && frame.Type == 1 &&
				(frame.ID == 2254 || frame.ID == 2256 || frame.ID == 2257) &&
				bootstrapped && verified && worldState != nil && worldState.role.ID == selectedCharacterID {
				eskCtx, eskCancel := context.WithTimeout(context.Background(), 5*time.Second)
				plan, eskErr := worldState.equipmentSkillPackets(eskCtx, frame.ID, plaintext)
				eskCancel()
				if eskErr != nil {
					event(map[string]any{"kind": "equipment_skill_rejected", "id": frame.ID,
						"character_id": selectedCharacterID, "reason": eskErr.Error()})
					continue
				}
				for _, packet := range plan {
					if e := sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "id": packet.ID,
						"payload_bytes": len(packet.Payload)})
				}
				continue
			}
			if frame.Type == 1 && frame.ID == 2382 && bootstrapped && verified && worldState != nil && worldState.role.ID == selectedCharacterID {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				plan, oathErr := worldState.oathSelectionPackets(ctx, plaintext)
				cancel()
				if oathErr != nil {
					event(map[string]any{"kind": "oath_selection_rejected", "character_id": selectedCharacterID, "reason": oathErr.Error()})
					continue
				}
				for _, packet := range plan {
					if err = sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": selectedCharacterID, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			if frame.Type == 1 && frame.ID == 649 && bootstrapped && verified {
				var oldShield uint32
				if worldState != nil {
					if b, err := inventory.ReadBag(worldState.role.State); err == nil {
						oldShield = b.KnightDeck()[0]
					}
				}
				plan, deckErr := equipmentState.handleKnightDeck(wearService, worldState, plaintext, frame.Raw)
				event(knightShieldObservation(worldState, 649, plaintext, oldShield, deckErr))
				if sendErr := sendPayload(1, 649, protocol.KnightDeckAck()); sendErr != nil {
					return
				}
				event(map[string]any{"kind": "knight_deck_acknowledged", "type": 1, "id": 649, "payload_bytes": 3})
				if deckErr == nil {
					for _, packet := range plan {
						if sendErr := sendPayload(packet.Kind, packet.ID, packet.Payload); sendErr != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "type": packet.Kind, "id": packet.ID, "payload_bytes": len(packet.Payload), "plain_hex": hex.EncodeToString(packet.Payload)})
					}
				}
				continue
			}
			if frame.ID == 19 && bootstrapped && verified && wearService != nil {
				shieldRequest, shieldDecodeErr := protocol.DecodeItemMove(plaintext)
				shieldMove := shieldDecodeErr == nil && inventory.IsKnightShieldMove(shieldRequest)
				var oldShield uint32
				if shieldMove && worldState != nil {
					if b, err := inventory.ReadBag(worldState.role.State); err == nil {
						oldShield = b.KnightDeck()[0]
					}
				}
				plan, e := equipmentState.handle(wearService, worldState, plaintext, frame.Raw)
				if shieldMove {
					event(knightShieldObservation(worldState, 19, plaintext, oldShield, e))
				}
				if e != nil {
					event(map[string]any{"kind": "equipment_move_refused", "reason": e.Error()})
					r, _ := protocol.DecodeItemMove(plaintext)
					if e = sendPayload(1, 19, protocol.ItemMoveRefused(r, inventory.MoveRefusalCode(e))); e != nil {
						return
					}
					continue
				}
				r, decodeErr := protocol.DecodeItemMove(plaintext)
				var cloneRefresh []outboundPacket
				var cloneRefreshed bool
				if decodeErr == nil && len(plan) > 0 {
					cloneRefresh, cloneRefreshed, e = dungeonCloneEquipmentRefresh(worldState, r)
					if e != nil {
						event(map[string]any{"kind": "equipment_dungeon_clone_refresh_error", "error": e.Error()})
						cloneRefreshed = false
					}
				}
				// Mode-0 actor rebuilds can strand the next dungeon room request.
				// Only an actual creature-list move may use this separate refresh.
				if decodeErr == nil && characters != nil && moveNeedsCreatureActorAppearance(r) {
					var visual []byte
					visual, e = characters.EntryBasicProbe(worldState.role, [2]byte{})
					if e == nil {
						plan = append(plan, outboundPacket{"creature_actor_appearance_updated", 0, 2, visual})
					}
				}
				if decodeErr == nil && characters != nil && !cloneRefreshed && cloneAvatarRemoval(r, wearService.Catalog) {
					// attempt 3/3: entry's known mode-1 reader restores the ordinary
					// Avatar association on relog. Send it only after every CMD19
					// NOTI13/14 and mode-0 refresh, so later slot reconstruction
					// cannot immediately discard the restored association.
					addition, additionErr := characters.EntryAddition(worldState.role)
					if additionErr != nil {
						event(map[string]any{"kind": "equipment_avatar_addition_error", "error": additionErr.Error()})
					} else {
						plan = append(plan, outboundPacket{"equipment_avatar_addition_refreshed", 0, 2, addition})
					}
				}
				plan = worldState.appendFameUpdate(plan, event)
				if decodeErr == nil && characters != nil && worldState != nil &&
					((r.SourceList == 3 && r.SourceSlot == 47) || (r.DestinationList == 3 && r.DestinationSlot == 47)) {
					oathCtx, oathCancel := context.WithTimeout(context.Background(), 5*time.Second)
					selection, oathErr := characters.Store.EquippedOathSelection(oathCtx, developmentAccount, worldState.role.ID)
					oathCancel()
					if oathErr != nil {
						event(map[string]any{"kind": "oath_selection_refresh_error", "character_id": worldState.role.ID, "reason": oathErr.Error()})
					} else if info, infoErr := protocol.OathSystemInfo(selection.Level, selection.Option); infoErr == nil {
						plan = append(plan, outboundPacket{"oath_system_info_after_wear", 0, 2839, info})
					}
				}
				if cloneRefreshed {
					plan = append(plan, cloneRefresh...)
				}
				prepared, e := preparePackets(keys, plan)
				if e != nil {
					event(map[string]any{"kind": "equipment_encode_error", "error": e.Error()})
					return
				}
				if e = writePackets(c, prepared, func(p preparedPacket) {
					event(map[string]any{"kind": p.Name, "character_id": worldState.role.ID, "type": p.Kind, "id": p.ID, "payload_bytes": len(p.Payload), "plain_hex": hex.EncodeToString(p.Payload)})
				}); e != nil {
					event(map[string]any{"kind": "equipment_write_error", "error": e.Error()})
					return
				}
				continue
			}
			if frame.ID == 20 && bootstrapped && verified && worldState != nil && len(plaintext) > 0 {
				// CMD20 SORT_ITEM: the client has already arranged the bag and
				// asks the server to adopt it. Answering is also what clears the
				// client's "inventory in use" latch.
				var plan []outboundPacket
				var e error
				switch plaintext[0] {
				case 0:
					plan, e = sortState.handle(wearService, worldState, plaintext, frame.Raw)
				case 2, 45:
					plan, e = worldState.sortVaultSpace(plaintext[0])
				case 12:
					plan, e = worldState.sortAccountVaultCmd()
				default:
					event(map[string]any{"kind": "sort_unsupported_container", "space": plaintext[0]})
					continue
				}
				if e != nil {
					event(map[string]any{"kind": "item_sort_refused", "reason": e.Error()})
					if plaintext[0] != 0 {
						continue
					}
					if e = sendPayload(1, 20, protocol.Refusal(4)); e != nil {
						return
					}
					continue
				}
				prepared, e := preparePackets(keys, plan)
				if e != nil {
					event(map[string]any{"kind": "item_sort_encode_error", "error": e.Error()})
					return
				}
				if e = writePackets(c, prepared, func(p preparedPacket) {
					event(map[string]any{"kind": p.Name, "character_id": worldState.role.ID, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
				}); e != nil {
					event(map[string]any{"kind": "item_sort_write_error", "error": e.Error()})
					return
				}
				continue
			}
			if frame.Type != 1 {
				event(map[string]any{"kind": "unsupported_client_type", "type": frame.Type})
				continue
			}
			if bootstrapped && frame.ID == 1960 {
				if !verified {
					event(map[string]any{"kind": "server_time_request_rejected", "reason": "服务器时间请求校验失败"})
					continue
				}
				if err := protocol.DecodeServerTimeRequest(plaintext); err != nil {
					event(map[string]any{"kind": "server_time_request_rejected", "reason": err.Error()})
					continue
				}
				if err := sendServerTime("客户端请求"); err != nil {
					return
				}
				continue
			}
			if bootstrapped && frame.ID == 1395 {
				if !verified {
					event(map[string]any{"kind": "adventure_request_rejected", "reason": "冒险团请求校验失败"})
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				payload, err := worldState.handleAdventure(ctx, selectedCharacterID, plaintext)
				cancel()
				if err != nil {
					event(map[string]any{"kind": "adventure_request_rejected", "character_id": selectedCharacterID, "reason": err.Error()})
					// 原生处理器不检查成功参数，不能给它发送仅三字节的
					// 通用拒绝体，否则它仍会读取完整详情并越界。
					continue
				}
				if err := sendPayload(1, frame.ID, payload); err != nil {
					return
				}
				event(map[string]any{"kind": "adventure_info_sent", "character_id": selectedCharacterID, "id": frame.ID, "attempt": "4（CMD217与原生包尾读取链已核实）", "plain_hex": hex.EncodeToString(payload)})
				continue
			}
			if bootstrapped && selectedCharacterID != 0 && (frame.ID == 1406 || frame.ID == 2331 || frame.ID == 1719 || frame.ID == 1811 || frame.ID == 2419 || frame.ID == 2405 || frame.ID == 2139) {
				if !verified {
					event(map[string]any{"kind": "adventure_request_rejected", "id": frame.ID, "reason": "冒险团命令校验失败"})
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				var packets []outboundPacket
				var err error
				switch frame.ID {
				case 2139:
					event(map[string]any{"kind": "图鉴引导登记请求", "attempt": "2/3（按原生CMD33更新单任务进度）", "character_id": selectedCharacterID})
					packets, err = worldState.registerAdventureCollection(ctx, plaintext, frame.Raw, purchaseSession.prefix)
				case 2419:
					packets, err = worldState.claimSeasonReward(ctx, plaintext)
				case 2405:
					packets, err = worldState.acquireSeasonOath(ctx, plaintext, frame.Raw, purchaseSession.prefix)
				case 2331:
					packets, err = worldState.setAdventureBestHonor(ctx, plaintext, frame.Raw, purchaseSession.prefix)
				case 1719:
					packets, err = worldState.setAdventureElite(ctx, plaintext, frame.Raw, purchaseSession.prefix)
				case 1811:
					packets, err = worldState.loadAdventureElite(ctx, plaintext)
				default:
					packets, err = worldState.buyAdventureItem(ctx, plaintext, frame.Raw, purchaseSession.prefix)
				}
				cancel()
				if err != nil {
					event(map[string]any{"kind": "adventure_request_rejected", "id": frame.ID, "reason": err.Error()})
					if err = sendPayload(1, frame.ID, adventureFailure(frame.ID, err)); err != nil {
						return
					}
					continue
				}
				for _, packet := range packets {
					if err = sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": selectedCharacterID})
				}
				continue
			}
			if bootstrapped && mailboxRequest(frame.ID) {
				if !verified {
					event(map[string]any{"kind": "mailbox_request_rejected", "id": frame.ID, "reason": "邮箱请求校验失败"})
					continue
				}
				mailCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				packets, recipient, err := worldState.handleMailbox(mailCtx, selectedCharacterID, frame.ID, plaintext, frame.Raw, keys, purchaseSession.prefix)
				cancel()
				if err != nil {
					event(map[string]any{"kind": "mailbox_request_rejected", "id": frame.ID, "character_id": selectedCharacterID, "reason": err.Error()})
					// CMD781 的原生完成路径是 NOTI705；失败只记录，不猜测命令应答。
					if frame.ID == 781 {
						continue
					}
					if err := sendPayload(1, frame.ID, mailboxFailure(frame.ID, err)); err != nil {
						return
					}
					continue
				}
				// 投递已提交，即使发件人连接随后断开，收件人仍应收到通知。
				if recipient != 0 && hub != nil {
					hub.notifyMailbox(recipient)
				}
				for _, packet := range packets {
					if err := sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": selectedCharacterID, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				// CMD95/134 原生回调会更新附件与已读/删除状态，不再发送 NOTI99。
				// NOTI99 会重新请求 CMD96；NOTI97 全量恢复经 0x145FCF970 销毁旧
				// 邮件对象，而详情领取路径 0x145FD6A50 仍会访问已选对象。
				// 保留登录和真正新投递的提醒，避免读信后刷新造成悬空引用。
				continue
			}
			if bootstrapped && frame.ID == 2261 {
				if !verified {
					event(map[string]any{"kind": "special_warp_rejected", "reason": "checksum failed"})
					continue
				}
				plan, err := worldState.prepareSpecialWarp(plaintext)
				if err != nil {
					event(map[string]any{"kind": "special_warp_rejected", "reason": err.Error()})
					continue
				}
				for _, packet := range plan {
					if err := sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": selectedCharacterID, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}

			if bootstrapped && frame.ID == 2285 {
				if !verified {
					event(map[string]any{"kind": "exit_dialog_rejected", "reason": "checksum failed"})
					continue
				}
				if len(plaintext) != 0 {
					event(map[string]any{"kind": "exit_dialog_rejected", "reason": "unexpected request body", "bytes": len(plaintext)})
					continue
				}
				// CMD2285 is CONTENT_BRIEFING, issued by the in-game menu before
				// its local Exit path. Its callback at 145250c80 consumes a
				// mandatory 0x80-byte structure after the common success header.
				// A header-only reply triggers CMD217 (CMDPACKET_OVERFLOW_INFO).
				if err := sendPayload(1, frame.ID, protocol.ExitDialogReady()); err != nil {
					return
				}
				event(map[string]any{"kind": "exit_dialog_ready", "id": frame.ID, "character_id": selectedCharacterID})
				continue
			}
			if bootstrapped && frame.ID == 682 {
				if !verified {
					event(map[string]any{"kind": "exit_shutdown_rejected", "reason": "checksum failed"})
					continue
				}
				fastExit, e := protocol.DecodeExitShutdownSignal(plaintext)
				if e != nil {
					event(map[string]any{"kind": "exit_shutdown_rejected", "reason": e.Error(), "bytes": len(plaintext)})
					continue
				}
				event(map[string]any{"kind": "exit_shutdown_signal", "id": frame.ID, "character_id": selectedCharacterID, "fast": fastExit})
				selectedCharacterID = 0
				selectedBasic, selectedAddition = nil, nil
				if worldState != nil {
					worldState.departArea()
				}
				clearSelectedWorld(worldState)
				bootstrapped = false
				event(map[string]any{"kind": "exit_session_closed", "peer": peer})
				return
			}

			if bootstrapped && frame.ID == 2377 {
				if !verified {
					event(map[string]any{"kind": "unified_option_rejected", "reason": "checksum failed"})
					continue
				}
				// CMD2377 SET_UNIFIED_OPTION carries one option block per frame:
				// subtype 0x13 is the skill lock, 0x12 character effects, 0x05
				// ordinary settings, and 0x01 the account block restored by NOTI2826.
				opt, e := protocol.DecodeUnifiedOption(plaintext)
				if e != nil {
					event(map[string]any{"kind": "unified_option_rejected", "reason": e.Error(), "bytes": len(plaintext)})
					continue
				}
				event(map[string]any{"kind": "unified_option_accepted", "character_id": selectedCharacterID, "scope": opt.Scope, "subtype": opt.Subtype, "entries": len(opt.Entries)})
				switch opt.Subtype {
				case protocol.UnifiedOptionSkillLock:
					if characters == nil || worldState == nil || worldState.role.ID == 0 || worldState.role.ID != selectedCharacterID {
						event(map[string]any{"kind": "skill_lock_rejected", "reason": "skill lock requires the owned selected character", "character_id": selectedCharacterID})
						continue
					}
					sum := sha256.Sum256(plaintext)
					key := fmt.Sprintf("skill-lock-v1:%x", sum)
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					locks, applied, e := characters.SaveSkillLocks(ctx, worldState.role, key, opt)
					cancel()
					if e != nil {
						event(map[string]any{"kind": "skill_lock_rejected", "reason": e.Error(), "character_id": selectedCharacterID})
						continue
					}
					event(map[string]any{"kind": "skill_lock_saved", "character_id": selectedCharacterID, "applied": applied, "count": len(locks), "locks": locks})
				case protocol.UnifiedOptionSettings:
					if characters == nil || worldState == nil || worldState.role.ID == 0 || worldState.role.ID != selectedCharacterID {
						event(map[string]any{"kind": "character_settings_rejected", "reason": "requires the owned selected character", "character_id": selectedCharacterID})
						continue
					}
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					e = characters.Store.SaveCharacterUnifiedOptions(ctx, worldState.role.AccountID, worldState.role.ID, unifiedEntries(opt.Entries))
					cancel()
					if e != nil {
						event(map[string]any{"kind": "character_settings_rejected", "reason": e.Error(), "character_id": selectedCharacterID})
						continue
					}
					event(map[string]any{"kind": "character_settings_saved", "character_id": selectedCharacterID, "entries": len(opt.Entries)})
				case protocol.UnifiedOptionCharacterEffects:
					if opt.Scope != protocol.UnifiedOptionScopeCharac || characters == nil || worldState == nil || worldState.role.ID == 0 || worldState.role.ID != selectedCharacterID {
						event(map[string]any{"kind": "character_effect_options_rejected", "reason": "requires the owned selected character", "character_id": selectedCharacterID, "scope": opt.Scope})
						continue
					}
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					e = characters.Store.SaveCharacterUnifiedOptionGroup(ctx, worldState.role.AccountID, worldState.role.ID, opt.Subtype, unifiedEntries(opt.Entries))
					cancel()
					if e != nil {
						event(map[string]any{"kind": "character_effect_options_rejected", "reason": e.Error(), "character_id": selectedCharacterID})
						continue
					}
					event(map[string]any{"kind": "character_effect_options_saved", "character_id": selectedCharacterID, "entries": len(opt.Entries), "options": opt.Entries})
				case protocol.UnifiedOptionAccount:
					if characters == nil || opt.Scope != protocol.UnifiedOptionScopeAccount {
						event(map[string]any{"kind": "account_settings_rejected", "reason": "账号设置存储不可用或作用域无效"})
						continue
					}
					accountID := developmentAccount
					ownedRole := worldState != nil && worldState.role.ID != 0 && worldState.role.ID == selectedCharacterID
					if ownedRole {
						accountID = worldState.role.AccountID
					}
					var effectFlags byte
					for _, entry := range opt.Entries {
						if entry.Position == protocol.GrowthEffectOption && entry.Value != 65535 {
							effectFlags, e = protocol.GrowthEffectFlags(entry.Value)
							if e != nil {
								break
							}
						}
					}
					if e != nil {
						event(map[string]any{"kind": "account_settings_rejected", "reason": e.Error()})
						continue
					}
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					e = characters.Store.SaveAccountUnifiedOptions(ctx, accountID, unifiedEntries(opt.Entries))
					cancel()
					if e != nil {
						event(map[string]any{"kind": "account_settings_rejected", "reason": e.Error()})
						continue
					}
					event(map[string]any{"kind": "account_settings_saved", "entries": len(opt.Entries)})
					if effectFlags != 0 && ownedRole {
						// attempt 1/3：按1452FA194注册及1452C9940读取发送NOTI343。
						// 只更新显示阶段；不发送会重建装备、技能的全量USERINFO。
						effect := protocol.CharacterGrowthEffect(worldState.role.WireID, effectFlags)
						if e = sendPayload(0, 343, effect); e != nil {
							return
						}
						basic, refreshErr := characters.EntryBasicProbe(worldState.role, [2]byte{})
						if refreshErr != nil {
							event(map[string]any{"kind": "growth_effect_cache_error", "reason": refreshErr.Error()})
						} else {
							selectedBasic = basic
							worldState.hub.updateGrowthEffect(worldState.peer, basic, effect)
						}
						event(map[string]any{"kind": "growth_effect_updated", "character_id": selectedCharacterID, "flags": effectFlags})
					}
				case protocol.UnifiedOptionHotkeys, protocol.UnifiedOptionHotkeysExt:
					if characters == nil {
						event(map[string]any{"kind": "hotkeys_rejected", "reason": "storage unavailable"})
						continue
					}
					charID := selectedCharacterID
					if charID == 0 && worldState != nil && worldState.role.ID != 0 {
						charID = worldState.role.ID
					}
					accountID := developmentAccount
					if worldState != nil && worldState.role.AccountID != 0 {
						accountID = worldState.role.AccountID
					}
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					if opt.Scope == protocol.UnifiedOptionScopeAccount {
						if charID != 0 {
							_ = characters.Store.PromoteCharacterHotkeysToAccount(ctx, accountID, charID, opt.Subtype)
						}
						if len(opt.Entries) > 0 {
							e = characters.Store.SaveAccountHotkeys(ctx, accountID, opt.Subtype, unifiedEntries(opt.Entries))
						}
						if e == nil {
							_ = characters.Store.ClearAccountCharacterHotkeys(ctx, accountID, opt.Subtype)
						}
						cancel()
						if e != nil {
							event(map[string]any{"kind": "account_hotkeys_rejected", "reason": e.Error(), "subtype": opt.Subtype})
							continue
						}
						event(map[string]any{"kind": "account_hotkeys_saved", "subtype": opt.Subtype, "entries": len(opt.Entries), "character_id": charID})
					} else {
						if charID == 0 {
							cancel()
							event(map[string]any{"kind": "character_hotkeys_rejected", "reason": "requires the owned selected character", "character_id": selectedCharacterID})
							continue
						}
						_ = characters.Store.CopyAccountHotkeysToCharacter(ctx, accountID, charID, opt.Subtype)
						e = characters.Store.SaveCharacterHotkeys(ctx, accountID, charID, opt.Subtype, unifiedEntries(opt.Entries))
						cancel()
						if e != nil {
							event(map[string]any{"kind": "character_hotkeys_rejected", "reason": e.Error(), "character_id": charID, "subtype": opt.Subtype})
							continue
						}
						event(map[string]any{"kind": "character_hotkeys_saved", "character_id": charID, "subtype": opt.Subtype, "entries": len(opt.Entries)})
					}
				case protocol.UnifiedOptionHotkeyUI:
					event(map[string]any{"kind": "hotkey_ui_event", "character_id": selectedCharacterID, "scope": opt.Scope})
				}
				continue
			}

			if bootstrapped && (frame.ID == 1950 || frame.ID == 1951) {
				if !verified {
					event(map[string]any{"kind": "gamepad_settings_rejected", "id": frame.ID, "reason": "checksum failed"})
					continue
				}
				if characters == nil {
					event(map[string]any{"kind": "gamepad_settings_rejected", "id": frame.ID, "reason": "storage unavailable"})
					continue
				}
				charID := selectedCharacterID
				if charID == 0 && worldState != nil && worldState.role.ID != 0 {
					charID = worldState.role.ID
				}
				if frame.ID == 1950 {
					if len(plaintext) < 14 {
						event(map[string]any{"kind": "gamepad_keys_rejected", "reason": "payload too short", "bytes": len(plaintext)})
						continue
					}
					scope := plaintext[13]
					tsv := plaintext[14:]
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					var err error
					var hotPayload []byte
					if scope == 1 {
						err = characters.Store.SaveAccountGamepadKeys(ctx, developmentAccount, tsv)
						if err == nil {
							_ = characters.Store.ClearAccountCharacterGamepadSettings(ctx, developmentAccount)
							hotPayload, _ = characters.Store.AccountGamepadPayload(ctx, developmentAccount)
						}
					} else {
						if charID == 0 {
							cancel()
							event(map[string]any{"kind": "gamepad_keys_rejected", "reason": "requires selected character", "bytes": len(plaintext)})
							continue
						}
						err = characters.Store.SaveCharacterGamepadKeys(ctx, developmentAccount, charID, tsv)
						if err == nil {
							hotPayload, _ = characters.Store.ResolveGamepadPayload(ctx, developmentAccount, charID)
						}
					}
					cancel()
					if err != nil {
						event(map[string]any{"kind": "gamepad_keys_save_error", "scope": scope, "error": err.Error()})
						continue
					}
					// 回复 ACK: Kind=1, ID=1950, Payload=[0]
					if err := sendPayload(1, 1950, []byte{0}); err != nil {
						event(map[string]any{"kind": "gamepad_keys_ack_error", "error": err.Error()})
						continue
					}
					// 即时热生效：主动向客户端发送最新的 NOTI 2128
					if len(hotPayload) > 0 {
						if err := sendPayload(0, 2128, hotPayload); err != nil {
							event(map[string]any{"kind": "gamepad_hot_reload_error", "error": err.Error()})
						} else {
							event(map[string]any{"kind": "gamepad_hot_reloaded", "account_id": developmentAccount, "character_id": charID, "scope": scope, "bytes": len(hotPayload)})
						}
					}
					event(map[string]any{"kind": "gamepad_keys_saved", "account_id": developmentAccount, "character_id": charID, "scope": scope, "bytes": len(tsv)})
				} else if frame.ID == 1951 {
					if len(plaintext) < 24 {
						event(map[string]any{"kind": "gamepad_options_rejected", "reason": "payload too short", "bytes": len(plaintext)})
						continue
					}
					scope := plaintext[13]
					opts := plaintext[14:24]
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					var err error
					if scope == 1 {
						err = characters.Store.SaveAccountGamepadOptions(ctx, developmentAccount, opts)
						if err == nil {
							_ = characters.Store.ClearAccountCharacterGamepadSettings(ctx, developmentAccount)
						}
					} else {
						if charID == 0 {
							cancel()
							event(map[string]any{"kind": "gamepad_options_rejected", "reason": "requires selected character", "bytes": len(plaintext)})
							continue
						}
						err = characters.Store.SaveCharacterGamepadOptions(ctx, developmentAccount, charID, opts)
					}
					cancel()
					if err != nil {
						event(map[string]any{"kind": "gamepad_options_save_error", "scope": scope, "error": err.Error()})
						continue
					}
					// 回复 ACK: Kind=1, ID=1951, Payload=[0]
					if err := sendPayload(1, 1951, []byte{0}); err != nil {
						event(map[string]any{"kind": "gamepad_options_ack_error", "error": err.Error()})
						continue
					}
					event(map[string]any{"kind": "gamepad_options_saved", "account_id": developmentAccount, "character_id": charID, "scope": scope})
				}
				continue
			}

			if bootstrapped && (frame.ID == 3 || frame.ID == 7 || frame.ID == 1301) {
				if !verified {
					event(map[string]any{"kind": "menu_rejected", "id": frame.ID, "reason": "checksum failed"})
					continue
				}
				option, e := protocol.DecodeMenuRequest(frame.ID, plaintext)
				if e != nil {
					event(map[string]any{"kind": "menu_rejected", "id": frame.ID, "reason": e.Error()})
					continue
				}
				payload := protocol.MenuLeaveSuccess()
				if frame.ID == 1301 {
					payload, e = worldState.returnDestination()
					if e != nil {
						event(map[string]any{"kind": "village_return_refused", "reason": e.Error()})
						// Native refusal clears the manager wait at143ca6b3f.
						payload = protocol.Refusal(4)
					}
				}
				if e = sendPayload(1, frame.ID, payload); e != nil {
					return
				}
				event(map[string]any{"kind": "menu_response", "id": frame.ID, "character_id": selectedCharacterID, "option": option, "plain_hex": hex.EncodeToString(payload)})
				if frame.ID == 3 || frame.ID == 7 {
					selectedCharacterID = 0
					selectedBasic, selectedAddition = nil, nil
					if worldState != nil {
						worldState.departArea()
					}
					clearSelectedWorld(worldState)
					if frame.ID == 3 {
						bootstrapped = false
						event(map[string]any{"kind": "menu_exit_session_closed", "peer": peer, "option": option})
						return
					}
				}
				continue
			}
			// The exit button emits CMD1302 and CMD2285 in the same
			// millisecond; 1302 is an upload with no receive handler, 2285
			// is the content-briefing report whose own handler fills the
			// exit window. Answering it is what gives the in-game exit
			// button something to present; the client used to get silence.
			if bootstrapped && frame.ID == 2285 {
				if !verified {
					event(map[string]any{"kind": "content_briefing_rejected", "reason": "checksum failed"})
					continue
				}
				payload := protocol.ExitContentBriefingDefaults()
				if err := sendPayload(1, 2285, payload); err != nil {
					return
				}
				event(map[string]any{"kind": "content_briefing_response", "id": frame.ID, "character_id": selectedCharacterID, "bytes": len(payload), "plain_hex": hex.EncodeToString(payload)})
				continue
			}
			if characters != nil && bootstrapped && frame.ID == 331 {
				if !verified {
					event(map[string]any{"kind": "skill_commands_rejected", "reason": "checksum failed", "character_id": selectedCharacterID})
					continue
				}
				if worldState == nil || worldState.role.ID != selectedCharacterID {
					event(map[string]any{"kind": "skill_commands_rejected", "reason": "character selection mismatch", "character_id": selectedCharacterID})
					continue
				}
				count, e := skillState.saveCommands(characters, worldState, plaintext)
				if e != nil {
					event(map[string]any{"kind": "skill_commands_rejected", "reason": e.Error(), "character_id": selectedCharacterID})
				} else {
					event(map[string]any{"kind": "skill_commands_saved", "character_id": selectedCharacterID, "count": count})
					restore, restoreErr := characters.EntrySkills(worldState.role)
					if restoreErr != nil {
						event(map[string]any{"kind": "skill_commands_refresh_failed", "reason": restoreErr.Error(), "character_id": selectedCharacterID})
					} else {
						if e = sendPayload(0, 19, restore); e != nil {
							return
						}
						event(map[string]any{"kind": "skill_commands_refreshed", "character_id": selectedCharacterID, "id": 19})
						preset, presetErr := characters.SkillPresetInfo(worldState.role)
						if presetErr != nil {
							event(map[string]any{"kind": "skill_preset_refresh_failed", "reason": presetErr.Error(), "character_id": selectedCharacterID})
						} else if len(preset) > 0 {
							if e = sendPayload(0, 2758, preset); e != nil {
								return
							}
							event(map[string]any{"kind": "skill_preset_restored_after_commands", "character_id": selectedCharacterID, "id": 2758})
						}
						combo, comboErr := characters.ComboSkillInfoNotify(worldState.role)
						if comboErr != nil {
							event(map[string]any{"kind": "combo_skill_info_refresh_failed", "character_id": selectedCharacterID, "error": comboErr.Error()})
						} else if len(combo) > 0 {
							if e = sendPayload(0, 433, combo); e != nil {
								return
							}
							event(map[string]any{"kind": "combo_skill_info_restored_after_commands", "character_id": selectedCharacterID, "type": 0, "id": 433, "plain_hex": hex.EncodeToString(combo)})
						}
					}
				}
				continue
			}
			if characters != nil && bootstrapped && (frame.ID == 500 || frame.ID == 502) {
				if !verified || worldState == nil || worldState.role.ID != selectedCharacterID {
					event(map[string]any{"kind": "combo_skill_info_rejected", "id": frame.ID, "character_id": selectedCharacterID, "reason": "checksum or character selection mismatch"})
					continue
				}
				req, saveErr := comboState.save(characters, worldState, frame.ID, plaintext)
				if saveErr != nil {
					event(map[string]any{"kind": "combo_skill_info_rejected", "id": frame.ID, "character_id": selectedCharacterID, "reason": saveErr.Error()})
					continue
				}
				event(map[string]any{"kind": "combo_skill_info_saved", "id": frame.ID, "character_id": selectedCharacterID, "cells": req.Cells, "plain_hex": hex.EncodeToString(plaintext)})
				if frame.ID == 500 {
					notify, encodeErr := protocol.EncodeComboSkillInfoNotify(req)
					if encodeErr != nil {
						event(map[string]any{"kind": "combo_skill_info_reply_failed", "character_id": selectedCharacterID, "error": encodeErr.Error()})
						continue
					}
					if !bytes.Equal(notify, comboState.lastNotify) {
						if sendErr := sendPayload(0, 433, notify); sendErr != nil {
							return
						}
						comboState.lastNotify = notify
						event(map[string]any{"kind": "combo_skill_info_replied", "character_id": selectedCharacterID, "type": 0, "id": 433, "plain_hex": hex.EncodeToString(notify)})
					}
				}
				continue
			}
			if characters != nil && bootstrapped && frame.ID == 527 {
				if !verified || worldState == nil || worldState.role.ID != selectedCharacterID {
					continue
				}
				ack, saveErr := cubeContractState.save(worldState, plaintext)
				if saveErr != nil {
					event(map[string]any{"kind": "cube_contract_selection_rejected", "character_id": selectedCharacterID, "reason": saveErr.Error()})
					ack = protocol.Refusal(0)
				}
				if e := sendPayload(1, 527, ack); e != nil {
					return
				}
				if saveErr == nil {
					event(map[string]any{"kind": "cube_contract_selection_saved", "character_id": selectedCharacterID, "selection": ack[2]})
				}
				continue
			}
			if characters != nil && bootstrapped && (frame.ID == 28 || frame.ID == 29 || frame.ID == 483 || frame.ID == 2179 || frame.ID == 2346 || frame.ID == 2347) {
				if !verified {
					continue
				}
				plan, e := skillState.handle(characters, worldState, frame.ID, plaintext, frame.Raw)
				if e != nil {
					event(map[string]any{"kind": "skill_refused", "id": frame.ID, "reason": e.Error()})
					plan = []outboundPacket{{"skill_refused_response", 1, frame.ID, protocol.Refusal(4)}}
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": selectedCharacterID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			// Teaching/notice read reports. CMD469 is tree 1 (INFORM_NOTICE,
			// payload = u32 notice id); CMD495 is tree 2 (INFORM_NOTICE_2ND)
			// with two shapes: the bulletin-board opcode 0x3e (62) and the
			// teaching frame's (u32 id, u8 value). 62 doubles as the
			// "player chose Manual Setup" mark, so persisting it here is what
			// stops the third-awakening teaching frame from re-popping once the
			// guide has been answered. Both are acknowledged with {1,0}.
			if characters != nil && bootstrapped && selectedCharacterID != 0 && (frame.ID == 469 || frame.ID == 495) {
				if !verified {
					continue
				}
				if worldState == nil || worldState.role.ID != selectedCharacterID {
					event(map[string]any{"kind": "notice_seen_rejected", "reason": "character selection mismatch", "character_id": selectedCharacterID})
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				var persistErr error
				switch frame.ID {
				case 469:
					if len(plaintext) < 4 {
						persistErr = fmt.Errorf("short notice-seen report")
						break
					}
					nid := binary.LittleEndian.Uint32(plaintext[:4])
					if nid > 255 {
						persistErr = fmt.Errorf("notice id out of wire range")
						break
					}
					persistErr = characters.Store.MarkCharacterNotice(ctx, developmentAccount, selectedCharacterID, 1, uint16(nid), true)
					if persistErr == nil {
						event(map[string]any{"kind": "notice_seen_persisted", "character_id": selectedCharacterID, "tree": 1, "notice_id": nid})
					}
				case 495:
					if len(plaintext) >= 4 && binary.LittleEndian.Uint32(plaintext[:4]) == 0x3e {
						// Bulletin-board opcode 62. Persisting 62 into tree 2
						// also covers the Manual Setup teaching mark; the
						// season-5 Anton quest chain (pre-req 3223 -> NPC15
						// 3226) is a separate flow and is not implemented here.
						persistErr = characters.Store.MarkCharacterNotice(ctx, developmentAccount, selectedCharacterID, 2, 62, true)
						if persistErr == nil {
							event(map[string]any{"kind": "notice_2nd_persisted", "character_id": selectedCharacterID, "notice_id": 62})
						}
						break
					}
					if len(plaintext) < 5 {
						persistErr = fmt.Errorf("short notice-seen report")
						break
					}
					nid := binary.LittleEndian.Uint32(plaintext[:4])
					value := plaintext[4]
					if nid > 255 {
						persistErr = fmt.Errorf("notice id out of wire range")
						break
					}
					persistErr = characters.Store.MarkCharacterNotice(ctx, developmentAccount, selectedCharacterID, 2, uint16(nid), value != 0)
					if persistErr == nil {
						if value != 0 {
							event(map[string]any{"kind": "notice_2nd_seen_persisted", "character_id": selectedCharacterID, "notice_id": nid})
						} else {
							event(map[string]any{"kind": "notice_2nd_seen_removed", "character_id": selectedCharacterID, "notice_id": nid})
						}
					}
				}
				cancel()
				if persistErr != nil {
					event(map[string]any{"kind": "notice_seen_rejected", "character_id": selectedCharacterID, "reason": persistErr.Error()})
					continue
				}
				if e := sendPayload(1, frame.ID, []byte{1, 0}); e != nil {
					return
				}
				event(map[string]any{"kind": "notice_seen_ack", "character_id": selectedCharacterID, "id": frame.ID})
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 18 {
				if !verified {
					continue
				}
				// 技能材料原因 2 和晶体契约原因 5 共用真实扣除与幂等事务。
				// 城镇丢弃、非材料物品等请求继续由通用删除路径处理。
				plan, e := worldState.deleteSkillMaterial(plaintext, frame.Raw)
				if e == nil {
					for _, packet := range plan {
						if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
					}
					continue
				}
				event(map[string]any{"kind": "skill_material_refused", "reason": e.Error(), "character_id": worldState.role.ID})
				plan, e = worldState.deleteItems(plaintext, frame.Raw)
				if e != nil {
					event(map[string]any{"kind": "item_delete_refused", "reason": e.Error(), "character_id": worldState.role.ID})
					rows, _ := protocol.DecodeMaterialDelete(plaintext)
					reply := protocol.MaterialDeleteReply(rows, false)
					if contractRows, decodeErr := protocol.DecodeCubeContractDelete(plaintext); decodeErr == nil {
						reply = protocol.CubeContractDeleteReply(contractRows, false)
					}
					if e = sendPayload(1, 18, reply); e != nil {
						return
					}
					event(map[string]any{"kind": "item_delete_refusal_sent", "character_id": worldState.role.ID, "plain_hex": hex.EncodeToString(reply)})
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 507 {
				if !verified {
					continue
				}
				if len(plaintext) >= 11 && binary.LittleEndian.Uint32(plaintext[7:11]) == protocol.SeasonCapsuleAction {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					packets, err := worldState.useSeasonCapsule(ctx, plaintext, frame.Raw, purchaseSession.prefix)
					cancel()
					if err != nil {
						event(map[string]any{"kind": "season_capsule_refused", "character_id": selectedCharacterID, "reason": err.Error()})
						if slot, decodeErr := protocol.DecodeSeasonCapsule(plaintext); decodeErr == nil {
							if err = sendPayload(1, 507, protocol.SeasonCapsuleReply(slot, false)); err != nil {
								return
							}
						}
						continue
					}
					for _, packet := range packets {
						if err = sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "character_id": selectedCharacterID, "id": packet.ID})
					}
					continue
				}
				// CMD507 is the shared "use stackable" frame. Split it by action so
				// the fatigue potion (54) and `[add skin storage]` (169, damage font)
				// paths never collide; the fatigue path keeps its exact prior shape.
				_, action, actionErr := protocol.DecodeStackableAction(plaintext)
				if actionErr == nil && action == protocol.RosterBackgroundTicketAction {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					packets, e := worldState.useRosterBackgroundTicket(ctx, plaintext, frame.Raw, purchaseSession.prefix, event)
					cancel()
					if e != nil {
						event(map[string]any{"kind": "背景券使用被拒绝", "character_id": worldState.role.ID, "reason": e.Error()})
						continue
					}
					for _, packet := range packets {
						if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
					}
					continue
				}
				if actionErr == nil && action == protocol.AddSkinStorageAction {
					plan, e := worldState.useAddSkinStorage(plaintext, event)
					if e != nil {
						event(map[string]any{"kind": "add_skin_storage_refused", "character_id": worldState.role.ID, "reason": e.Error()})
						continue
					}
					for _, packet := range plan {
						if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "id": packet.ID})
					}
					continue
				}
				if len(plaintext) >= 11 && binary.LittleEndian.Uint32(plaintext[7:11]) == 206 {
					plan, e := worldState.useQuestAirshipItem(plaintext, event)
					if e != nil {
						event(map[string]any{"kind": "quest_item_action_refused", "character_id": worldState.role.ID, "reason": e.Error()})
						continue
					}
					for _, packet := range plan {
						if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID})
					}
					continue
				}
				if actionErr == nil && (action == protocol.ActionOpenAuraSkinSlot || action == protocol.ActionOpenCreatureSkinSlot) {
					// 幻化栏扩展券：光环 action 101、宠物 action 197。同一个 CMD507 上复用多种
					// 动作，这里只接这两路，54/169/206 保持各自原有的入口形状。
					plan, e := worldState.stackableAction(plaintext)
					if e != nil {
						event(map[string]any{"kind": "skin_slot_expand_refused", "character_id": worldState.role.ID, "reason": e.Error()})
						continue
					}
					for _, packet := range plan {
						if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID})
					}
					continue
				}
				if fatigueService == nil {
					event(map[string]any{"kind": "fatigue_potion_refused", "character_id": worldState.role.ID, "reason": "fatigue service unavailable"})
					continue
				}
				plan, e := worldState.recoverFatiguePotion(plaintext)
				if e != nil {
					event(map[string]any{"kind": "fatigue_potion_refused", "character_id": worldState.role.ID, "reason": e.Error()})
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID})
				}
				continue
			}
			// CMD857 = ENUM_CMDPACKET_OPEN_AURA_SKIN_SLOT：幻化栏窗口弹「Unlock the Aura
			// Skin slot?」时点 OK 发的就是它。以前没有这一分支，服务端既不置位也不回包，客户
			// 端拿不到成功标志就永远不推进 —— 玩家看到的就是「点了没反应」。
			if worldState != nil && bootstrapped && frame.ID == 857 {
				if !verified {
					event(map[string]any{"kind": "open_skin_slot_rejected", "reason": "幻化栏开启请求校验失败"})
					continue
				}
				plan, e := worldState.openSkinSlot(plaintext)
				if e != nil {
					event(map[string]any{"kind": "open_skin_slot_refused", "character_id": worldState.role.ID,
						"reason": e.Error(), "request_hex": hex.EncodeToString(plaintext)})
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "id": packet.ID})
				}
				continue
			}
			// 增幅摧毁装备后客户端会把金币显示清 0（存档是对的）。
			// 挂在会话上的延后补发在这里出队 —— 放在主循环里串行发送，避免并发写 socket。
			if worldState != nil {
				if body := equipmentState.takePendingGold(time.Now()); len(body) > 0 {
					if err := sendPayload(0, 14, body); err != nil {
						return
					}
					event(map[string]any{"kind": "amplify_gold_resynced", "character_id": worldState.role.ID})
				}
			}
			if worldState != nil && bootstrapped && frame.ID == 80 {
				if !verified {
					event(map[string]any{"kind": "reinforcement_rejected", "reason": "强化请求校验失败"})
					continue
				}
				plan, err := equipmentState.reinforce(wearService, worldState, plaintext, frame.Raw, event)
				if err != nil {
					// 客户端在 CMD80 的错误分支只认错误码（u16）去取 dstr 文案，不认原因字符串。
					// 以前一律发 22，而 22 恰好映射到「材料不足」，于是任何拒绝都被玩家看成材料不够。
					code := reinforcementRefusalCode(err)
					event(map[string]any{
						"kind":         "reinforcement_refused",
						"character_id": worldState.role.ID,
						"reason":       err.Error(),
						"error_code":   code,
						"request_hex":  hex.EncodeToString(plaintext),
					})
					// 14529B2F0 的失败分支只使用分发器读取的错误码，并清除等待态。
					if err = sendPayload(1, 80, protocol.Refusal(code)); err != nil {
						return
					}
					continue
				}
				for _, packet := range plan {
					if err = sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
				}
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 205 {
				// CMD205 = ENUM_CMDPACKET_INVEST_ITEM_AMPLIFY_OPTION：用增幅书（红字书）
				// 给装备打次元属性。
				//
				// ★ 2026-09-28：这条分派以前**整条缺失**。服务层的 ApplyAmplifyGrimoire 与
				// 流程层的 applyAmplifyGrimoire 都在、清单也装载了，但没有任何地方调用它们
				// —— 客户端发 205 得到的是「既不改状态也不回包」，玩家看到的就是
				// 「增幅书打了没效果」（白银书与黄金书都受影响）。
				if !verified {
					event(map[string]any{"kind": "amplify_grimoire_rejected", "reason": "打红字请求校验失败"})
					continue
				}
				plan, err := worldState.applyAmplifyGrimoire(wearService, plaintext, frame.Raw, event)
				if err != nil {
					event(map[string]any{
						"kind":         "amplify_grimoire_refused",
						"character_id": worldState.role.ID,
						"reason":       err.Error(),
						"request_hex":  hex.EncodeToString(plaintext),
					})
					if err = sendPayload(1, 205, amplifyGrimoireRefusal()); err != nil {
						return
					}
					continue
				}
				for _, packet := range plan {
					if err = sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
				}
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 430 {
				// CMD430 = 锻造（Refine，NPC Kiri）：仅武器、上限 +8、失败等级不变不碎。
				if !verified {
					event(map[string]any{"kind": "refine_rejected", "reason": "锻造请求校验失败"})
					continue
				}
				plan, err := worldState.refine(wearService, plaintext, frame.Raw, event)
				if err != nil {
					event(map[string]any{
						"kind": "refine_refused", "character_id": worldState.role.ID,
						"reason": err.Error(), "request_hex": hex.EncodeToString(plaintext),
					})
					// 锻造与其它升级命令共用同一张错误码表（唯一差别是 17 那格：
					// 锻造是 35076 "The equipment cannot be refined."，CMD80 是 1652）。
					code := refineRefusalCode(err)
					if err = sendPayload(1, 430, protocol.Refusal(code)); err != nil {
						return
					}
					continue
				}
				for _, packet := range plan {
					if err = sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
				}
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 272 {
				// CMD272 = 附魔宝珠（ENCHANT_BY_BEAD）：扣 1 颗宝珠、把卡的附魔写进装备行。
				if !verified {
					event(map[string]any{"kind": "enchant_rejected", "reason": "附魔请求校验失败"})
					continue
				}
				plan, err := worldState.enchantByBead(wearService, plaintext, frame.Raw, event)
				if err != nil {
					event(map[string]any{
						"kind": "enchant_refused", "character_id": worldState.role.ID,
						"reason": err.Error(), "request_hex": hex.EncodeToString(plaintext),
					})
					if err = sendPayload(1, 272, protocol.Refusal(enchantRefusalCode(err))); err != nil {
						return
					}
					continue
				}
				for _, packet := range plan {
					if err = sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
				}
				continue
			}
			// CMD1722 = 装备继承：把材料件的强化 / 增幅 / 锻造 / 附魔转移到基础件，
			// 材料件清零保留。一次请求可带多条记录（多对装备同时轮换继承）。
			//
			// ★ 2026-09-28：这条分派以前**整条缺失**。客户端按下确认后直发 1722，
			// 服务端既不改状态也不回包，客户端就一直停在等待态 —— 玩家看到的就是
			// 「按下继承毫无效果」。与 CMD205（增幅书）那次是同一类缺陷。
			//
			// ⚠️⚠️ **任何方向、任何形态的 1722 出站包都禁止**。客户端的 opcode 表是
			// 两套独立命名空间：kind=1 → CMD 表，1722 在那一侧是客户端自己发出去的
			// 命令、没有接收 handler，回了会被当成「自己发的继承命令」解析、格式不符；
			// kind=0 → NOTI 表，1722 在那一侧的 handler 是**小游戏道具使用计数通知**
			// （sub_143348A90），与继承结果无关。所以成功 / 失败 / 拒绝都
			// **只落库 + 发 id14 行刷新，不回任何 1722 包**。取证见
			// internal/game/protocol/inherit.go 末尾的注释块。
			if worldState != nil && bootstrapped && frame.ID == 1722 {
				if !verified {
					event(map[string]any{"kind": "inherit_rejected", "reason": "继承请求校验失败"})
					continue
				}
				plan, err := worldState.inherit(wearService, plaintext, frame.Raw, event)
				if err != nil {
					event(map[string]any{
						"kind":         "inherit_refused",
						"character_id": worldState.role.ID,
						"reason":       err.Error(),
						"request_hex":  hex.EncodeToString(plaintext),
					})
					// 拒绝只记日志：1722 没有任何合法的回包通道（见上方注释），
					// 存档也没变所以无需刷新包。
					continue
				}
				for _, packet := range plan {
					if err = sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
						return
					}
				}
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 1565 {
				if !verified {
					continue
				}
				// CMD1565 is the skin cargo's 应用 click. The client sends it and
				// waits: nothing on screen changes until the selection frame comes
				// back, which is why an applied damage font used to look inert.
				plan, e := worldState.selectSkin(plaintext, event)
				if e != nil {
					event(map[string]any{"kind": "skin_selection_failed", "character_id": worldState.role.ID, "reason": e.Error()})
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "id": packet.ID,
						"plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			// 武器幻化复制确认（CMD1592）：窗口点确认后服务端扣武器本体 + 一枚模具，
			// 并把皮肤登记进幻化仓库。包体只有八字节，模板要靠 Index 自己解析。
			if worldState != nil && bootstrapped && frame.ID == 1592 {
				if !verified {
					event(map[string]any{"kind": "make_skin_rejected", "reason": "checksum failed"})
					continue
				}
				plan, e := worldState.makeSkin(plaintext, event)
				if e != nil {
					event(map[string]any{"kind": "make_skin_refused", "character_id": worldState.role.ID, "reason": e.Error()})
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": worldState.role.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			// 武器幻化应用（CMD1565）：幻化仓库窗口的容器同步。开页签与按 Apply 都发这
			// 一条，而 Apply 处理器硬编码 subtype=4（武器外观页），所以 subtype 4 且带皮
			// 肤 id 的帧就是「把这个外观应用到我的武器上」。落库后立刻用 opcode 2 的
			// mode0 用户信息块重建角色——装备外观块是驱动世界模型的唯一通道。
			if worldState != nil && bootstrapped && frame.ID == 1565 {
				if !verified {
					event(map[string]any{"kind": "skin_cargo_sync_rejected", "reason": "checksum failed"})
					continue
				}
				plan, e := worldState.syncSkin(plaintext, event)
				if e != nil {
					event(map[string]any{"kind": "skin_cargo_sync_refused", "character_id": worldState.role.ID, "reason": e.Error()})
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": worldState.role.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 44 && lootService != nil {
				if !verified {
					event(map[string]any{"kind": "item_use_rejected", "reason": "checksum failed"})
					continue
				}
				plan, e := worldState.useStackable(plaintext, event)
				if e != nil {
					event(map[string]any{"kind": "item_use_refused", "character_id": worldState.role.ID, "reason": e.Error()})
					if r, decodeErr := protocol.DecodeUseStackable(plaintext); decodeErr == nil {
						if e = sendPayload(1, 44, protocol.UseStackableRefused(r)); e != nil {
							return
						}
					}
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "id": packet.ID})
				}
				continue
			}
			// 表情快捷键（CMD1551）：体是 `u32 表情皮肤 id, u32 0`。实机 2026-09-28 三格三个值
			// （10179 / 10176 / 10175），都是 configs/skin-storage-items.json 里 instant emoticon 的
			// 皮肤 id（模板 10325568 / 10325577 等）⇒ 体首就是玩家按下的那个表情。
			//
			// 这里**只留观测，不回包**：attempt 1/3 试过回 CMD 2039（体一个 01 状态字节，客户端
			// sub_1444E8CC0 状态非 0 只发界面事件 2345），四格各按一次、服务端六条 emote_use_
			// acknowledged，实机**没有气泡**⇒ 已否证，帧撤掉。另有一条独立理由：2039 的处理器一个
			// 体字节都不读，带不了「哪个角色放哪个表情」，而气泡必须点名角色。1551 仍登记进
			// request_scope（见那里的注释）：不登记就只解密前八次，第八条之后连这条日志都没有。
			if worldState != nil && bootstrapped && verified && frame.ID == 1551 {
				if len(plaintext) < 4 {
					event(map[string]any{"kind": "emote_use_rejected", "reason": "short body",
						"bytes": len(plaintext)})
					continue
				}
				event(map[string]any{"kind": "emote_use_observed", "character_id": worldState.role.ID,
					"skin_id": binary.LittleEndian.Uint32(plaintext)})
				continue
			}
			// 装备库（装备图鉴）「制作 / 变换」：CMD2259。
			// **阶段一：只回 6 字节应答（"打开哪个制作窗口"），不碰存档。**
			if worldState != nil && bootstrapped && frame.ID == 2259 {
				if !verified {
					event(map[string]any{"kind": "equipment_craft_rejected", "id": frame.ID, "reason": "checksum failed"})
					continue
				}
				plan, e := worldState.equipmentCraft(plaintext, event)
				if e != nil {
					event(map[string]any{"kind": "equipment_craft_refused", "id": frame.ID, "character_id": worldState.role.ID, "reason": e.Error()})
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID,
						"id": packet.ID, "bytes": len(packet.Payload)})
				}
				continue
			}
			// 装备库（装备图鉴）收藏：CMD2264。
			// 回包顺序 = 先提交账本 → 回 2264（**非空**正文）→ 补发 2610 权威快照。
			if worldState != nil && bootstrapped && frame.ID == 2264 && lootService != nil && journalRules != nil {
				if !verified {
					event(map[string]any{"kind": "equipment_journal_rejected", "id": frame.ID, "reason": "checksum failed"})
					continue
				}
				plan, e := worldState.equipmentFavorite(plaintext)
				if e != nil {
					event(map[string]any{"kind": "equipment_journal_refused", "id": frame.ID, "character_id": worldState.role.ID, "reason": e.Error()})
					if e = sendPayload(1, frame.ID, protocol.Refusal(19)); e != nil {
						return
					}
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "id": packet.ID, "bytes": len(packet.Payload)})
				}
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 26 && lootService != nil {
				if !verified {
					event(map[string]any{"kind": "disjoint_rejected", "id": frame.ID, "reason": "checksum failed"})
					continue
				}
				plan, e := worldState.disjointItem(plaintext, event)
				if e != nil {
					event(map[string]any{"kind": "disjoint_refused", "id": frame.ID, "character_id": worldState.role.ID, "reason": e.Error()})
					refusalCode := uint16(19)
					if strings.Contains(e.Error(), "material inventory is full") {
						refusalCode = 4
					}
					if e = sendPayload(1, frame.ID, protocol.Refusal(refusalCode)); e != nil {
						return
					}
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "id": packet.ID})
				}
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 393 && unsealService != nil && lootService != nil {
				if !verified {
					event(map[string]any{"kind": "unseal_rejected", "reason": "checksum failed"})
					continue
				}
				plan, request, e := worldState.unsealRandomOption(unsealService, lootService.Catalog.Source.Checksum, plaintext)
				if e != nil {
					event(map[string]any{"kind": "unseal_refused", "id": frame.ID, "character_id": worldState.role.ID, "reason": e.Error()})
					if e = sendPayload(1, frame.ID, protocol.UnsealRefused(unsealRefusalCode(e))); e != nil {
						return
					}
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "id": packet.ID, "slot": request.TargetSlot})
				}
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 21 && lootService != nil {
				if !verified {
					event(map[string]any{"kind": "shop_buy_rejected", "reason": "checksum failed"})
					continue
				}
				event(map[string]any{"kind": "shop_buy_request", "character_id": worldState.role.ID, "id": 21, "plain_hex": hex.EncodeToString(plaintext)})
				plan, e := worldState.buyItem(plaintext)
				if e != nil {
					event(map[string]any{"kind": "shop_buy_refused", "character_id": worldState.role.ID, "reason": e.Error(), "plain_hex": hex.EncodeToString(plaintext)})
					if e = sendPayload(1, 21, protocol.Refusal(4)); e != nil {
						return
					}
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 22 && lootService != nil {
				if !verified {
					event(map[string]any{"kind": "shop_sell_rejected", "reason": "checksum failed"})
					continue
				}
				event(map[string]any{"kind": "shop_sell_request", "character_id": worldState.role.ID, "id": 22, "plain_hex": hex.EncodeToString(plaintext)})
				plan, e := worldState.sellItem(plaintext)
				if e != nil {
					event(map[string]any{"kind": "shop_sell_refused", "character_id": worldState.role.ID, "reason": e.Error(), "plain_hex": hex.EncodeToString(plaintext)})
					if e = sendPayload(1, 22, protocol.Refusal(4)); e != nil {
						return
					}
					continue
				}
				for _, packet := range plan {
					if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "character_id": worldState.role.ID, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
				}
				continue
			}
			if characters != nil && bootstrapped && frame.ID == 143 {
				if !verified || selectedCharacterID == 0 {
					event(map[string]any{"kind": "tutorial_rejected", "reason": "invalid checksum or no selected character"})
					continue
				}
				r, e := protocol.DecodeTutorialChange(plaintext)
				if e != nil {
					event(map[string]any{"kind": "tutorial_rejected", "reason": e.Error()})
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				e = characters.Store.SaveTutorialFlag(ctx, developmentAccount, selectedCharacterID, r.Index, r.Completed)
				cancel()
				if e != nil {
					event(map[string]any{"kind": "tutorial_save_error", "error": e.Error()})
					continue
				}
				if e = sendPayload(1, 143, protocol.TutorialChangeSaved()); e != nil {
					return
				}
				event(map[string]any{"kind": "tutorial_flag_saved", "character_id": selectedCharacterID, "index": r.Index, "completed": r.Completed, "rewards_granted": false})
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 15 {
				if !verified {
					event(map[string]any{"kind": "dungeon_gate_rejected", "reason": "checksum failed"})
					continue
				}
				plan, e := worldState.dungeonGate(plaintext)
				if e != nil {
					worldState.approvedDungeonGate = 0
					event(map[string]any{"kind": "dungeon_gate_rejected", "reason": e.Error()})
					if e = sendPayload(1, 15, protocol.Refusal(4)); e != nil {
						return
					}
					continue
				}
				prepared, e := preparePackets(keys, plan)
				if e != nil {
					event(map[string]any{"kind": "dungeon_encode_error", "error": e.Error()})
					continue
				}
				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if e = writePackets(c, prepared, func(p preparedPacket) {
					event(map[string]any{"kind": p.Name, "id": p.ID, "character_id": selectedCharacterID, "plain_hex": hex.EncodeToString(p.Payload)})
				}); e != nil {
					event(map[string]any{"kind": "dungeon_write_error", "error": e.Error()})
					return
				}
				worldState.selectingDungeon = true
				worldState.approvedDungeonGate, _ = protocol.DecodeDungeonGate(plaintext)
				worldState.pendingTownArrival = nil
				continue
			}
			if worldState != nil && bootstrapped && dungeonRequest(frame.ID) {
				if !verified {
					event(map[string]any{"kind": "dungeon_request_rejected", "id": frame.ID, "reason": "checksum failed"})
					continue
				}
				var plan []outboundPacket
				var pending *dungeon.Session
				var townArrivalLoading bool
				var e error
				switch frame.ID {
				case 16:
					pending, plan, e = worldState.selectDungeon(plaintext)
				case 1852:
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					pending, plan, e = worldState.startBlackPurgatory(ctx, plaintext)
					cancel()
				case 37:
					if worldState.activeDungeon == nil && worldState.pendingTownArrival != nil {
						scene := worldState.townArrivalScenes[worldState.pendingTownArrival.Definition.ID]
						if worldState.state.Position.Town != scene.Town || worldState.state.Position.Area != scene.Area {
							worldState.pendingTownArrival = nil
							e = fmt.Errorf("town arrival scene origin changed before loading")
							break
						}
						worldState.activeDungeon = worldState.pendingTownArrival
						townArrivalLoading = true
					}
					plan, e = worldState.finishDungeonLoading(plaintext)
					if e != nil && townArrivalLoading {
						worldState.activeDungeon = nil
						townArrivalLoading = false
					}
				case 38:
					pending, plan, e = worldState.interactDoor(plaintext)
				case 39:
					worldState.completionErr = nil
					plan, e = worldState.monsterDeath(plaintext, event)
				case 2329:
					plan, e = worldState.scaleStatus(plaintext, event)
				case 40:
					plan, e = worldState.playerDeath(plaintext, frame.Raw)
					if e == nil && worldState.bleedingMineStart == nil {
						// [MERGE-20260928-DEATH-FAIL-TIMEOUT] 原生「倒计时结束 → 挑战失败」
						// 由服务端推进：客户端进复活 UI 后只会等，不会发请求。死亡后等待
						// deathFailTimeout，期间没复活就下发 NOTI33 (FAIL_CLEAR_DUNGEON)，
						// 驱动失败结算与回城。此前只有 Elvenmere(100003126) 会发，其它副本
						// 死亡后永远停在 Dead 界面（实机 2026-09-28：倒计时结束不回城，
						// 剧情叠在死亡界面上卡死）。
						w := worldState
						select {
						case <-done:
						default:
							time.AfterFunc(deathFailTimeout, func() {
								d := w.pilotDeath
								if d == nil || !d.Dead || w.activeDungeon == nil {
									return
								}
								// reason 100 = timeout（0 是「默认死亡」）。
								if err := sendPayload(0, 33, protocol.DungeonFailClear(100)); err != nil {
									return
								}
								// 只发 FAIL_CLEAR 不够：客户端收到后只播死亡镜头，不会自己
								// 离开副本 —— 实机 2026-09-28 客户端 trace 里
								// `RECV ENUM_NOTIPACKET_FAIL_CLEAR_DUNGEON` 之后 25 秒毫无
								// 动作，直到玩家手动发 GIVEUP_GAME(42) 才回城。
								// 这里照「放弃」那条路径把玩家送回城。
								leave, e := w.leaveDungeon()
								if e != nil {
									event(map[string]any{"kind": "death_fail_leave_error", "error": e.Error()})
									return
								}
								for _, p := range leave {
									if e := sendPayload(p.Kind, p.ID, p.Payload); e != nil {
										return
									}
								}
								// 主循环在发出 dungeon_leave_ack 时会清掉副本会话
								// （main.go 的 `p.Name == "dungeon_leave_ack"` 分支），
								// 这里绕过了那段，必须自己清 —— 否则客户端回城后发来的
								// 门请求仍会命中一个已离开的会话。
								w.activeDungeon = nil
								w.bleedingMineStart = nil
								event(map[string]any{"kind": "death_fail_timeout", "run": d.Run, "steps": len(leave) + 1})
							})
						}
					}
				case 43:
					plan, e = worldState.pickup(plaintext)
				case 117:
					plan, e = worldState.bossCheck(plaintext)
				case 45:
					if worldState.pilotDeath != nil && worldState.activeDungeon != nil && worldState.pilotDeath.Run == worldState.activeDungeon.RunID && worldState.pilotDeath.Dead {
						e = fmt.Errorf("room movement requires living player")
					} else {
						pending, plan, e = worldState.moveDungeonRoom(plaintext)
					}
				case 46:
					plan, e = worldState.dungeonResult(plaintext)
				case 69, 70:
					plan, e = worldState.cardStage(frame.ID, plaintext)
				case 71:
					plan, e = worldState.cardPick(plaintext)
				case 72:
					pending, plan, e = worldState.settlementExit(plaintext)
				case 449:
					plan, e = worldState.tournamentSelectState(plaintext)
				case 450:
					plan, e = worldState.tournamentSelect(plaintext)
				case 132:
					plan, e = worldState.returnFromDungeonSelection(plaintext)
				case 2319:
					plan, e = worldState.giveUpBleedingMine(plaintext)
				case 1461:
					plan, e = worldState.bleedingMineDeath(plaintext)
				case 2320:
					plan, e = worldState.settleBleedingMineStage(plaintext)
				case 2321:
					plan, e = worldState.finishBleedingMine(plaintext)
				case 2325:
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					plan, e = worldState.composeBleedingMineRewards(ctx, plaintext, frame.Raw)
					cancel()
				case 2322, 2323:
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					if frame.ID == 2322 {
						plan, e = worldState.claimBleedingMineRewards(ctx, plaintext)
					} else {
						plan, e = worldState.openBleedingMineRewards(ctx, plaintext)
					}
					cancel()
				case 2327:
					plan, e = worldState.reviveBleedingMine(plaintext, frame.Raw, time.Now())
				case 42:
					if len(plaintext) != 0 {
						e = fmt.Errorf("unexpected give-up body")
					} else if worldState.bleedingMineStart != nil {
						plan, e = worldState.endBleedingMine()
						if e == nil {
							plan = append([]outboundPacket{{"dungeon_leave_ack", 1, 42, []byte{1}}}, plan...)
						}
					} else {
						plan, e = worldState.leaveDungeon()
					}
				case 2015:
					plan, e = worldState.elvenmereTeleport(plaintext)
				case 2062:
					pending, plan, e = worldState.directMoveDungeon(plaintext)
				}
				if e != nil {
					event(map[string]any{"kind": "dungeon_request_refused", "id": frame.ID, "reason": e.Error()})
					if frame.ID == 69 || frame.ID == 70 {
						continue
					}
					if frame.ID == 71 {
						if sendPayload(1, 71, worldState.cardSnapshot()) != nil {
							return
						}
						continue
					}
					if frame.ID == 72 {
						if request, err := protocol.DecodeSettlementExit(plaintext); err == nil {
							if sendPayload(1, 72, protocol.SettlementExitRefused(request.Option)) != nil {
								return
							}
						}
						continue
					}
					if frame.ID == 43 {
						if request, decodeErr := protocol.DecodePickup(plaintext); decodeErr == nil {
							body, encodeErr := protocol.PickupRefused(request.Object)
							if encodeErr != nil || sendPayload(1, 43, body) != nil {
								return
							}
						}
						continue
					}
					// CMD39 failure reads a monster u16; NOTI132 has no generic
					// command refusal. Never send the generic error shape there.
					if frame.ID == 39 || frame.ID == 46 || frame.ID == 117 || frame.ID == 132 || frame.ID == 2015 || frame.ID == 2062 || frame.ID == 2319 {
						continue
					}
					refusalCode := uint16(4)
					if frame.ID == 1852 {
						// 黑鸦原生应答14525C4C0使用错误8表示无法开始，不套用普通选图错误4。
						refusalCode = 8
					}
					if frame.ID >= 2320 && frame.ID <= 2325 {
						refusalCode = 3
						// 14073D080的错误8专指未参与探索，不能拿来表示邮箱已满。
						if frame.ID == 2322 && e == errBleedingMineClaimActor {
							refusalCode = 8
						}
					}
					if e = sendPayload(1, frame.ID, protocol.Refusal(refusalCode)); e != nil {
						return
					}
					continue
				}
				prepared, e := preparePackets(keys, plan)
				if e != nil {
					event(map[string]any{"kind": "dungeon_encode_error", "error": e.Error()})
					// Domain state may already be committed. Close this failed
					// transport and restore from storage on reconnect.
					return
				}
				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if e = writePackets(c, prepared, func(p preparedPacket) {
					event(map[string]any{"kind": p.Name, "id": p.ID, "character_id": selectedCharacterID, "plain_hex": hex.EncodeToString(p.Payload)})
				}); e != nil {
					return
				}
				if frame.ID == 39 && worldState.completionErr != nil {
					event(map[string]any{"kind": "dungeon_completion_error", "map": worldState.activeDungeon.Room.Map, "error": worldState.completionErr.Error()})
					worldState.completionErr = nil
				}
				if pending != nil && frame.ID == 16 {
					if scene, ok := worldState.townArrivalScenes[pending.Definition.ID]; ok {
						worldState.pendingTownArrival = pending
						worldState.selectingDungeon = false
						worldState.approvedDungeonGate = 0
						event(map[string]any{"kind": "town_arrival_scene_waiting_for_load", "quest": scene.QuestID, "town": scene.Town, "area": scene.Area, "dungeon": scene.DungeonID})
						pending = nil
					}
				}
				if townArrivalLoading {
					worldState.deathSent = map[uint16]bool{}
					worldState.drops = nil
					worldState.resetCards()
					worldState.completionSent = false
					worldState.completionErr = nil
					worldState.resultSent = false
					worldState.pendingTownArrival = nil
					worldState.leaveScene()
					worldState.selectingDungeon = false
					worldState.approvedDungeonGate = 0
					event(map[string]any{"kind": "dungeon_session_started", "dungeon": worldState.activeDungeon.Definition.ID, "maze": worldState.activeDungeon.Maze.Index, "map": worldState.activeDungeon.Room.Map, "monsters": len(worldState.activeDungeon.Monsters), "quests_changed": false, "town_arrival": true})
				}
				if pending != nil {
					if frame.ID == 16 || frame.ID == 72 || frame.ID == 1852 || frame.ID == 2062 {
						worldState.deathSent = map[uint16]bool{}
						worldState.drops = nil
						worldState.resetCards()
					}
					worldState.activeDungeon = pending
					loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
					loyaltyPackets, loyaltyErr := worldState.refreshCreatureLoyalty(loyaltyCtx, time.Now(), true)
					loyaltyCancel()
					if loyaltyErr != nil {
						event(map[string]any{"kind": "creature_loyalty_error", "character_id": selectedCharacterID, "error": loyaltyErr.Error()})
					} else {
						for _, packet := range loyaltyPackets {
							if e := sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
								return
							}
						}
					}
					// A dungeon is a private instance: this actor leaves the shared town.
					worldState.leaveScene()
					worldState.completionSent = false
					worldState.completionErr = nil
					worldState.resultSent = false
					worldState.selectingDungeon = false
					worldState.approvedDungeonGate = 0
					event(map[string]any{"kind": "dungeon_session_started", "dungeon": pending.Definition.ID, "maze": pending.Maze.Index, "map": pending.Room.Map, "monsters": len(pending.Monsters), "quests_changed": false})
				}
				if frame.ID == 37 && worldState.activeDungeon != nil {
					worldState.activeDungeon.Loaded = true
					worldState.activeDungeon.TryComplete()
					if completed, err := worldState.completeDungeon(); err != nil {
						event(map[string]any{"kind": "dungeon_completion_error", "map": worldState.activeDungeon.Room.Map, "error": err.Error()})
					} else if len(completed) > 0 {
						for _, packet := range completed {
							if err = sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
								return
							}
							event(map[string]any{"kind": packet.Name, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload), "character_id": selectedCharacterID})
							if packet.Name == "dungeon_clear_enabled" || packet.Name == "赤红铁矿领主通关确认" {
								worldState.completionSent = true
							}
						}
					}
				}
				for _, p := range plan {
					if p.Name == "solo_party_initialized" {
						worldState.soloPartyReady = true
					}
					if p.Name == "card_scroll_ack" {
						worldState.cardScrolled = true
					}
					if p.Name == "card_layout_ack" {
						if !worldState.cardLayoutSent && worldState.cardReceipt == nil && worldState.activeDungeon != nil &&
							worldState.activeDungeon.Definition.ID == blackPurgatorySquadDungeon {
							worldState.cardAutoPickAt = time.Now().Add(3 * time.Second)
						}
						worldState.cardLayoutSent = true
					}
					if p.Name == "dungeon_return_users" {
						// Back in town: exchange actor info with everyone standing there.
						if e = worldState.announceSelf(event); e != nil {
							event(map[string]any{"kind": "area_presence_error", "error": e.Error()})
						}
					}
					mineEnded := p.Name == "赤红铁矿开战会话结束" && worldState.bleedingMineStart != nil
					if (p.Name == "settlement_exit_ack" || p.Name == "dungeon_leave_ack" || mineEnded) && pending == nil {
						worldState.bleedingMineStart = nil
						worldState.activeDungeon = nil
						loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
						loyaltyPackets, loyaltyErr := worldState.refreshCreatureLoyalty(loyaltyCtx, time.Now(), false)
						loyaltyCancel()
						if loyaltyErr != nil {
							event(map[string]any{"kind": "creature_loyalty_error", "character_id": selectedCharacterID, "error": loyaltyErr.Error()})
						} else {
							for _, packet := range loyaltyPackets {
								if e := sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
									return
								}
							}
						}
						worldState.drops = nil
						worldState.deathSent = nil
						worldState.completionSent = false
						worldState.resultSent = false
						worldState.resetCards()
						if p.Name == "settlement_exit_ack" {
							// selectingDungeon was already set by settlementExit from
							// the decoded request. Never index the outbound
							// acknowledgement again: its width is a protocol detail and
							// reading byte 2 of the native two-byte body is an
							// out-of-range panic.
							event(map[string]any{"kind": "settlement_exit_flag", "character_id": worldState.role.ID, "selecting_dungeon": worldState.selectingDungeon, "payload_len": len(p.Payload)})
						} else {
							worldState.selectingDungeon = false
							worldState.approvedDungeonGate = 0
						}
						// A legion run lives inside a dungeon, so leaving it ends
						// the run. The client normally says so itself; this is
						// the backstop for a player who just walks out (P6).
						if note, closed := legionState.abandonOnLeave(p.Name, worldState.role.ID); closed {
							note["id"] = frame.ID
							event(note)
						}
					}
					if p.Name == "monster_death_confirmed" {
						if worldState.deathSent == nil {
							worldState.deathSent = map[uint16]bool{}
						}
						// Same failure mode as the acknowledgement index above: the
						// read used to be bare indexing on a payload whose width is
						// only guaranteed elsewhere.
						entity, ok := monsterDeathEntity(p.Payload)
						if !ok {
							event(map[string]any{"kind": "monster_death_ack_short_payload", "character_id": worldState.role.ID, "length": len(p.Payload)})
						} else {
							worldState.deathSent[entity] = true
							if worldState.drops != nil && len(worldState.drops.Skipped[entity]) > 0 {
								event(map[string]any{"kind": "drop_rules_pending", "entity": entity, "rules": worldState.drops.Skipped[entity]})
							}
						}
					}
					if p.Name == "dungeon_clear_enabled" || p.Name == "赤红铁矿领主通关确认" {
						worldState.completionSent = true
					}
					if p.Name == "dungeon_clear_reward" {
						worldState.resultSent = true
					}
					// [MERGE-20260928-DIAG] 把场景换图的决策路径落进 events，便于实机取证。
					if p.Name == "dungeon_next_map_sent" && worldState.sceneDiag != "" {
						event(map[string]any{
							"kind": "scene_transition_diag", "character_id": worldState.role.ID,
							"from_map": worldState.sceneDiagFrom, "to_map": worldState.sceneDiagTo,
							"detail": worldState.sceneDiag,
						})
						worldState.sceneDiag = ""
					}
				}
				if frame.ID == 42 {
					worldState.activeDungeon = nil
					loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
					loyaltyPackets, loyaltyErr := worldState.refreshCreatureLoyalty(loyaltyCtx, time.Now(), false)
					loyaltyCancel()
					if loyaltyErr != nil {
						event(map[string]any{"kind": "creature_loyalty_error", "character_id": selectedCharacterID, "error": loyaltyErr.Error()})
					} else {
						for _, packet := range loyaltyPackets {
							if e := sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
								return
							}
						}
					}
					if note, closed := legionState.abandonOnLeave("CMD42 dungeon leave", worldState.role.ID); closed {
						note["id"] = frame.ID
						event(note)
					}
				}
				if frame.ID == 42 || frame.ID == 132 {
					worldState.selectingDungeon = false
					worldState.approvedDungeonGate = 0
				}
				if returnedToTown(plan) {
					refresh, err := worldState.graduateOdysseyAtTown()
					if err != nil {
						event(map[string]any{"kind": "odyssey_graduation_error", "character_id": selectedCharacterID, "reason": err.Error()})
						return
					}
					for _, packet := range refresh {
						if err = sendPayload(packet.Kind, packet.ID, packet.Payload); err != nil {
							return
						}
						event(map[string]any{"kind": packet.Name, "character_id": selectedCharacterID, "id": packet.ID})
					}
					if len(refresh) > 0 {
						if err = worldState.announceSelf(event); err != nil {
							return
						}
					}
				}
				continue
			}
			if worldState != nil && bootstrapped && frame.ID == 191 && verified {
				r, e := protocol.DecodeStoryPause(plaintext)
				if e != nil || worldState.activeDungeon == nil || worldState.role.ID == 0 {
					event(map[string]any{"kind": "story_pause_refused", "reason": "invalid state or absent owned dungeon"})
					continue
				}
				p, e := protocol.StoryPauseNotice(worldState.role.WireID, r)
				if e != nil {
					continue
				}
				if e = sendPayload(0, 170, p); e != nil {
					return
				}
				event(map[string]any{"kind": "story_pause_restored", "character_id": worldState.role.ID, "state": r.State, "story_kind": r.Kind})
				continue
			}
			if worldState != nil && bootstrapped && (frame.ID == 35 || frame.ID == 36 || frame.ID == 1418) {
				if worldState.activeDungeon != nil {
					event(map[string]any{"kind": "dungeon_world_request_pending", "id": frame.ID, "origin_preserved": true})
					continue
				}
				if !verified {
					event(map[string]any{"kind": "world_rejected", "id": frame.ID, "error": "checksum rejected"})
					continue
				}
				if worldState.pendingTownArrival != nil {
					if worldState.isTownArrivalOriginSync(frame.ID, plaintext) {
						event(map[string]any{"kind": "town_arrival_scene_origin_sync", "dungeon": worldState.pendingTownArrival.Definition.ID, "id": frame.ID})
					} else {
						event(map[string]any{"kind": "town_arrival_scene_remains_town", "dungeon": worldState.pendingTownArrival.Definition.ID, "id": frame.ID})
						worldState.pendingTownArrival = nil
					}
				}
				if e := worldState.handle(frame.ID, plaintext, sendPayload, event); e != nil {
					event(map[string]any{"kind": "world_error", "id": frame.ID, "error": e.Error()})
				}
				continue
			}
			if questService != nil && bootstrapped && worldState != nil && (frame.ID == 1422 || frame.ID == 2278) {
				if !verified || worldState.role.ID == 0 || worldState.activeDungeon != nil {
					event(map[string]any{"kind": "act_quest_clear_refused", "id": frame.ID, "reason": "invalid session or checksum"})
					continue
				}
				if frame.ID == 1422 {
					if e := protocol.DecodeClearQuestTicket(plaintext); e != nil {
						event(map[string]any{"kind": "act_quest_clear_refused", "id": frame.ID, "reason": e.Error()})
						continue
					}
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					count, e := questService.ClearActQuests(ctx, worldState.role)
					cancel()
					if e != nil {
						event(map[string]any{"kind": "act_quest_clear_refused", "id": frame.ID, "reason": e.Error()})
						continue
					}
					if e = sendPayload(1, 1422, protocol.ClearQuestTicketAccepted()); e != nil {
						return
					}
					event(map[string]any{"kind": "act_quests_cleared", "character_id": worldState.role.ID, "count": count})
				} else {
					// The native CMD1422 success branch immediately requests CMD2278.
					// Its sender writes no body; publish the quest snapshots here.
					if len(plaintext) != 0 {
						event(map[string]any{"kind": "act_quest_refresh_refused", "reason": "unexpected request body"})
						continue
					}
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					plan, e := worldState.actQuestRefresh(ctx)
					cancel()
					if e != nil {
						event(map[string]any{"kind": "act_quest_refresh_refused", "reason": e.Error()})
						continue
					}
					for _, p := range plan {
						if e = sendPayload(p.Kind, p.ID, p.Payload); e != nil {
							return
						}
						event(map[string]any{"kind": p.Name, "character_id": worldState.role.ID})
					}
				}
				continue
			}
			if questService != nil && bootstrapped && frame.Type == 1 && frame.ID == 467 {
				if !verified || worldState == nil || worldState.role.ID != selectedCharacterID || len(plaintext) != 0 {
					event(map[string]any{"kind": "image_communication_rejected", "reason": "invalid request or session"})
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				qid, npc, lookupErr := questService.ImageCommunicationTarget(ctx, worldState.role)
				cancel()
				if lookupErr != nil {
					event(map[string]any{"kind": "image_communication_rejected", "reason": lookupErr.Error(), "character_id": selectedCharacterID})
					continue
				}
				ack := protocol.ImageCommunicationAck(npc)
				if e := sendPayload(1, 467, ack); e != nil {
					return
				}
				worldState.communicationQuest = qid
				worldState.communicationNPC = npc
				worldState.communicationTown = worldState.state.Position.Town
				worldState.communicationArea = worldState.state.Position.Area
				worldState.communicationUntil = time.Now().Add(20 * time.Second) // PVF [summon time] = 20000 ms
				event(map[string]any{"kind": "image_communication_ack", "character_id": selectedCharacterID, "quest": qid, "npc": npc, "attempt": "2/3", "plain_hex": hex.EncodeToString(ack)})
				continue
			}
			if questService != nil && bootstrapped && frame.ID == 33 && worldState != nil && verified {
				plan, e := worldState.questInteraction(plaintext)
				if diagnostic := worldState.npcPresenceShadow(plaintext); diagnostic != nil {
					if e != nil {
						diagnostic["legacy_outcome"] = "refused"
					} else if len(plan) == 0 {
						diagnostic["legacy_outcome"] = "no_progress"
					} else {
						diagnostic["legacy_outcome"] = "handled"
					}
					event(diagnostic)
				}
				if e != nil {
					event(map[string]any{"kind": "quest_interaction_refused", "reason": e.Error()})
					continue
				}
				for _, p := range plan {
					if e = sendPayload(p.Kind, p.ID, p.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": p.Name, "character_id": worldState.role.ID})
				}
				continue
			}
			if questService != nil && bootstrapped && frame.ID == 34 {
				if !verified || worldState.role.ID == 0 {
					event(map[string]any{"kind": "quest_rejected", "error": "invalid checksum or no character"})
					continue
				}
				r, e := protocol.DecodeQuestSubmit(plaintext)
				if e != nil {
					event(map[string]any{"kind": "quest_rejected", "error": e.Error()})
					continue
				}
				plan, e := worldState.finishQuest(r)
				if e != nil {
					event(map[string]any{"kind": "quest_submit_refused", "character_id": worldState.role.ID, "quest": r.ID, "reason": e.Error()})
					if e = sendPayload(1, 34, protocol.QuestSubmitRefused()); e != nil {
						return
					}
					continue
				}
				prepared, e := preparePackets(keys, plan)
				if e != nil {
					event(map[string]any{"kind": "quest_submit_encode_error", "error": e.Error()})
					return
				}
				if e = writePackets(c, prepared, func(p preparedPacket) {
					event(map[string]any{"kind": p.Name, "character_id": worldState.role.ID, "quest": r.ID})
				}); e != nil {
					event(map[string]any{"kind": "quest_submit_write_error", "quest": r.ID, "error": e.Error()})
					return
				}
				if worldState.answeredQuests == nil {
					worldState.answeredQuests = map[uint16]bool{}
				}
				worldState.answeredQuests[r.ID] = true
				continue
			}
			if questService != nil && bootstrapped && (frame.ID == 31 || frame.ID == 32) {
				if !verified || worldState.role.ID == 0 {
					event(map[string]any{"kind": "quest_rejected", "error": "invalid checksum or no selected character"})
					continue
				}
				qid, e := protocol.DecodeQuestRequest(frame.ID, plaintext)
				if e != nil {
					event(map[string]any{"kind": "quest_rejected", "error": e.Error()})
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				var response []byte
				if frame.ID == 31 {
					var state storage.QuestState
					state, e = questService.Accept(ctx, worldState.role, qid)
					response = protocol.QuestAccepted(qid, state.Progress)
				} else {
					e = characters.Store.AbandonQuest(ctx, developmentAccount, worldState.role.ID, qid)
					response = protocol.QuestAbandoned(qid)
				}
				cancel()
				if e != nil {
					event(map[string]any{"kind": "quest_rejected", "quest": qid, "error": e.Error()})
					continue
				}
				if e = sendPayload(1, frame.ID, response); e != nil {
					return
				}
				event(map[string]any{"kind": "quest_saved_and_sent", "character_id": worldState.role.ID, "quest": qid, "operation": frame.ID, "plain_hex": hex.EncodeToString(response), "client_acceptance": "pending"})
				if frame.ID == 31 {
					// Accepting one [collision quest] branch removes its
					// siblings from the offer list; push the refreshed list so
					// the unchosen faction quests disappear from the client
					// immediately instead of at the next level-up/finish/relog.
					// Must use kind=0 (quest-book update push, same as the
					// finish/CMD2278 refresh paths). kind=1 would route the
					// payload to the client's shop-buy response parser (CMD21
					// is also the buy request opcode) and crash DFO.exe.
					refreshCtx, refreshCancel := context.WithTimeout(context.Background(), 5*time.Second)
					refreshBody, refreshErr := worldState.availableQuestPayload(refreshCtx)
					refreshCancel()
					if refreshErr != nil {
						event(map[string]any{"kind": "quest_available_refresh_error", "quest": qid, "error": refreshErr.Error()})
					} else if e = sendPayload(0, 21, refreshBody); e != nil {
						return
					} else {
						event(map[string]any{"kind": "available_quests_refreshed_after_accept", "character_id": worldState.role.ID, "quest": qid})
					}
					// A quest is normally accepted while standing at the
					// very NPC it names, so its objective can already be
					// satisfied the moment it is accepted.
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					e = worldState.settleProximityObjectives(ctx, sendPayload, event)
					cancel()
					if e != nil {
						return
					}
				}
				continue
			}
			if characters != nil && bootstrapped && frame.ID == 4 && selectProbe != nil {
				if !verified {
					event(map[string]any{"kind": "select_rejected", "error": "checksum or cipher rejected"})
					continue
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				role, e := characters.Select(ctx, developmentAccount, plaintext)
				cancel()
				// Legacy third-awakened saves predate the 5-point VP grant: the
				// panel may show 5 points while the ledger still reads zero, and
				// one ordinary Learn response then clears it. Reconcile on
				// selection; the repair is a no-op once the ledger matches.
				if characters != nil {
					ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
					reconciled, backfilled, reconcileErr := characters.ReconcileTechniquePoints(ctx, role)
					cancel()
					if reconcileErr != nil {
						event(map[string]any{"kind": "technique_points_reconcile_pending", "character_id": role.ID, "reason": reconcileErr.Error()})
					} else {
						role = reconciled
						if backfilled {
							event(map[string]any{"kind": "technique_points_reconciled", "character_id": role.ID})
						}
					}
				}
				if e != nil {
					event(map[string]any{"kind": "select_rejected", "error": e.Error()})
					continue
				}
				if odysseyRewardsEnabled() {
					ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
					updated, applied, rewardErr := grantOdysseyArmor(ctx, characters.Store, wearService, role)
					cancel()
					if rewardErr != nil {
						event(map[string]any{"kind": "odyssey_armor_pending", "character_id": role.ID, "reason": rewardErr.Error()})
					} else {
						role = updated
						if applied {
							event(map[string]any{"kind": "odyssey_armor_granted", "character_id": role.ID, "templates": odysseyArmor})
						}
					}
				}
				if odysseyRewardsEnabled() {
					ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
					updated, applied, rewardErr := grantOdysseyWeaponBox(ctx, characters.Store, role)
					cancel()
					if rewardErr != nil {
						event(map[string]any{"kind": "odyssey_weapon_box_pending", "character_id": role.ID, "reason": rewardErr.Error()})
					} else {
						role = updated
						if applied {
							event(map[string]any{"kind": "odyssey_weapon_box_granted", "character_id": role.ID, "template": 10417789})
						}
					}
				}
				// 创建奖励 10417791 的 [stackable] 块第三行（10418028 x30）。
				// 独立事件键，与上面两项互不干扰；满包/目录未就绪时记 pending，
				// 下次登录自动重试。
				if odysseyRewardsEnabled() {
					if lootService == nil {
						event(map[string]any{"kind": "odyssey_create_potion_pending", "character_id": role.ID, "reason": "loot catalog unavailable"})
					} else {
						ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
						updated, applied, rewardErr := grantOdysseyCreatePotion(ctx, characters.Store, lootService.Catalog, lootService.BagRules, role)
						cancel()
						if rewardErr != nil {
							event(map[string]any{"kind": "odyssey_create_potion_pending", "character_id": role.ID, "reason": rewardErr.Error()})
						} else {
							role = updated
							if applied {
								event(map[string]any{"kind": "odyssey_create_potion_granted", "character_id": role.ID, "template": 10418028, "quantity": 30})
							}
						}
					}
				}
				if odysseyTemporaryCreditsEnabled() {
					ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
					updated, applied, creditErr := grantOdysseyCredits(ctx, characters.Store, role)
					cancel()
					if creditErr != nil {
						event(map[string]any{"kind": "odyssey_test_credits_pending", "reason": creditErr.Error()})
					} else {
						role = updated
						if applied {
							event(map[string]any{"kind": "odyssey_test_credits_granted", "character_id": role.ID, "credits": 10})
						}
					}
				}
				if progressionService != nil && progressionService.Odyssey != nil {
					ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
					caught, repaired, catchErr := progressionService.OdysseyCatchup(ctx, role)
					if catchErr != nil {
						event(map[string]any{"kind": "odyssey_growth_catchup_pending", "character_id": role.ID, "reason": catchErr.Error()})
					} else {
						role = caught
					}
					if repaired {
						event(map[string]any{"kind": "odyssey_growth_catchup_committed", "character_id": role.ID})
					}
					updated, applied, pending := progressionService.OdysseyGifts(ctx, role)
					cancel()
					role = updated
					if applied {
						event(map[string]any{"kind": "odyssey_milestone_gifts_granted", "character_id": role.ID})
					}
					for _, err := range pending {
						event(map[string]any{"kind": "odyssey_milestone_gift_pending", "character_id": role.ID, "reason": err.Error()})
					}
					// 七章奖励：按服务端自有通关成绩补发，逐行独立收据；满包留欠，
					// 下次登录/通关重试。
					ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
					rewarded, chapterApplied, chapterPending := progressionService.OdysseyChapterRewards(ctx, role)
					cancel()
					role = rewarded
					if chapterApplied {
						event(map[string]any{"kind": "odyssey_chapter_rewards_granted", "character_id": role.ID})
					}
					for _, err := range chapterPending {
						event(map[string]any{"kind": "odyssey_chapter_reward_pending", "character_id": role.ID, "reason": err.Error()})
					}
					// Graduation and earlier-level quests share one durable receipt.
					// Honour rewards remain pending mail delivery, never bag grants.
					if questService != nil {
						ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
						graduated, applied, gradErr := questService.GraduateOdyssey(ctx, role)
						cancel()
						if gradErr != nil {
							event(map[string]any{"kind": "odyssey_graduation_error", "character_id": role.ID, "reason": gradErr.Error()})
							continue
						}
						role = graduated
						if applied {
							event(map[string]any{"kind": "odyssey_graduated", "character_id": role.ID, "model": storage.OdysseyGraduationEvent})
						}
					}
				}
				profile := *selectProbe
				if lootService != nil && lootService.Boxes != nil {
					ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
					updated, applied, repairErr := lootService.RepairBoxRewards(ctx, role)
					cancel()
					if repairErr != nil {
						event(map[string]any{"kind": "box_reward_repair_error", "character_id": role.ID, "error": repairErr.Error()})
						continue
					}
					role = updated
					if applied {
						event(map[string]any{"kind": "box_rewards_repaired", "character_id": role.ID})
					}
				}
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				profile.TutorialCompleted, e = characters.Store.TutorialFlags(ctx, developmentAccount, role.ID)
				cancel()
				if e != nil {
					event(map[string]any{"kind": "tutorial_restore_error", "error": e.Error()})
					continue
				}
				// Experiment: native tutorial-progress global dword_14F5AF24 is
				// clamped to >= 0x1E (30) by sub_146CCE300. The leading selectProbe
				// byte (TutorialFlag) initializes it. Sending 1 left it < 30 and the
				// opening recap replayed; sending 30 should clear the gate.
				profile.TutorialFlag = 30
				event(map[string]any{"kind": "tutorial_flags_restored", "character_id": role.ID, "tutorial_flag": profile.TutorialFlag, "completed": profile.TutorialCompleted})
				profile.CreatedTime = uint32(role.CreatedAt.Unix())
				// Cera is an account balance the client reads from this
				// response. Without this it stayed at the configured zero,
				// so an operator top-up could never be seen in game.
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				cera, ce := characters.Store.AccountCera(ctx, developmentAccount)
				cancel()
				if ce != nil {
					event(map[string]any{"kind": "cera_restore_error", "error": ce.Error()})
					continue
				}
				if cera > 0xffffffff {
					cera = 0xffffffff
				}
				profile.Cash = uint32(cera)
				event(map[string]any{"kind": "cera_restored", "account": developmentAccount, "cera": profile.Cash})
				// This loopback probe has one development account. A shared
				// multiplayer world will allocate its own unique actor IDs.
				profile.ActorServerID = role.WireID
				var fatiguePayload []byte
				if fatigueService != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					fp, err := fatigueService.State(ctx, developmentAccount, role.ID, time.Now())
					cancel()
					if err != nil {
						event(map[string]any{"kind": "fatigue_restore_error", "error": err.Error()})
						continue
					}
					profile.Fatigue = [3]uint16{fp.Used, fp.Limit, fp.UsedMax}
					fatiguePayload, err = protocol.Fatigue(fp.Used, fp.Limit, fp.UsedMax)
					if err != nil {
						event(map[string]any{"kind": "fatigue_restore_error", "error": err.Error()})
						continue
					}
					event(map[string]any{"kind": "fatigue_restored", "character_id": role.ID, "state": fp})
				}
				if questService != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					profile.ActiveQuests, e = questService.Active(ctx, role)
					cancel()
					if e != nil {
						event(map[string]any{"kind": "quest_restore_rejected", "error": e.Error()})
						continue
					}
				}
				var basic []byte
				var addition []byte
				var vaultPayload []byte
				var secondaryVaultPayload []byte
				var accountVaultPayload []byte
				if vaultService != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					vaultPayload, e = vaultService.Bootstrap(ctx, role)
					if e == nil {
						secondaryVaultPayload, e = vaultService.BootstrapSpace(ctx, role, 45)
					}
					if e == nil && vaultService.Rules.Account != nil {
						var accountVault storage.AccountVaultState
						accountVault, e = vaultService.Store.LoadAccountVault(ctx, role.AccountID, role.ID)
						if e == nil {
							accountVaultPayload, e = inventory.AccountVaultPayload(accountVault, *vaultService.Rules.Account)
						}
					}
					cancel()
					if e != nil {
						event(map[string]any{"kind": "vault_entry_rejected", "error": e.Error()})
						continue
					}
				}
				var areaPayload []byte
				if *townProbeFile != "" || contractPurchaseCrashFixEnabled() {
					premiumCtx, premiumCancel := context.WithTimeout(context.Background(), 5*time.Second)
					premiums, pe := characters.Store.ActivePremiums(premiumCtx, developmentAccount, time.Now())
					premiumCancel()
					if pe != nil {
						event(map[string]any{"kind": "premium_restore_error", "error": pe.Error()})
						continue
					}
					for _, premium := range premiums {
						profile.Premiums = append(profile.Premiums, protocol.PremiumEntry{Type: premium.Type, RemainingSecond: premium.RemainingSecond})
					}
					event(map[string]any{"kind": "premiums_restored", "account": developmentAccount, "count": len(profile.Premiums)})
				}
				if *townProbeFile != "" {
					var state character.State
					if e = json.Unmarshal(role.State, &state); e != nil || !townCatalog.Allows(state.Level, townPolicy.X, townPolicy.Y) {
						event(map[string]any{"kind": "town_entry_rejected", "error": "character level or spawn policy incompatible with source area"})
						continue
					}
					areaPayload, e = protocol.AreaUsers(townCatalog.TownID, townCatalog.AreaID, []protocol.AreaUser{{ActorServerID: role.WireID, X: townPolicy.X, Y: townPolicy.Y, Flags: townPolicy.Flags}})
					if e != nil {
						event(map[string]any{"kind": "town_entry_error", "error": e.Error()})
						continue
					}
				}
				if *entryBasicProbe {
					basic, e = characters.EntryBasicProbe(role, [2]byte{})
					if e != nil {
						event(map[string]any{"kind": "entry_basic_error", "error": e.Error()})
						continue
					}
					if len(basic) >= 13 {
						event(map[string]any{"kind": "character_mode_projection", "character_id": role.ID, "name": role.Name, "odyssey_pilot": characters.Rules.OdysseyPilot, "entry_mode_byte": basic[len(basic)-13]})
					}
				}
				if *entryAdditionProbe {
					addition, e = characters.EntryAddition(role)
					if e != nil {
						event(map[string]any{"kind": "entry_addition_error", "error": e.Error()})
						continue
					}
				}
				if worldState != nil {
					e = worldState.enter(role, storage.WorldPosition{Town: townCatalog.TownID, Area: townCatalog.AreaID, X: townPolicy.X, Y: townPolicy.Y})
					if e != nil {
						event(map[string]any{"kind": "world_entry_error", "error": e.Error()})
						continue
					}
					// Publish this actor before the area list is serialized, so the list already
					// carries the other players standing in the same place.
					if hub != nil && len(basic) > 0 {
						worldState.peer = &lanPeer{roleID: role.ID, actorID: role.WireID, channel: channel, info: basic, addition: addition, send: sendPayload, mailChanged: mailChanges}
						hub.add(worldState.peer)
						worldState.enterArea()
					}
					areaPayload, e = worldState.areaPayload()
					if e != nil {
						event(map[string]any{"kind": "world_entry_error", "error": e.Error()})
						continue
					}
				}
				payload, e := protocol.SelectProbeSuccess(profile)
				if e != nil {
					event(map[string]any{"kind": "entry_prepare_error", "error": e.Error()})
					continue
				}
				var userArea []byte
				if worldState != nil && len(areaPayload) > 0 {
					userArea, e = worldState.userAreaPayload()
					if e != nil {
						event(map[string]any{"kind": "entry_prepare_error", "error": e.Error()})
						continue
					}
				}
				accountOptions := append([]byte(nil), accountOptionsPayload...)
				if characters != nil {
					optCtx, optCancel := context.WithTimeout(context.Background(), 5*time.Second)
					overrides, optErr := characters.Store.AccountUnifiedOptions(optCtx, developmentAccount)
					optCancel()
					if optErr != nil {
						event(map[string]any{"kind": "account_options_restore_error", "error": optErr.Error()})
					} else if len(overrides) > 0 {
						if accountOptions, optErr = protocol.AccountOptions(overrides); optErr != nil {
							event(map[string]any{"kind": "account_options_restore_error", "error": optErr.Error()})
							accountOptions = append([]byte(nil), accountOptionsPayload...)
						}
					}
					hkCtx, hkCancel := context.WithTimeout(context.Background(), 5*time.Second)
					accHkA, errA := characters.Store.AccountHotkeys(hkCtx, developmentAccount, protocol.UnifiedOptionHotkeys)
					accHkB, errB := characters.Store.AccountHotkeys(hkCtx, developmentAccount, protocol.UnifiedOptionHotkeysExt)
					hkCancel()
					if errA != nil || errB != nil {
						event(map[string]any{"kind": "account_hotkeys_restore_error", "error_a": fmt.Sprint(errA), "error_b": fmt.Sprint(errB)})
					} else if len(accHkA) > 0 || len(accHkB) > 0 {
						if accountOptions == nil {
							var tmplErr error
							accountOptions, tmplErr = protocol.AccountOptions(nil)
							if tmplErr != nil {
								event(map[string]any{"kind": "account_options_template_error", "error": tmplErr.Error()})
							}
						}
						if accountOptions != nil {
							if fe := protocol.FillAccountHotkeys(accountOptions, accHkA, accHkB); fe != nil {
								event(map[string]any{"kind": "account_hotkeys_restore_error", "error": fe.Error()})
							} else {
								event(map[string]any{"kind": "account_hotkeys_restored", "count_a": len(accHkA), "count_b": len(accHkB)})
							}
						}
					}
				}
				plan := entryPayloads{Select: payload, Basic: basic, Addition: addition, Vault: vaultPayload, UserArea: userArea, Area: areaPayload, Fatigue: fatiguePayload, AccountOptions: accountOptions}
				if characters != nil {
					oathCtx, oathCancel := context.WithTimeout(context.Background(), 5*time.Second)
					selection, oathErr := characters.Store.EquippedOathSelection(oathCtx, developmentAccount, role.ID)
					oathCancel()
					if oathErr != nil {
						event(map[string]any{"kind": "oath_selection_restore_error", "character_id": role.ID, "reason": oathErr.Error()})
						continue
					}
					plan.OathSystemInfo, oathErr = protocol.OathSystemInfo(selection.Level, selection.Option)
					if oathErr != nil {
						event(map[string]any{"kind": "oath_selection_restore_error", "character_id": role.ID, "reason": oathErr.Error()})
						continue
					}
				}
				// 装备技能栏/冷却提醒/自定义按键：两组快照（S2C2609）。恒发，
				// 没设过的角色得到全零载荷（等于客户端默认）。
				if characters != nil && equipmentSkillEnabled() {
					eskCtx, eskCancel := context.WithTimeout(context.Background(), 5*time.Second)
					eskSkills, eskCommands, eskErr := characters.Store.EquipmentSkillSnapshots(eskCtx, developmentAccount, role.ID)
					eskCancel()
					if eskErr != nil {
						event(map[string]any{"kind": "equipment_skill_restore_error", "character_id": role.ID, "reason": eskErr.Error()})
					} else if eskInfo, eskInfoErr := protocol.EquipmentSkillInfo(eskSkills, eskCommands); eskInfoErr != nil {
						event(map[string]any{"kind": "equipment_skill_restore_error", "character_id": role.ID, "reason": eskInfoErr.Error()})
					} else {
						plan.EquipmentSkill = eskInfo
					}
				}
				plan.SecondaryVault = secondaryVaultPayload
				collectionCtx, collectionCancel := context.WithTimeout(context.Background(), 5*time.Second)
				collectionEquipment, collectionErr := characters.Store.AdventureCollectionEquipment(collectionCtx, role.AccountID, role.ID)
				collectionCancel()
				if collectionErr != nil {
					event(map[string]any{"kind": "冒险图鉴登录读取失败", "character_id": role.ID, "error": collectionErr.Error()})
					continue
				}
				plan.AdventureCollection = protocol.AdventureCollectionGuide(collectionEquipment)
				// 装备图鉴从已提交的角色状态恢复，不覆盖冒险团及快捷键快照。
				if body, jErr := equipmentJournalEntryPayload(role, journalRules); jErr != nil {
					event(map[string]any{"kind": "equipment_journal_restore_error", "character_id": role.ID, "error": jErr.Error()})
				} else if len(body) > 0 {
					plan.Journal = body
				}
				plan.AccountVault = accountVaultPayload
				if characters != nil {
					gpCtx, gpCancel := context.WithTimeout(context.Background(), 5*time.Second)
					gpPayload, gpErr := characters.Store.ResolveGamepadPayload(gpCtx, developmentAccount, role.ID)
					gpCancel()
					if gpErr != nil {
						event(map[string]any{"kind": "gamepad_options_restore_error", "character_id": role.ID, "error": gpErr.Error()})
					} else if len(gpPayload) > 0 {
						plan.GamepadOptions = gpPayload
					}
				}
				// Restore the persisted category-0 skin state; without owned +
				// selection frames the inventory CharBG keeps its default NEW
				// animation. A failed restore aborts this entry rather than
				// sending a fabricated success state.
				if characters != nil {
					skinCtx, skinCancel := context.WithTimeout(context.Background(), 5*time.Second)
					skinState, skinErr := characters.Store.RestoreProfileSkins(skinCtx, developmentAccount, role.ID)
					skinCancel()
					if skinErr == nil {
						plan.ProfileSkinCargo, plan.ProfileSkinSelection, skinErr = protocol.ProfileSkinRestore(skinState)
					}
					if skinErr != nil {
						event(map[string]any{"kind": "profile_skin_restore_error", "character_id": role.ID, "error": skinErr.Error()})
						continue
					}
				}
				// Feed the account's damage-font cargo so the panel grid has
				// something to enumerate. A read failure only drops this frame:
				// entry must not depend on the skin storage the way the profile
				// decoration state does.
				if characters != nil && skinCatalog != nil {
					cargoCtx, cargoCancel := context.WithTimeout(context.Background(), 5*time.Second)
					cargo, cargoErr := damageFontCargo(cargoCtx, characters.Store, developmentAccount, skinCatalog)
					if cargoErr == nil {
						plan.SkinCargoDamageFont = cargo
						// The chosen fonts are per character and per panel tab, and
						// only these frames put them back on the damage numbers.
						plan.SkinSelectionDamageFontNormal, cargoErr = restoreDamageFontSelection(cargoCtx,
							characters.Store, role.ID, developmentAccount, skinCatalog,
							protocol.SkinSelectionDamageFontNormal)
						if cargoErr == nil {
							plan.SkinSelectionDamageFontCumulative, cargoErr = restoreDamageFontSelection(cargoCtx,
								characters.Store, role.ID, developmentAccount, skinCatalog,
								protocol.SkinSelectionDamageFontCumulative)
						}
					}
					cargoCancel()
					if cargoErr != nil {
						event(map[string]any{"kind": "skin_cargo_damage_font_restore_error", "character_id": role.ID, "reason": cargoErr.Error()})
					}
				}
				// The 边框 and 觉醒插图 pages are filled for the same entry group. Their
				// packets go after the profile pair because a NOTI1545 frame rebuilds the
				// page it names and the last frame for page 0 is the state the client keeps;
				// these payloads carry that feature's built-in rows too, and a selection
				// frame is only sent when this character actually stores one.
				if characters != nil && skinCatalog != nil {
					familyCtx, familyCancel := context.WithTimeout(context.Background(), 5*time.Second)
					plan.restoreSkinFamilies(familyCtx, characters.Store, developmentAccount, role.ID,
						skinCatalog, event)
					familyCancel()
				}
				// 幻化仓库（武器外观页签）容器只在复制时被推过一次，客户端重登即空；
				// 这里按存档重推 NOTI1545。读不出状态只记事件照常进场，仓库空一次
				// 比卡在角色选择界面好。
				//
				// 列表按本职业戴不戴得上过一遍：客户端那一页不做职业判断（见
				// weaponSkinPageIDs），修好复制校验之前存下的条目就一直摆在那里
				// （实机 2026-09-28）。nil 谓词 = 没有装备目录 = 原样发。
				weaponSkinUsable := characters.WeaponSkinUsableFor(role)
				plan.SkinCargo, e = skinCargoRestore(role.State, weaponSkinUsable)
				if e != nil {
					event(map[string]any{"kind": "skin_cargo_restore_error", "error": e.Error()})
					plan.SkinCargo = nil
					e = nil
				}
				// 幻化仓库里正在佩戴那一行的高亮：客户端只在 Apply 时写窗口本地格，
				// 重开窗口就丢。入场补一次 NOTI1546，重登后第一次打开就能看到边框。
				plan.SkinSelection, e = skinSelectionRestore(role.State)
				if e != nil {
					event(map[string]any{"kind": "skin_selection_restore_error", "error": e.Error()})
					plan.SkinSelection = nil
					e = nil
				}
				// 「最近获得」那五行格只由 NOTI1547 喂，而这帧是整表重建，所以入场必须
				// 把账号注册过的皮肤连同本角色复制出的武器外观一次发全。读失败只记事件
				// 不发帧：仓库少一栏条不能把进城卡住。
				if characters != nil && skinCatalog != nil {
					recentCtx, recentCancel := context.WithTimeout(context.Background(), 5*time.Second)
					recent, re := skinRecentRestore(recentCtx, characters.Store, developmentAccount,
						role.State, skinCatalog, weaponSkinUsable)
					recentCancel()
					if re != nil {
						event(map[string]any{"kind": "skin_recent_restore_error",
							"character_id": role.ID, "reason": re.Error()})
					} else {
						plan.SkinRecent = recent
					}
				}
				plan.CubeContract, e = cubeContractRestore(role.State)
				if e != nil {
					event(map[string]any{"kind": "cube_contract_restore_error", "character_id": role.ID, "reason": e.Error()})
					continue
				}
				// Read-notice ledger for NOTI402 (tree 1) and NOTI426 (tree 2).
				// Without it the client re-pops the third-awakening teaching
				// frame on every login; a read failure leaves the frames unset,
				// matching the pre-fix behavior.
				if characters != nil {
					noticeCtx, noticeCancel := context.WithTimeout(context.Background(), 5*time.Second)
					noticeTree1, t1Err := characters.Store.CharacterNoticeSeen(noticeCtx, developmentAccount, role.ID, 1)
					noticeTree2, t2Err := characters.Store.CharacterNoticeSeen(noticeCtx, developmentAccount, role.ID, 2)
					noticeCancel()
					if t1Err == nil {
						plan.InformNotice = protocol.InformNoticeSeen(noticeTree1)
					} else {
						event(map[string]any{"kind": "notice_seen_restore_error", "character_id": role.ID, "tree": 1, "reason": t1Err.Error()})
					}
					if t2Err == nil {
						plan.InformNotice2nd = protocol.InformNoticeSeen(noticeTree2)
					} else {
						event(map[string]any{"kind": "notice_seen_restore_error", "character_id": role.ID, "tree": 2, "reason": t2Err.Error()})
					}
				}
				// Introduce the players already standing here before the area list that
				// places them: the client only places actors it already knows.
				if worldState != nil {
					for _, o := range worldState.joinedPeers {
						plan.Peers = append(plan.Peers, worldState.hub.basicInfo(o))
						if len(o.addition) > 0 {
							plan.Peers = append(plan.Peers, o.addition)
						}
					}
				}
				plan.CinematicSkips, e = cinematicRestore(role.State)
				if e == nil {
					// Same success guard as cinematicRestore: an unparseable
					// state must drop both frames together, never emit the
					// digest without the skip bitmap.
					plan.StoryDigest, e = storyDigestRestore(role.State)
				}
				if e == nil {
					plan.SynopsisRead, e = synopsisRestore(role.State)
				}
				if e == nil && characters != nil {
					plan.SkillVariations, e = characters.VariationRestore(role)
				}
				if e != nil {
					event(map[string]any{"kind": "cinematic_restore_error", "error": e.Error()})
					continue
				}
				// Locked skills ride in the character option block (NOTI2827),
				// which entryPayloads.packets() puts last: this client crashes
				// about 0.3~1s after town entry when 2827 arrives early.
				lockCtx, lockCancel := context.WithTimeout(context.Background(), 5*time.Second)
				locks, lockErr := characters.Store.SkillLocks(lockCtx, role.ID)
				lockCancel()
				if lockErr != nil {
					event(map[string]any{"kind": "entry_skill_lock_error", "error": lockErr.Error()})
					continue
				}
				plan.SkillLocks, e = unifiedCharacPayload(unifiedCharacTemplate, locks, *skillLockOffset)
				if e != nil {
					event(map[string]any{"kind": "entry_skill_lock_error", "error": e.Error()})
					continue
				}
				// Restore per-character system settings (CMD2377 subtype 0x05)
				// onto the fresh NOTI2827 block so toggles survive relog/char switch.
				{
					restoreCtx, restoreCancel := context.WithTimeout(context.Background(), 3*time.Second)
					settings, sErr := characters.Store.CharacterUnifiedOptions(restoreCtx, role.ID)
					restoreCancel()
					if sErr != nil {
						event(map[string]any{"kind": "charac_settings_restore_error", "error": sErr.Error()})
					} else if len(settings) > 0 {
						if fe := protocol.FillCharacSettings(plan.SkillLocks, settings); fe != nil {
							event(map[string]any{"kind": "charac_settings_restore_error", "error": fe.Error()})
						} else {
							event(map[string]any{"kind": "charac_settings_restored", "character_id": role.ID, "count": len(settings)})
						}
					}
				}
				// Restore the six character effect settings carried by CMD2377
				// subtype 0x12 into their own object in NOTI2827.
				{
					effectCtx, effectCancel := context.WithTimeout(context.Background(), 3*time.Second)
					effects, effectErr := characters.Store.CharacterUnifiedOptionGroup(effectCtx, role.ID, protocol.UnifiedOptionCharacterEffects)
					effectCancel()
					if effectErr != nil {
						event(map[string]any{"kind": "charac_effect_options_restore_error", "character_id": role.ID, "error": effectErr.Error()})
					} else if len(effects) > 0 {
						if fe := protocol.FillCharacEffects(plan.SkillLocks, effects); fe != nil {
							event(map[string]any{"kind": "charac_effect_options_restore_error", "character_id": role.ID, "error": fe.Error()})
						} else {
							event(map[string]any{"kind": "charac_effect_options_restored", "character_id": role.ID, "count": len(effects), "options": effects})
						}
					}
				}
				// Restore per-character hotkeys (CMD2377 subtype 0x03 / 0x04)
				// onto the fresh NOTI2827 block.
				if characters != nil && len(plan.SkillLocks) == protocol.UnifiedCharacOptionSize {
					chkCtx, chkCancel := context.WithTimeout(context.Background(), 3*time.Second)
					charHkA, errA := characters.Store.CharacterHotkeys(chkCtx, role.ID, protocol.UnifiedOptionHotkeys)
					charHkB, errB := characters.Store.CharacterHotkeys(chkCtx, role.ID, protocol.UnifiedOptionHotkeysExt)
					chkCancel()
					if errA != nil || errB != nil {
						event(map[string]any{"kind": "charac_hotkeys_restore_error", "character_id": role.ID, "error_a": fmt.Sprint(errA), "error_b": fmt.Sprint(errB)})
					} else if len(charHkA) > 0 || len(charHkB) > 0 {
						if fe := protocol.FillCharacHotkeys(plan.SkillLocks, charHkA, charHkB); fe != nil {
							event(map[string]any{"kind": "charac_hotkeys_restore_error", "character_id": role.ID, "error": fe.Error()})
						} else {
							event(map[string]any{"kind": "charac_hotkeys_restored", "character_id": role.ID, "count_a": len(charHkA), "count_b": len(charHkB)})
						}
					}
				}
				event(map[string]any{"kind": "entry_skill_lock_prepared", "character_id": role.ID, "count": len(locks), "bytes": len(plan.SkillLocks)})
				if lootService != nil {
					// Relocate old stackables before the list-0 inventory snapshot.
					// A failed relocation rolls back and does not prevent entry.
					sweepCtx, sweepCancel := context.WithTimeout(context.Background(), 5*time.Second)
					var applied bool
					var sweepErr error
					var sweptRole storage.Character
					sweptRole, applied, sweepErr = characters.Store.CommitCharacterEvent(sweepCtx, role.AccountID, role.ID, role.ConfigVersion,
						"stack-slot-resweep", "stack-slot-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
							bag, err := inventory.ReadBag(current.State)
							if err != nil {
								return nil, nil, err
							}
							fixed, moved, err := inventory.SweepStackSlots(bag, lootService.Catalog, lootService.BagRules)
							if err != nil {
								return nil, nil, err
							}
							state, err := inventory.SaveBag(current.State, fixed)
							if err != nil {
								return nil, nil, err
							}
							outcome, err := json.Marshal(map[string]bool{"moved": moved})
							return state, outcome, err
						})
					sweepCancel()
					if sweepErr != nil {
						event(map[string]any{"kind": "entry_stack_slot_error", "character_id": role.ID, "error": sweepErr.Error()})
					} else {
						role = sweptRole
						if applied {
							event(map[string]any{"kind": "entry_stack_slot_swept", "character_id": role.ID})
						}
					}
					petCtx, petCancel := context.WithTimeout(context.Background(), 5*time.Second)
					petRole, _, petErr := characters.Store.CommitCharacterEvent(petCtx, role.AccountID, role.ID, role.ConfigVersion,
						"pet-container-resweep", "pet-container-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
							bag, err := inventory.ReadBag(current.State)
							if err != nil {
								return nil, nil, err
							}
							fixed, _, err := inventory.SweepPetConsumables(bag, lootService.Catalog, lootService.BagRules)
							if err != nil {
								return nil, nil, err
							}
							state, err := inventory.SaveBag(current.State, fixed)
							return state, json.RawMessage(`{}`), err
						})
					petCancel()
					if petErr != nil {
						event(map[string]any{"kind": "entry_pet_container_error", "character_id": role.ID, "error": petErr.Error()})
					} else {
						role = petRole
					}
				}
				if wearService != nil && wearService.Catalog != nil {
					gearCtx, gearCancel := context.WithTimeout(context.Background(), 5*time.Second)
					gearRole, applied, gearErr := characters.Store.CommitCharacterEvent(gearCtx, role.AccountID, role.ID, role.ConfigVersion,
						"pet-gear-resweep", "pet-gear-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
							bag, err := inventory.ReadBag(current.State)
							if err != nil {
								return nil, nil, err
							}
							fixed, _, err := inventory.SweepPetGear(bag, wearService.Catalog)
							if err != nil {
								return nil, nil, err
							}
							state, err := inventory.SaveBag(current.State, fixed)
							return state, json.RawMessage(`{}`), err
						})
					gearCancel()
					if gearErr != nil {
						event(map[string]any{"kind": "entry_pet_gear_error", "character_id": role.ID, "error": gearErr.Error()})
					} else {
						role = gearRole
						if applied {
							event(map[string]any{"kind": "entry_pet_gear_swept", "character_id": role.ID})
						}
					}
				}
				loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
				loyaltyRole, _, loyaltyErr := characters.Store.CommitCharacterEvent(loyaltyCtx, role.AccountID, role.ID, role.ConfigVersion,
					fmt.Sprintf("creature-loyalty-login:%d", time.Now().UnixNano()), "creature-loyalty-login-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
						state, err := inventory.BeginCreatureLoyaltySession(current.State, time.Now().Unix())
						return state, json.RawMessage(`{}`), err
					})
				loyaltyCancel()
				if loyaltyErr != nil {
					event(map[string]any{"kind": "entry_creature_loyalty_error", "character_id": role.ID, "error": loyaltyErr.Error()})
				} else {
					role = loyaltyRole
				}
				if wearService != nil {
					plan.KnightDeck, e = wearService.KnightDeckPayload(role)
					if e != nil {
						event(map[string]any{"kind": "entry_knight_deck_error", "character_id": role.ID, "error": e.Error()})
						plan.KnightDeck = nil
					}
					plan.Worn, e = inventory.WornPayload(role.State)
					if e == nil {
						plan.WornUpdate, e = inventory.WornSpaceUpdate(role.State)
					}
					if e == nil {
						// Full worn set through the id-14 slot-update channel,
						// mirroring the equipment-move heal frames (see the
						// third-pass note in entry_flow.go packets()).
						var bag inventory.Bag
						bag, e = inventory.ReadBag(role.State)
						if e == nil && len(bag.Worn) > 0 {
							// Coexisting clone/look avatars share a body slot; the
							// id-14 slot channel carries one row per slot.
							plan.WornSlots, e = inventory.EquipmentPayload(3, bag.WornBaseItems(), false)
						}
					}
					if e == nil {
						plan.WeaponEquipped = inventory.HasWornWeapon(role.State)
						if plan.WeaponEquipped {
							plan.WeaponAppearance, e = characters.AppearanceProbe(role, [2]byte{})
						}
					}
					if e == nil {
						plan.AvatarReady, e = inventory.SpecialEquipmentRestorePayload(role.State, 1)
					}
					if e == nil {
						plan.Avatars, e = inventory.EquipmentPayload(1, nil, true)
					}
					if e == nil {
						plan.Creatures, e = inventory.PetContainerRestorePayload(role.State)
					}
					if e == nil {
						plan.CreatureList, _ = inventory.CreatureListPayload(role.State)
						plan.CreatureGrowth, _ = inventory.CreatureGrowthPayload(role.State)
					}
					if e != nil {
						event(map[string]any{"kind": "entry_worn_error", "error": e.Error()})
						continue
					}
				}
				if lootService != nil {
					rewardCtx, rewardCancel := context.WithTimeout(context.Background(), 5*time.Second)
					recovered, rewardErr := lootService.RecoverBlackPurgatoryCards(rewardCtx, role)
					rewardCancel()
					role = recovered
					if rewardErr != nil {
						event(map[string]any{"kind": "黑鸦未领翻牌或领主奖励保留", "character_id": role.ID, "error": rewardErr.Error()})
					}
					// Sweep the seventeen account-shared materials out of the bag
					// into the account storage before the snapshots are built, then
					// deliver the list35 storage snapshot ahead of list0 so the
					// client harvest (sub_145ADC2A0) adopts the fixed slots.
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					var materials inventory.AccountMaterials
					role, materials, e = sweepAccountMaterials(ctx, characters.Store, role)
					cancel()
					if e != nil {
						event(map[string]any{"kind": "entry_account_materials_error", "error": e.Error()})
						materials = inventory.NewAccountMaterials()
						fallbackCtx, fallbackCancel := context.WithTimeout(context.Background(), 5*time.Second)
						if raw, readErr := characters.Store.AccountMaterials(fallbackCtx, role.AccountID); readErr == nil {
							if savedMaterials, parseErr := inventory.ReadAccountMaterials(raw); parseErr == nil {
								materials = savedMaterials
							}
						}
						fallbackCancel()
						e = nil
					}
					plan.AccountMaterials, e = accountMaterialSnapshot(materials)
					if e == nil {
						plan.RadiantSouls, e = radiantSoulSnapshot(materials)
					}
					if e == nil {
						plan.Inventory, e = lootService.Bootstrap(role)
					}
					if e != nil {
						event(map[string]any{"kind": "entry_inventory_error", "error": e.Error()})
						continue
					}
				}
				if progressionService != nil {
					plan.OdysseyProgress, e = progressionService.OdysseyProgressPayload(role)
					if e != nil {
						event(map[string]any{"kind": "entry_odyssey_progress_error", "error": e.Error()})
						continue
					}
					plan.Experience, e = character.ExperiencePayload(role)
					if e != nil {
						event(map[string]any{"kind": "entry_experience_error", "error": e.Error()})
						continue
					}
					if questService != nil {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						ids, qe := questService.Completed(ctx, role)
						if qe == nil {
							plan.CompletedQuests, qe = protocol.CompletedQuests(ids)
						}
						if qe == nil {
							available, ae := questService.Available(ctx, role)
							if ae != nil {
								qe = ae
							} else {
								plan.AvailableQuests, qe = protocol.AvailableQuests(plan.Experience[0], available)
							}
						}
						cancel()
						if qe != nil {
							event(map[string]any{"kind": "entry_completed_quests_error", "error": qe.Error()})
							continue
						}
					}
				}
				if len(addition) > 0 {
					plan.Skills, e = characters.EntrySkills(role)
					if e != nil {
						event(map[string]any{"kind": "entry_skills_error", "error": e.Error()})
						continue
					}
					plan.ComboSkillInfo, e = characters.ComboSkillInfoNotify(role)
					if e != nil {
						event(map[string]any{"kind": "entry_combo_skill_info_error", "error": e.Error()})
						continue
					}
					plan.SkillPreset, e = characters.SkillPresetInfo(role)
					if e != nil {
						event(map[string]any{"kind": "entry_skill_preset_error", "error": e.Error()})
						continue
					}
				}
				if len(addition) > 0 && len(areaPayload) > 0 {
					plan.Complete = protocol.EnterGameworldComplete()
				}
				if *boosterGageHide {
					// NOTI398 displayValue=0 collapses the top-left Liberation Trace
					// panel; preparePackets skips empty payloads, so the flag-off path
					// equals the pre-fix behavior.
					plan.BoosterGage = protocol.BoosterGage(0)
				}
				prepared, e := preparePackets(keys, plan.packets())
				if e != nil {
					event(map[string]any{"kind": "entry_encode_error", "character_id": role.ID, "error": e.Error(), "frames_sent": 0})
					continue
				}
				event(map[string]any{"kind": "entry_preflight_passed", "character_id": role.ID, "frame_count": len(prepared)})
				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				e = writePackets(c, prepared, func(p preparedPacket) {
					entry := map[string]any{"kind": p.Name, "character_id": role.ID, "actor_server_id": role.WireID, "type": p.Kind, "id": p.ID, "plain_bytes": len(p.Payload), "client_acceptance": "pending"}
					if p.ID == 4 {
						entry["name"] = role.Name
					}
					if p.ID == 23 || p.ID == 24 {
						if worldState != nil {
							entry["position"] = worldState.state.Position
						} else {
							entry["town_id"], entry["area_id"] = townCatalog.TownID, townCatalog.AreaID
						}
					}
					if p.ID == 13 || p.ID == 36 || p.ID == 2425 || (p.Kind == 0 && p.ID == 433) {
						entry["plain_hex"] = hex.EncodeToString(p.Payload)
					}
					event(entry)
				})
				if e != nil {
					event(map[string]any{"kind": "entry_write_error", "character_id": role.ID, "error": e.Error()})
					return
				}
				comboState.lastNotify = nil
				selectedCharacterID = role.ID
				if worldState != nil {
					worldState.fameInitialized = false
					for _, packet := range worldState.appendFameUpdate(nil, event) {
						if e = sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
							return
						}
					}
				}
				selectedBasic, selectedAddition = basic, addition
				mailAlarmRole, mailDeliveryID = 0, 0
				select {
				case mailChanges <- struct{}{}:
				default:
				}
				if worldState != nil {
					if e = worldState.announceSelf(event); e != nil {
						event(map[string]any{"kind": "area_presence_error", "error": e.Error()})
					}
				}
				// 进城镇好感度全量同步：NOTI733(NPC_FAVOR_POINT_INFO) 是客户端
				// 唯一的无弹窗全量装载入口（handler 0x1452db190：先清空 favor
				// map 再逐条装入并刷新，不派发任何 UI 事件）；806 ack 虽也写
				// 缓存但必弹好感度窗。NOTI124 刚完成时好感度子系统尚未就绪，
				// 早发会被丢弃，沿用 900ms 延迟（2026-09-29 定案时序）。
				go func(characterID int64) {
					time.Sleep(900 * time.Millisecond)
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					points, listErr := characters.Store.ListFavor(ctx, characterID)
					cancel()
					if listErr != nil {
						event(map[string]any{"kind": "npc_favor_point_info_skipped", "character_id": characterID, "error": listErr.Error()})
						return
					}
					if len(points) == 0 {
						return
					}
					records := make([]protocol.FavorPointInfoRecord, 0, len(points))
					for _, fp := range points {
						records = append(records, protocol.FavorPointInfoRecord{NPCID: fp.NPCID, Point: uint32(fp.Point)})
					}
					payload := protocol.FavorPointInfo(records)
					if err := sendPayload(0, 733, payload); err != nil {
						event(map[string]any{"kind": "npc_favor_point_info_failed", "character_id": characterID, "error": err.Error()})
						return
					}
					event(map[string]any{"kind": "npc_favor_point_info_sent", "character_id": characterID, "npc_count": len(records), "plain_bytes": len(payload)})
				}(role.ID)
				continue
			}
			if characters != nil && bootstrapped && frame.ID == 295 {
				if !verified {
					event(map[string]any{"kind": "character_slot_rejected", "error": "request checksum or cipher rejected"})
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				err := changeRosterSlot(ctx, characters, developmentAccount, selectedCharacterID, plaintext)
				cancel()
				payload := protocol.CharacterSlotSuccess()
				if err != nil {
					event(map[string]any{"kind": "character_slot_rejected", "error": err.Error()})
					payload = protocol.Refusal(19)
				} else {
					event(map[string]any{"kind": "character_slot_saved", "account_id": developmentAccount, "plain_hex": hex.EncodeToString(plaintext)})
				}
				if e := sendPayload(1, 295, payload); e != nil {
					return
				}
				// The native drag already updates its local maps. Do not reload
				// the roster between the packets of a multi-cell drag operation.
				// A refused operation leaves those maps ahead of storage; close
				// this session so it cannot select/archive a different character.
				if err != nil {
					return
				}
				continue
			}
			if characters != nil && bootstrapped && frame.ID == 637 {
				if verified && len(plaintext) == 0 && selectedCharacterID == 0 {
					// 145250040 reads the count of pending delayed deletions.
					// Local archival is immediate, so no pending timer rows.
					if e := sendPayload(1, 637, []byte{1, 0}); e != nil {
						return
					}
				}
				continue
			}
			if characters != nil && bootstrapped && (frame.ID == 433 || frame.ID == 848) {
				if !verified {
					event(map[string]any{"kind": "roster_followup_rejected", "id": frame.ID, "error": "checksum or cipher rejected"})
					continue
				}
				var payload []byte
				var e error
				if frame.ID == 433 {
					payload, e = protocol.EmptyMercenaryInfo(plaintext)
				} else {
					payload, e = protocol.ProbeRosterCounters(plaintext)
				}
				if e != nil {
					event(map[string]any{"kind": "roster_followup_rejected", "id": frame.ID, "error": e.Error()})
					continue
				}
				encrypted, e := wire.EncryptPayload(keys, frame.ID, payload)
				if e != nil {
					event(map[string]any{"kind": "roster_followup_error", "error": e.Error()})
					return
				}
				response, e := wire.ServerFrame(1, frame.ID, encrypted)
				if e != nil {
					return
				}
				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if _, e = io.Copy(c, bytes.NewReader(response)); e != nil {
					return
				}
				event(map[string]any{"kind": "roster_followup_response", "id": frame.ID, "hex": hex.EncodeToString(response)})
				continue
			}
			if characters != nil && bootstrapped && frame.ID == 1725 {
				if !verified || selectedCharacterID != 0 {
					event(map[string]any{"kind": "roster_background_rejected", "error": "背景选择需要有效校验及选角状态"})
					continue
				}
				req, e := protocol.DecodeSelectRosterBackground(plaintext)
				if e == nil {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					_, e = characters.Store.SelectRosterBackground(ctx, developmentAccount, req.Page, req.Background)
					cancel()
				}
				if e != nil {
					event(map[string]any{"kind": "roster_background_rejected", "error": e.Error()})
				} else {
					event(map[string]any{"kind": "roster_background_selected", "page": req.Page, "category": req.Background.Category, "background_id": req.Background.ID})
				}
				// 原生按钮会乐观应用选择；拒绝时也恢复账号的权威状态，不编造未知 ACK。
				if e = sendRosterBackgrounds(); e != nil {
					return
				}
				continue
			}
			if characters != nil && bootstrapped && (frame.ID == 5 || frame.ID == 6 || frame.ID == 684 || frame.ID == 8) {
				if !verified {
					event(map[string]any{"kind": "character_rejected", "id": frame.ID, "error": "request checksum or cipher unsupported"})
					return
				}
				var userInfoMode byte
				if frame.ID == 8 {
					uid, mode, e := protocol.DecodeUserInfoRequest(plaintext)
					if e != nil {
						event(map[string]any{"kind": "userinfo_rejected", "error": e.Error()})
						continue
					}
					userInfoMode = mode
					if !((mode == 2 && uid == 0xffff) || (mode == 0 && uid == 0xffff && len(selectedBasic) > 0) || (mode == 1 && uid == 0xffff && len(selectedAddition) > 0)) {
						event(map[string]any{"kind": "userinfo_mode_pending", "mode": mode, "uid": uid, "selected_character_id": selectedCharacterID})
						continue
					}
				}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				var payload []byte
				created := false
				var err error
				kind, id := byte(1), frame.ID
				switch frame.ID {
				case 6:
					var req protocol.DeleteCharacterRequest
					var deletedID int64
					req, err = protocol.DecodeDeleteCharacter(plaintext)
					if err == nil && selectedCharacterID != 0 {
						err = fmt.Errorf("delete requires character selection screen")
					}
					if err == nil {
						deletedID, err = characters.Store.DeleteCharacter(ctx, developmentAccount, req.Slot, req.Name)
					}
					if err == nil {
						created = true // refresh complete roster after the native removal callback
						payload = protocol.DeleteCharacterSuccess(req.Slot)
						event(map[string]any{"kind": "character_archived", "character_id": deletedID, "slot": req.Slot, "name": req.Name})
					}
				case 684:
					payload, err = characters.CheckName(ctx, plaintext)
				case 5:
					var role storage.Character
					role, err = characters.Create(ctx, developmentAccount, plaintext)
					if err == nil {
						var slot uint16
						slot, err = characters.RosterSlot(ctx, developmentAccount, role.ID)
						if err == nil {
							created = true
							payload = protocol.CreateSuccess(slot, role.Name)
							// Only a character created from here on owes a
							// starting route; every earlier character was
							// backfilled as already finished.
							owed := "not tracked"
							if tutorialRoutes != nil {
								if e := characters.Store.StartBirth(ctx, developmentAccount, role.ID); e != nil {
									owed = "record failed: " + e.Error()
								} else {
									owed = "pending"
								}
							}
							event(map[string]any{"kind": "character_committed", "id": role.WireID, "slot": slot, "name": role.Name, "profession": role.Profession, "config_version": role.ConfigVersion, "starting_route": owed})
						}
					}
				case 8:
					kind, id = 0, 2
					if userInfoMode == 0 {
						payload = selectedBasic
						if worldState != nil && worldState.role.ID == selectedCharacterID {
							payload, err = characters.EntryBasicProbe(worldState.role, [2]byte{})
						}
					} else if userInfoMode == 1 {
						payload = selectedAddition
						if worldState != nil && worldState.role.ID == selectedCharacterID {
							payload, err = characters.EntryAddition(worldState.role)
						}
					} else {
						payload, err = characters.ListWithFatigue(ctx, developmentAccount, fatigueService, time.Now())
					}
				}
				cancel()
				if err != nil {
					event(map[string]any{"kind": "character_rejected", "id": frame.ID, "error": err.Error()})
					// Both native creation/name handlers explicitly handle code 2
					// and restore input state. Never drop a valid connection for
					// a business refusal; other code meanings remain unverified.
					kind, id = 1, frame.ID
					payload = protocol.Refusal(2)
				}
				ciphertext, err := wire.EncryptPayload(keys, id, payload)
				if err != nil {
					event(map[string]any{"kind": "character_error", "error": err.Error()})
					return
				}
				response, err := wire.ServerFrame(kind, id, ciphertext)
				if err != nil {
					return
				}
				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if _, err = io.Copy(c, bytes.NewReader(response)); err != nil {
					return
				}
				event(map[string]any{"kind": "character_response", "id": id, "bytes": len(response), "hex": hex.EncodeToString(response)})
				// NOTI2 先建立选角管理器，再由 NOTI1759 初始化背景列表和五页选择。
				if frame.ID == 8 && userInfoMode == 2 && kind == 0 && id == 2 {
					if err = sendRosterBackgrounds(); err != nil {
						return
					}
				}
				if created {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					list, e := characters.ListWithFatigue(ctx, developmentAccount, fatigueService, time.Now())
					cancel()
					if e != nil {
						event(map[string]any{"kind": "character_list_error", "error": e.Error()})
						continue
					}
					encrypted, e := wire.EncryptPayload(keys, 2, list)
					if e != nil {
						return
					}
					notification, e := wire.ServerFrame(0, 2, encrypted)
					if e != nil {
						return
					}
					c.SetWriteDeadline(time.Now().Add(5 * time.Second))
					if _, e = io.Copy(c, bytes.NewReader(notification)); e != nil {
						return
					}
					event(map[string]any{"kind": "character_list_after_mutation", "request": frame.ID, "id": 2, "bytes": len(notification)})
					if e = sendRosterBackgrounds(); e != nil {
						return
					}
				}
				continue
			}
			if response, ok := responses[frame.ID]; ok {
				if frame.ID == 1 && channelNotice != nil {
					response, err = channelLoginResponse(keys, response, channelNotice[12])
					if err != nil {
						event(map[string]any{"kind": "channel_login_error", "error": err.Error()})
						return
					}
				}
				if frame.ID == 1 && moonConfig != nil && channel == moonConfig.Channel {
					var e error
					response, e = moonLoginResponse(response, keys)
					if e != nil {
						event(map[string]any{"kind": "moon_login_error", "error": e.Error()})
						return
					}
				}
				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if _, err := io.Copy(c, bytes.NewReader(response)); err != nil {
					event(map[string]any{"kind": "write_error", "error": err.Error()})
					return
				}
				event(map[string]any{"kind": "server_response", "peer": peer, "id": frame.ID, "hex": hex.EncodeToString(response)})
				if frame.ID == 1 {
					bootstrapped = true
					// 固定 CMD1 登录模板不经过 NOTI1 的服务器时钟初始化。
					// 145257F70 是已注册的 CMD1960 原生同步入口；1459A2D70
					// 按 opcode 直接分发，无请求等待态依赖。必须在角色/UI包前
					// 建立时钟，不能等冒险团日期判断已访问空指针后再补发。
					if err = sendServerTime("登录初始化"); err != nil {
						return
					}
					if channelNotice != nil {
						if err = sendPayload(0, 2435, channelNotice); err != nil {
							return
						}
						event(map[string]any{"kind": "channel_identity_sent", "server": channelCfg.ServerID, "channel": channel})
					}
				}
			}
		}
	}
	for _, set := range listeners {
		go func(set channelListener) {
			for {
				c, err := set.ln.Accept()
				if err != nil {
					log.Print(err)
					return
				}
				go handleClient(c, set.channel)
			}
		}(set)
	}
	select {}
}

// unifiedEntries converts a decoded CMD2377 block into the storage shape so
// account (0x01) and character (0x05) settings can be persisted durably.
func unifiedEntries(entries []protocol.UnifiedOptionEntry) []storage.UnifiedOptionEntry {
	out := make([]storage.UnifiedOptionEntry, 0, len(entries))
	for _, e := range entries {
		out = append(out, storage.UnifiedOptionEntry{Position: e.Position, Value: e.Value})
	}
	return out
}
