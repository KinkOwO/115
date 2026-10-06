package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"dfolan/internal/quest"
	"dfolan/internal/reward"
	"dfolan/internal/workflow"
	"dfolan/internal/world"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type gatewayRuntime struct {
	config                Config
	accountOptionsPayload []byte
	apocalypseCatalog     *catalog.ApocalypseCatalog
	apocalypseClock       *legion.ApocalypseClock
	boosterCatalog        *BoosterCatalog
	boostCatalog          *boostup.Catalog
	boostEventInfo        []byte
	characters            *character.Service
	channelDirectory      *catalog.ChannelDirectory
	channelTowns          map[uint32]catalog.TownArea
	channelGuides         map[uint32]uint32
	channelInfo           *catalog.ChannelInfo
	developmentAccount    int64
	dungeonCatalog        *catalog.DungeonCatalog
	fatigueService        *character.FatigueService
	gameHost              string
	gameStore             *database.Store
	hub                   *lanHub
	itemService           *inventory.ItemService
	journalRules          *catalog.EquipmentJournalRules
	lootService           *loot.Service
	lotteryPools          *lotteryItemCatalog
	moonConfig            *moonSoloConfig
	oathGradePair         [2]uint16
	oathGradeTable        *inventory.OathGradeTable
	oathInjectSpecs       []oathInjectSpec
	oathProgressSet       map[uint32]bool
	odysseyChoices        odysseyWeaponChoices
	omenInfoBytes         []byte
	omenState             bool
	progressionService    *character.ProgressionService
	questService          *quest.Service
	raw                   []byte
	responses             map[uint16][]byte
	rewards               reward.Notifier
	selectProbe           *protocol.SelectProbeState
	selectionBoxes        *catalog.SelectionBoxes
	shopPilot             *cashshop.Pilot
	shopService           *workflow.ShopService
	skinCatalog           map[uint32]catalog.SkinStorageEntry
	townArrivalScenes     map[uint32]catalog.TownArrivalScene
	townCatalog           catalog.TownArea
	townPolicy            townEntryPolicy
	tutorialDungeons      *catalog.DungeonCatalog
	tutorialRoutes        *catalog.TutorialCatalog
	unifiedCharacTemplate []byte
	unsealService         *workflow.UnsealService
	vaultService          *workflow.VaultService
	wearService           *workflow.WearService
	worldService          *world.Service
}

type townEntryPolicy struct {
	X     uint16  `json:"x"`
	Y     uint16  `json:"y"`
	Flags [3]byte `json:"flags"`
}

// prepareRuntime preserves catalog, migration and service assembly order.
// A nil runtime with no error means the explicit PVF check completed.
// On failure (or check mode), acquired resources close before returning.
func prepareRuntime(startup Config) (prepared *gatewayRuntime, cleanup func(), prepareErr error) {
	if err := startup.validate(); err != nil {
		return nil, nil, err
	}
	resources := new(runtimeCleanup)
	defer func() {
		if prepared == nil {
			resources.close()
		}
	}()
	// Confirmed gameplay behavior is not configurable.
	omenRewards := true
	omenState := true
	pvfCatalogs, pvfCatalogErr := gamedata.PrepareCatalogs(gamedata.CatalogInputs{
		Selection:              startup.PVFCatalogs,
		ArchivePath:            startup.PVFArchive,
		ArchiveChecksum:        startup.PVFSHA256,
		CharacterPath:          startup.CharacterCatalog,
		QuestPath:              startup.QuestCatalog,
		ProgressionPath:        startup.ProgressionCatalog,
		WorldPath:              startup.WorldCatalog,
		DerivedCacheDir:        startup.PVFCacheDir,
		ItemShopPath:           startup.ItemShop,
		ItemShopPolicyPath:     startup.PVFItemShopPolicy,
		BoxesPath:              startup.Boxes,
		BoxPolicyPath:          startup.PVFBoxPolicy,
		CashshopRelease:        startup.ShopRelease,
		CharacterPolicyPath:    startup.PVFCharacterPolicy,
		LayerRevisitPolicyPath: startup.PVFLayerRevisitPolicy,
		ScriptWarpPolicyPath:   startup.PVFScriptWarpPolicy,
		LotteryPolicyPath:      startup.PVFLotteryPolicy,
		SelectionPolicyPath:    startup.PVFSelectionPolicy,
		MinePath:               startup.BleedingMineRewards,
		IndexPath:              startup.ItemIndex,
		FullPrefix:             startup.EquipmentFullCatalog,
		JournalPath:            startup.EquipmentJournalRules,
		CreateCostPath:         startup.EquipmentCreateCost,

		TutorialPath:          startup.TutorialRoutes,
		VerifyBaselines:       startup.PVFVerifyBaselines,
		EnhancementPolicyPath: startup.PVFEnhancementPolicy,
		RandomOptionPath:      startup.RandomOptionCatalog,
		ShieldPath:            startup.KnightShieldCatalog,
		WearRulesPath:         startup.EquipmentWearRules,
		OathPath:              startup.OathGradesTable,
		VaultPath:             startup.VaultRules,
		VaultPolicyPath:       startup.PVFVaultPolicy,
		LootPath:              startup.LootCatalog,
		EquipmentPath:         startup.EquipmentCatalog,
		QuestEquipmentPath:    startup.QuestEquipmentCatalog,
		DropPolicyPath:        startup.PVFDropPolicy,
		TownPath:              startup.TownCatalog,
		TutorialDungeonPath:   startup.TutorialDungeons,
		ScenePolicyPath:       startup.PVFScenePolicy,
		ApocalypsePath:        startup.ApocalypseCatalog,
		AttunementPath:        startup.AttunementRewards,
		ContentPolicyPath:     startup.PVFContentPolicy,
		BoostChallenge:        startup.BoostUpChallenge,
	}, runtimeCatalogAdapters())
	// PrepareCatalogs can return partially acquired catalogs alongside an error.
	if pvfCatalogs != nil {
		if pvfCatalogs.Dungeons != nil {
			resources.add(func() { pvfCatalogs.Dungeons.CloseMapSource() })
		}
		if pvfCatalogs.Learning != nil {
			resources.add(func() { pvfCatalogs.Learning.Close() })
		}
		if pvfCatalogs.Quests != nil {
			resources.add(func() { pvfCatalogs.Quests.Close() })
		}
		if pvfCatalogs.Loot != nil {
			resources.add(func() { pvfCatalogs.Loot.CloseDetails() })
		}
		if pvfCatalogs.Equipment != nil {
			resources.add(func() { pvfCatalogs.Equipment.Close() })
		}
	}
	if pvfCatalogErr != nil {
		return nil, nil, fmt.Errorf("PVF candidate catalogs: %v", pvfCatalogErr)
	}
	if candidateSkills := os.Getenv("DFO_SKILL_CATALOG"); candidateSkills != "" {
		startup.SkillCatalog = candidateSkills
	}
	if startup.SkillCatalog != "" && pvfCatalogs.Learning == nil {
		return nil, nil, fmt.Errorf("skills require the native PVF skills domain")
	}
	if startup.LootCatalog != "" && (pvfCatalogs.Loot == nil || pvfCatalogs.Selection == nil || !pvfCatalogs.Selected("loot") || !pvfCatalogs.Prepared("loot") || !pvfCatalogs.Selected("equipment-selection") || !pvfCatalogs.Prepared("equipment-selection")) {
		return nil, nil, fmt.Errorf("loot requires native PVF loot and equipment-selection domains")
	}
	if startup.QuestEquipmentCatalog != "" && (pvfCatalogs.Selection == nil || !pvfCatalogs.Selected("equipment-selection") || !pvfCatalogs.Prepared("equipment-selection")) {
		return nil, nil, fmt.Errorf("quest equipment requires native PVF equipment-selection domain")
	}
	if startup.LootCatalog != "" && (!pvfCatalogs.Selected("materials") || !pvfCatalogs.Prepared("materials") || pvfCatalogs.Materials == nil) {
		return nil, nil, fmt.Errorf("loot requires the prepared native PVF materials domain")
	}
	if startup.LootCatalog != "" && pvfCatalogs.Enhancements == nil {
		return nil, nil, fmt.Errorf("enhancements require the native PVF enhancements domain")
	}
	if (os.Getenv("DFO_DUNGEON_CATALOG") != "" || os.Getenv("DFO_ODYSSEY_DUNGEON_CATALOG") != "") && pvfCatalogs.Dungeons == nil {
		return nil, nil, fmt.Errorf("dungeons require the native PVF dungeons domain")
	}
	// Reject retired content paths before opening database. Old flag names remain
	// compatible only when their native domain has actually been prepared.
	if startup.BoosterCatalog != "" {
		if _, err := pvfCatalogs.LoadBooster(startup.BoosterCatalog, startup.ItemIndex); err != nil {
			return nil, nil, err
		}
	}
	if (startup.BoosterCatalog != "" || pvfCatalogs.Boosters != nil || pvfCatalogs.Prepared("boosters")) && startup.ItemIndex != "" && pvfCatalogs.LotteryTables == nil {
		return nil, nil, fmt.Errorf("lottery requires the native PVF lottery domain")
	}
	if startup.SelectionBoxes != "" {
		if _, err := pvfCatalogs.LoadSelectionBoxes(startup.SelectionBoxes); err != nil {
			return nil, nil, err
		}
	}
	if startup.ShopPrices != "" {
		if _, err := pvfCatalogs.LoadShopPrices(startup.ShopPrices, pvfCatalogs.SourceChecksum); err != nil {
			return nil, nil, err
		}
	}
	if startup.ItemIndex != "" && pvfCatalogs.Items == nil {
		return nil, nil, fmt.Errorf("item index requires the native PVF items domain")
	}
	if startup.EquipmentFullCatalog != "" && pvfCatalogs.Equipment == nil {
		return nil, nil, fmt.Errorf("full equipment requires the native PVF equipment domain")
	}
	if startup.WorldCatalog != "" && pvfCatalogs.World == nil {
		return nil, nil, fmt.Errorf("world requires the native PVF world domain")
	}
	if startup.QuestCatalog != "" && pvfCatalogs.Quests == nil {
		return nil, nil, fmt.Errorf("quests require the native PVF quests domain")
	}
	if startup.ProgressionCatalog != "" && pvfCatalogs.Progression == nil {
		return nil, nil, fmt.Errorf("progression requires the native PVF progression domain")
	}
	if os.Getenv("DFO_NPC_PRESENCE_WORLD") != "" {
		return nil, nil, fmt.Errorf("NPC diagnostics require the active native PVF world; clear DFO_NPC_PRESENCE_WORLD")
	}
	if startup.PVFCheckCatalogs {
		pvfCatalogs.CollectImportMemory()
		if startup.PVFCheckHeapProfile != "" {
			if err := pvfCatalogs.WriteHeapProfile(startup.PVFCheckHeapProfile); err != nil {
				return nil, nil, err
			}
		}
		report, err := pvfCatalogs.CheckReport(startup.PVFCatalogs)
		if err != nil {
			return nil, nil, err
		}
		b, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return nil, nil, err
		}
		fmt.Println(string(b))
		return nil, nil, nil
	}
	if err := requireRuntimeContent(startup, pvfCatalogs); err != nil {
		return nil, nil, err
	}
	if _, err := pvfCatalogs.InstallAdventureRules(); err != nil {
		return nil, nil, fmt.Errorf("PVF adventure runtime rules: %v", err)
	}
	if _, err := pvfCatalogs.InstallRecommendedRules(); err != nil {
		return nil, nil, fmt.Errorf("PVF recommended dungeon runtime rules: %v", err)
	}
	if _, err := pvfCatalogs.InstallSeasonRules(); err != nil {
		return nil, nil, fmt.Errorf("PVF season runtime rules: %v", err)
	}
	if _, err := pvfCatalogs.InstallRosterBackgrounds(); err != nil {
		return nil, nil, fmt.Errorf("PVF roster background runtime rules: %v", err)
	}
	if _, err := pvfCatalogs.InstallFameRules(); err != nil {
		return nil, nil, fmt.Errorf("PVF fame runtime rules: %v", err)
	}
	if _, err := pvfCatalogs.InstallEquipmentAwakening(); err != nil {
		return nil, nil, fmt.Errorf("PVF equipment awakening runtime rules: %v", err)
	}
	if _, err := pvfCatalogs.InstallSoleEquipment(); err != nil {
		return nil, nil, fmt.Errorf("PVF sole equipment runtime rules: %v", err)
	}
	// 秘宝精度结算口径开关（业主 2026-10-02）：默认单机口径（单次 5..20，到上限截断）；
	// 打开后按原版（国服）结算 —— 保底 +1、25% 大成功、四阶段封顶、24/49/74/99 必到节点、
	// 25/50/75 之后必暴击。**两套并存**，见 internal/inventory/sole.go。
	inventory.SetSoleQualityNative(startup.SoleQualityNative)
	if startup.SoleQualityNative {
		log.Printf("sole quality NATIVE ON — 保底+1 / 25%% 大成功 / 四阶段封顶（-sole-quality-native / DFO_SOLE_QUALITY_NATIVE）")
	}
	if _, err := pvfCatalogs.InstallScriptWarps(); err != nil {
		return nil, nil, fmt.Errorf("PVF script warp runtime routes: %v", err)
	}
	pvfCatalogs.CollectImportMemory()
	equipmentCraftWindow = byte(startup.EquipmentCraftWindow)
	equipmentCraftVariant = byte(startup.EquipmentCraftVariant)
	equipmentCraftConfirmWindow = byte(startup.EquipmentCraftConfirmWindow)
	equipmentCraftConfirmVariant = byte(startup.EquipmentCraftConfirmVariant)
	equipmentCraftExecute = startup.EquipmentCraftExecute
	equipmentCraftGenerateWindow = byte(startup.EquipmentCraftGenerateWindow)
	equipmentCraftGenerateVariant = byte(startup.EquipmentCraftGenerateVariant)
	switch startup.EquipmentCraftExecuteOn {
	case "confirm", "first", "never":
		equipmentCraftExecuteOn = startup.EquipmentCraftExecuteOn
	default:
		return nil, nil, fmt.Errorf("invalid -equipment-craft-execute-on %q (want confirm/first/never)", startup.EquipmentCraftExecuteOn)
	}
	switch startup.EquipmentTransform {
	case "apply", "observe":
		equipmentTransformApply = startup.EquipmentTransform
	default:
		return nil, nil, fmt.Errorf("invalid -equipment-transform %q (want apply/observe)", startup.EquipmentTransform)
	}
	primerTransformWindow = byte(startup.PrimerTransformWindow)
	primerTransformVariant = byte(startup.PrimerTransformVariant)
	switch startup.PrimerTransform {
	case "apply", "observe":
		primerTransformApply = startup.PrimerTransform
	default:
		return nil, nil, fmt.Errorf("invalid -primer-transform %q (want apply/observe)", startup.PrimerTransform)
	}
	oathGradePair, oathGradesErr := parseOathGrades(startup.OathGrades)
	if oathGradesErr != nil {
		return nil, nil, fmt.Errorf("bad -oath-grades: %v", oathGradesErr)
	}
	// 档位表只服务「按穿戴装备算档位」这条诊断路径（-oath-grades-from-gear）。
	// 默认的保底路径不需要它，所以默认配置下**不加载、也不会因为缺表拒绝启动**。
	var oathGradeTable *inventory.OathGradeTable
	if startup.OathGradesFromGear {
		table, tableErr := pvfCatalogs.LoadOathGrades(startup.OathGradesTable)
		if tableErr != nil {
			return nil, nil, fmt.Errorf("bad -oath-grades-table: %v", tableErr)
		}
		oathGradeTable = table
	}
	oathProgressSet, oathProgressErr := parseOathProgressDungeons(startup.OathProgressDungeons)
	if oathProgressErr != nil {
		return nil, nil, fmt.Errorf("bad -oath-progress-dungeons: %v", oathProgressErr)
	}
	switch {
	case len(oathGradePair) == 2 && (oathGradePair[0] != 0 || oathGradePair[1] != 0):
		log.Printf("oath grades: overridden to primer=%d oath=%d (diagnostic)", oathGradePair[0], oathGradePair[1])
	case startup.OathGradesFromGear:
		log.Printf("oath grades: derived from worn oath/primer gear (%d known items, diagnostic)", oathGradeTable.Len())
	case omenState:
		log.Printf("oath grades: hidden boss driven by an omen full settlement on %s", startup.OathProgressDungeons)
	case startup.OathProgressClears > 0:
		log.Printf("oath grades: hidden-boss pity every %d clear(s) of %s", startup.OathProgressClears, startup.OathProgressDungeons)
	default:
		log.Printf("oath grades: always normal (pity disabled)")
	}
	oathInjectSpecs, oathInjectErr := parseOathInject(startup.OathInject)
	if oathInjectErr != nil {
		return nil, nil, fmt.Errorf("bad -oath-inject: %v", oathInjectErr)
	}
	if len(oathInjectSpecs) > 0 {
		log.Printf("oath injector armed: %d candidate notification(s)", len(oathInjectSpecs))
	}
	// 征兆队伍状态（noti 2836）的载荷。**在启动期校验**：以前这段在频道会话建立时
	// （每个频道一次）才解析，写错一个字符就会在玩家"进频道"的那一刻 log.Fatalf，
	// 现象是"启动游戏进不去频道"，而且加载日志已经刷完、错误行在最底下，极难定位。
	omenInfoBytes, omenInfoErr := parseOmenInfo(startup.OmenInfo)
	if omenInfoErr != nil {
		return nil, nil, fmt.Errorf("bad -omen-info: %v", omenInfoErr)
	}
	if len(omenInfoBytes) > 0 {
		log.Printf("omen info (noti 2836): injecting %d bytes: %s", len(omenInfoBytes), hex.EncodeToString(omenInfoBytes))
	}
	// 掉落调参（与官服的显式差异）。这里是**保留入口**的数值差异：关掉时表保持官方原值。
	attunementRebalance := loot.Rebalance{}
	if startup.AttunementRebalance {
		if startup.AttunementFixedTilt < 0 || startup.AttunementFixedTilt >= 100 {
			return nil, nil, fmt.Errorf("bad -attunement-fixed-tilt: %d is outside 0..99 (100 would empty the common tiers)", startup.AttunementFixedTilt)
		}
		attunementRebalance = loot.Rebalance{
			OmenHalveIdle:    true,
			FixedTiltPercent: uint32(startup.AttunementFixedTilt),
		}
	}

	skillRelease := os.Getenv("DFO_SKILL_RELEASE") == "1"
	// NOTI2827 restores locked skills from the client's own character option
	// block. The built-in block is the same version as this client, so the
	// template file and the offset override are escapes for a different build.
	var unifiedCharacTemplate []byte
	if startup.UnifiedCharacTemplate != "" {
		data, err := os.ReadFile(startup.UnifiedCharacTemplate)
		if err != nil {
			return nil, nil, err
		}
		if len(data) == 0 {
			return nil, nil, errors.New("empty character option template")
		}
		unifiedCharacTemplate = data
		log.Printf("NOTI2827 character option block overridden by %s (%d bytes)", startup.UnifiedCharacTemplate, len(data))
	}
	if startup.SkillLockOffset >= 0 {
		log.Printf("NOTI2827 skill lock offset overridden to %d", startup.SkillLockOffset)
	}
	var accountOptionsPayload []byte
	if startup.AccountOptions != "" {
		data, err := os.ReadFile(startup.AccountOptions)
		if err != nil {
			return nil, nil, err
		}
		var overrides map[uint16]uint16
		if err = json.Unmarshal(data, &overrides); err != nil {
			return nil, nil, err
		}
		accountOptionsPayload, err = protocol.AccountOptions(overrides)
		if err != nil {
			return nil, nil, err
		}
	}
	gameHost, _, gameListenError := net.SplitHostPort(startup.GameListen)
	// A wildcard bind is what makes the gateway reachable from other machines;
	// an explicit host still has to be a numeric address rather than a name.
	if gameListenError != nil || (gameHost != "" && net.ParseIP(gameHost) == nil) {
		return nil, nil, errors.New("game-listen must be host:port with a numeric IP host")
	}
	if startup.EntryBasicProbe && (startup.SelectProbeConfig == "" || startup.CharacterStorage == "") {
		return nil, nil, errors.New("entry basic probe requires persisted characters and SELECT probe configuration")
	}
	var townCatalog catalog.TownArea
	var townPolicy townEntryPolicy
	if startup.TownEntryProbe != "" {
		if !startup.EntryBasicProbe || (startup.TownCatalog == "" && pvfCatalogs.Town == nil) {
			return nil, nil, errors.New("town probe requires basic actor and town catalog")
		}
		var e error
		townCatalog, e = pvfCatalogs.LoadTown(startup.TownCatalog)
		if e != nil {
			return nil, nil, e
		}
		b, e := os.ReadFile(startup.TownEntryProbe)
		if e != nil {
			return nil, nil, e
		}
		if e = json.Unmarshal(b, &townPolicy); e != nil {
			return nil, nil, e
		}
		if !townCatalog.Allows(255, townPolicy.X, townPolicy.Y) {
			return nil, nil, errors.New("spawn policy lies outside source walkable rectangles")
		}
	}
	var selectProbe *protocol.SelectProbeState
	if startup.SelectProbeConfig != "" {
		b, e := os.ReadFile(startup.SelectProbeConfig)
		if e != nil {
			return nil, nil, e
		}
		selectProbe = new(protocol.SelectProbeState)
		if e = json.Unmarshal(b, selectProbe); e != nil {
			return nil, nil, e
		}
		if _, e = protocol.SelectProbeSuccess(*selectProbe); e != nil {
			return nil, nil, e
		}
	}
	var characters *character.Service
	var gameStore *database.Store
	var worldService *world.Service
	var wearService *workflow.WearService
	var questService *quest.Service
	var townArrivalScenes map[uint32]catalog.TownArrivalScene
	var vaultService *workflow.VaultService
	var fatigueService *character.FatigueService
	var developmentAccount int64
	var dungeonCatalog *catalog.DungeonCatalog
	var progressionService *character.ProgressionService
	var lootService *loot.Service
	// rewardNotifier is the optional Lua reward add-on; nil disables it.
	var rewardNotifier reward.Notifier
	var itemService *inventory.ItemService
	var shopService *workflow.ShopService
	// journalRules 是装备库规则（nil = 不登记）。它同时被 CMD26 的事务与入场 2610 用到，
	// 所以在这里声明、在 loot 块里装载。
	var journalRules *catalog.EquipmentJournalRules
	// equipmentCreateCost 是「装备生成」成本表（nil = 第二步只回窗口、不生成）。
	var equipmentCreateCost *catalog.EquipmentCreateCost
	// equipmentTransformSystem 是三条变换链（装备 2259 / 融合 / 晶体 2381）的费用与返还表。
	// nil = 变换算不出成本 ⇒ 拒绝执行，不静默改成"免费"。
	var equipmentTransformSystem *catalog.EquipmentTransformSystem
	// pointRules 是逐件「套装/誓约积分」表（setpointinfo.cos / oathpointinfo.cos）。
	// nil = 算不出积分 ⇒ 不推 NOTI2634（客户端保持原值），不发 0 冒充。
	var pointRules *catalog.PointRules
	var shopPilot *cashshop.Pilot
	var unsealService *workflow.UnsealService
	// skinCatalog maps an `[add skin storage]` stackable template to its PVF
	// facts, driving CMD507 action 169 (damage font) registration. Nil when no
	// item index is configured, which disables the skin flow.
	var skinCatalog map[uint32]catalog.SkinStorageEntry
	if startup.CharacterStorage != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cfg, e := database.LoadConfig(startup.CharacterStorage)
		if e != nil {
			return nil, nil, e
		}
		s, e := database.Open(ctx, cfg)
		if e != nil {
			return nil, nil, e
		}
		resources.add(func() { s.Close() })
		gameStore = s
		// 启动日志必须写明**实际打开的引擎与库**：pgsql 端「登录后账号不见了」这类
		// 报告，根因常常是启动器与服务端对同一份 local.json 选了不同的库
		// （见 internal/database.EngineForConfig）。这里读的是活动连接自己的结论，
		// PostgreSQL 报库名、SQLite 报文件路径，不再是推断。
		if driver, driverErr := database.EngineForConfig(cfg); driverErr == nil {
			target := "(unknown)"
			if name, nameErr := s.DatabaseName(ctx); nameErr == nil {
				target = name
			}
			log.Printf("storage: engine=%s target=%s config=%s", driver, target, startup.CharacterStorage)
		}

		// 兜底自愈：会话位置隔离修复之前，特殊征讨频道（月湖 215 / Azure 213 / 军团
		// 239 等）的会话位置曾被写进普通频道共享行，玩家切回普通频道会被客户端以
		// 「对立阵营起始点」拒绝。启动期清一次污染行（幂等），玩家下一次进普通频道
		// 走默认落点重建，无资产损失。
		if pvfCatalogs != nil && len(pvfCatalogs.ChannelTowns) > 0 {
			towns := make([]uint32, 0, len(pvfCatalogs.ChannelTowns))
			for _, a := range pvfCatalogs.ChannelTowns {
				towns = append(towns, a.TownID)
			}
			scrubCtx, scrubCancel := context.WithTimeout(context.Background(), 15*time.Second)
			if n, scrubErr := gameStore.ScrubPollutedWorldPositions(scrubCtx, towns); scrubErr != nil {
				log.Printf("warning: scrub polluted world positions: %v", scrubErr)
			} else if n > 0 {
				log.Printf("scrubbed %d polluted world position(s) from the shared row (special-channel towns: %v)", n, towns)
			}
			scrubCancel()
		}
		releaseAdminGuard, e := s.HoldAdminGuard(ctx)
		if e != nil {
			return nil, nil, e
		}
		resources.add(func() { releaseAdminGuard() })
		if e = s.InitializeGame(ctx); e != nil {
			return nil, nil, e
		}
		data, e := pvfCatalogs.LoadCharacters(startup.CharacterCatalog)
		if e != nil {
			return nil, nil, e
		}
		b, e := os.ReadFile(startup.CharacterRules)
		if e != nil {
			return nil, nil, e
		}
		var rules character.Rules
		if e = json.Unmarshal(b, &rules); e != nil {
			return nil, nil, e
		}
		characters, e = character.New(s, data, rules)
		if characters != nil {
			characters.DisableActorAppearance = skillRelease
			characters.DetailedWornCandidate = !skillRelease
		}
		if e != nil {
			return nil, nil, e
		}
		// 带期限物品一律按「永不过期」下发。**默认开启**（DFO_MAX_ITEM_PERIOD=0 才关）：
		// 三个 .cmd 入口都设了这个变量，但外部一键启动器自己拉起旧脚本编排、
		// 从不设置它 ⇒ 走一键启动器时整条兜底不生效，脚本声明过期限的模板
		// （银增幅书到期日 2022-11-08 之类）就会带着 0 下发，客户端显示
		// 「剩余期限已过」并拒绝使用（错误码 31730）。
		if os.Getenv("DFO_MAX_ITEM_PERIOD") != "0" {
			protocol.ConfigureStoredPeriodLifting(true)
			if startup.ItemIndex == "" && pvfCatalogs.Periods == nil && !pvfCatalogs.Prepared("periods") {
				log.Printf("maximum item period: no item index (-item-index), lifting stored periods only")
			} else {
				templates, periodErr := pvfCatalogs.LoadItemPeriods("", data.Source.Checksum)
				if periodErr != nil {
					if pvfCatalogs.Selected("periods") || pvfCatalogs.Periods != nil {
						return nil, nil, periodErr
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
		if startup.ItemIndex != "" || pvfCatalogs.Skins != nil || pvfCatalogs.Prepared("skins") {
			entries, skinErr := pvfCatalogs.LoadSkinStorage("", data.Source.Checksum)
			if skinErr != nil {
				if pvfCatalogs.Selected("skins") {
					return nil, nil, skinErr
				}
				log.Printf("skin storage registration disabled: %v", skinErr)
			} else if e = s.MigrateSkinCargo(ctx); e != nil {
				return nil, nil, e
			} else if e = s.MigrateSkinSelection(ctx); e != nil {
				return nil, nil, e
			} else if e = s.MigrateSkinSelectionList(ctx); e != nil {
				return nil, nil, e
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
		if pvfCatalogs.CashShop != nil {
			var database string
			if database, e = s.DatabaseName(ctx); e == nil {
				if database != "dfo_swordmaster_pilot_20260916" && !startup.ShopRelease {
					log.Printf("shop purchase pilot running on database: %s", database)
				}
			}
			shopPilot, e = pvfCatalogs.LoadCashShop(data.Source.Checksum, startup.ShopRelease)
			if e != nil {
				return nil, nil, e
			}
			if e = s.MigrateCashShop(ctx); e != nil {
				return nil, nil, e
			}
			log.Printf("PVF shop enabled: %d ordinary products", shopPilot.EnabledCount())
			log.Printf("商城发布模式：%t", shopPilot.Config.Release)
		}
		if startup.SkillCatalog != "" || pvfCatalogs.Learning != nil {
			characters.Learning, e = pvfCatalogs.LoadLearning(startup.SkillCatalog, data.Source.Checksum)
			if e != nil {
				return nil, nil, e
			}
			if e = s.MigrateCharacterEvents(ctx); e != nil {
				return nil, nil, e
			}
			if e = s.MigrateCharacterNotices(ctx); e != nil {
				return nil, nil, e
			}
			if e = s.MigrateSkillLocks(ctx); e != nil {
				return nil, nil, e
			}
		}
		developmentAccount, e = s.DevelopmentAccount(ctx, "probe")
		if e != nil {
			return nil, nil, e
		}
	}
	if startup.EntryAdditionProbe && !startup.EntryBasicProbe {
		return nil, nil, errors.New("addition requires a basic actor")
	}
	if startup.FatigueRules != "" {
		if characters == nil || selectProbe == nil {
			return nil, nil, errors.New("fatigue requires persisted characters and SELECT")
		}
		var e error
		fatiguePath := startup.FatigueRules
		if path := os.Getenv("DFO_FATIGUE_RULES"); path != "" {
			fatiguePath = path
		}
		fatigueService, e = character.LoadFatigueService(gameStore, fatiguePath)
		if e != nil {
			return nil, nil, e
		}
		// 疲劳消耗总开关：进本与房间两处一起归零（业主 2026-10-01 按玩家反馈要求）。
		fatigueService.Free = startup.FatigueFree
		if startup.FatigueFree {
			log.Printf("fatigue consumption OFF — 进本消耗与房间消耗都按 0 记（-fatigue-free / DFO_FATIGUE_FREE）")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = gameStore.MigrateFatigue(ctx)
		cancel()
		if e != nil {
			return nil, nil, e
		}
	}
	if startup.WorldCatalog != "" || pvfCatalogs.World != nil {
		if characters == nil || startup.TownEntryProbe == "" {
			return nil, nil, errors.New("world requires persisted characters and a spawn policy")
		}
		data, e := pvfCatalogs.LoadWorld(startup.WorldCatalog)
		if e != nil {
			return nil, nil, e
		}
		b, e := os.ReadFile(startup.WorldRules)
		if e != nil {
			return nil, nil, e
		}
		var rules world.Rules
		if e = json.Unmarshal(b, &rules); e != nil {
			return nil, nil, e
		}
		if data.Source.Checksum != characters.Catalog.Source.Checksum {
			return nil, nil, errors.New("world/character source versions differ")
		}
		worldService = &world.Service{Store: gameStore, Catalog: data, Rules: rules}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = gameStore.MigrateWorld(ctx)
		cancel()
		if e != nil {
			return nil, nil, e
		}
	}
	if pvfCatalogs.Dungeons != nil {
		if worldService == nil {
			return nil, nil, errors.New("dungeons require world sessions")
		}
		data := *pvfCatalogs.Dungeons
		trainingRoomPath := os.Getenv("DFO_TRAINING_ROOM_CATALOG")
		if trainingRoomPath == "" {
			trainingRoomPath = filepath.Join(filepath.Dir(startup.CharacterCatalog), "dungeons.training-room.json")
		}
		trainingRooms, e := pvfCatalogs.LoadTrainingDungeons(trainingRoomPath)
		if e != nil {
			return nil, nil, e
		}
		if e = catalog.MergeDungeonCatalog(&data, trainingRooms); e != nil {
			return nil, nil, e
		}
		overlayDirectory := filepath.Dir(startup.CharacterCatalog)
		if e = pvfCatalogs.AttachTerminalScenes(&data, ""); e != nil {
			return nil, nil, e
		}
		path := filepath.Join(overlayDirectory, "dungeons.layer-revisits.json")
		if e = pvfCatalogs.AttachLayerRevisits(&data, path); e != nil {
			return nil, nil, e
		}
		path = filepath.Join(overlayDirectory, "dungeons.tournament-quest-maps.json")
		if e = pvfCatalogs.AttachTournamentMaps(&data, path); e != nil {
			return nil, nil, e
		}
		path = filepath.Join(overlayDirectory, "dungeons.tower-of-grief-maps.json")
		if e = pvfCatalogs.AttachTowerGrief(&data, path); e != nil {
			return nil, nil, e
		}
		path = filepath.Join(overlayDirectory, "dungeons.tower-of-dazzlement-maps.json")
		if e = pvfCatalogs.AttachTowerDazzlement(&data, path); e != nil {
			return nil, nil, e
		}
		path = filepath.Join(overlayDirectory, "dungeons.maze-chance-rates.json")
		if e = pvfCatalogs.AttachMazeRates(&data, path); e != nil {
			return nil, nil, e
		}
		path = filepath.Join(overlayDirectory, "dungeons.hell-party-maps.json")
		if e = pvfCatalogs.AttachHellMaps(&data, path); e != nil {
			return nil, nil, e
		}
		if data.Source.Checksum != worldService.Catalog.Source.Checksum {
			return nil, nil, errors.New("dungeon/world source versions differ")
		}
		// 「哪些副本按权重掷骰选图」念出来（权重是我们改写过的，见 §41）。
		// 强制选图放在念完之后：日志先反映配置，再反映这次的诊断覆盖。
		logMazeChance(&data)
		if e = forceMaze(&data, os.Getenv("DFO_MAZE_FORCE")); e != nil {
			return nil, nil, e
		}
		dungeonCatalog = &data
	}
	// 诊断探针：DFO_DUNGEON_PROBE=100004131,100004136 打印副本的迷宫结构
	// （房间坐标/map/怪生成触发器），用于 SemiRaid 门控取证。只读，不影响运行。
	if probeSpec := os.Getenv("DFO_DUNGEON_PROBE"); probeSpec != "" && dungeonCatalog != nil {
		for _, idStr := range strings.Split(probeSpec, ",") {
			id64, err := strconv.ParseUint(strings.TrimSpace(idStr), 10, 32)
			if err != nil {
				continue
			}
			def, ok := dungeonCatalog.Dungeons[uint32(id64)]
			if !ok {
				log.Printf("dungeon-probe %d: not in catalog", id64)
				continue
			}
			log.Printf("dungeon-probe %d noFatigue=%v chances=%v", id64, def.NoFatigue, def.MazeChanceRates)
			for _, mz := range def.Mazes {
				log.Printf("dungeon-probe %d maze%d size=%v start=%v boss=%v rooms=%d",
					id64, mz.Index, mz.Size, mz.Start, mz.Boss, len(mz.Rooms))
				for _, r := range mz.Rooms {
					log.Printf("  room(%d,%d) map=%d boss=%v", r.X, r.Y, r.Map, r.Boss)
				}
			}
		}
	}
	// 疲劳的**进本消耗**完全来自源：`[use fatigue only start dungeon] <N>`（only start = 进本只收一次）。
	// 源未声明该段的副本由 EnterFatigueOf 返回 0，走 FatigueService 原有的「按房间计费」路径。
	//
	// 2026-10-01：本地策略里的 `dungeon_enter_fatigue` 兜底表已删除——源解析已覆盖全部声明副本
	// （启动日志 `PVF dungeons declaring [use fatigue only start dungeon]: …`），
	// JSON 兜底违反「单一内容真源铁律」（server/AGENTS.md §0）：PVF 里读得到，就不许写 JSON。
	if fatigueService != nil && dungeonCatalog != nil {
		dc := dungeonCatalog
		fatigueService.EnterFatigueOf = func(dungeonID uint32) uint16 {
			if d, ok := dc.Dungeons[dungeonID]; ok && d.EnterFatigue > 0 {
				return d.EnterFatigue
			}
			return 0
		}
	}
	if startup.ProgressionCatalog != "" || pvfCatalogs.Progression != nil {
		if characters == nil || dungeonCatalog == nil {
			return nil, nil, errors.New("progression requires source characters and dungeon sessions")
		}
		data, e := pvfCatalogs.LoadProgression(startup.ProgressionCatalog)
		if e != nil {
			return nil, nil, e
		}
		rules, e := character.LoadGrowthRules(startup.ProgressionRules)
		if e != nil {
			return nil, nil, e
		}
		if data.Source.Checksum != characters.Catalog.Source.Checksum || data.Source.Checksum != dungeonCatalog.Source.Checksum {
			return nil, nil, errors.New("progression source version mismatch")
		}
		progressionService = &character.ProgressionService{Store: gameStore, Catalog: data, Professions: characters.Catalog, Rules: rules}
		progressionService.CompletionRewards = pvfCatalogs.OdysseyCompletionRewards
		progressionService.MaxLevelReward = pvfCatalogs.MaxLevelReward
		if path := os.Getenv("DFO_ODYSSEY_GROWTH"); path != "" || pvfCatalogs.OdysseyGrowth != nil {
			progressionService.Odyssey, e = pvfCatalogs.LoadOdysseyGrowth(path)
			if e != nil {
				return nil, nil, e
			}
		}
		if path := os.Getenv("DFO_ODYSSEY_CHAPTERS"); path != "" || pvfCatalogs.OdysseyChapters != nil {
			progressionService.Chapters, e = pvfCatalogs.LoadOdysseyChapters(path)
			if e != nil {
				return nil, nil, e
			}
		}
		if e = pvfCatalogs.BindOdysseyRoutes(progressionService); e != nil {
			return nil, nil, e
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = gameStore.MigrateCharacterEvents(ctx)
		if e == nil {
			e = gameStore.MigrateCharacterNotices(ctx)
		}
		if e == nil {
			e = gameStore.MigrateSkillLocks(ctx)
		}
		if e == nil {
			e = gameStore.MigrateTowerProgress(ctx)
		}
		cancel()
		if e != nil {
			return nil, nil, e
		}
	}
	var tutorialRoutes *catalog.TutorialCatalog
	var tutorialDungeons *catalog.DungeonCatalog
	if startup.TutorialRoutes != "" {
		if dungeonCatalog == nil || characters == nil {
			return nil, nil, errors.New("starting routes require source dungeons and persisted characters")
		}
		if startup.TutorialDungeons == "" && pvfCatalogs.TutorialDungeons == nil {
			return nil, nil, errors.New("starting routes require their own dungeon catalog")
		}
		routes, e := pvfCatalogs.LoadTutorialRoutes(startup.TutorialRoutes, characters.Catalog.Source.Checksum)
		if e != nil {
			return nil, nil, e
		}
		data, e := pvfCatalogs.LoadTutorialDungeons(startup.TutorialDungeons)
		if e != nil {
			return nil, nil, e
		}
		if data.Source.Checksum != characters.Catalog.Source.Checksum {
			return nil, nil, errors.New("starting-route dungeon source version differs")
		}
		tutorialRoutes, tutorialDungeons = routes, &data
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		e = gameStore.MigrateBirth(ctx)
		cancel()
		if e != nil {
			return nil, nil, e
		}
	}
	if startup.LootCatalog != "" {
		if progressionService == nil {
			return nil, nil, errors.New("loot requires progression and owned dungeon sessions")
		}
		lootPath := startup.LootCatalog
		if path := os.Getenv("DFO_LOOT_CATALOG"); path != "" {
			lootPath = path
		}
		if err := pvfCatalogs.LoadEnhancements(filepath.Dir(lootPath)); err != nil {
			return nil, nil, err
		}
		// 锻造（CMD430 / Refine）的武器限制、成功率表与材料消耗。
		// 成功率由服主提供（115 版本），材料消耗 PVF 无表、走配置默认值。
		if err := inventory.LoadRefineRules(filepath.Join(filepath.Dir(startup.BagRules), "refine.json")); err != nil {
			return nil, nil, err
		}
		// 物品脚本自带的 [need material]（商店表 itemshop/**.shp 没有价格字段）：
		// 商店里「用材料交换」的商品，材料成本只写在物品脚本里（3242=1000×3037 等）。
		itemMaterials, matErr := pvfCatalogs.LoadItemMaterials("")
		if matErr != nil {
			return nil, nil, matErr
		}
		c, e := pvfCatalogs.LoadLoot(lootPath)
		if e != nil {
			return nil, nil, e
		}
		r, e := loot.LoadRules(startup.LootRules)
		if e != nil {
			return nil, nil, e
		}
		bag, e := inventory.LoadBagRules(startup.BagRules, pvfCatalogs.SourceChecksum)
		if e != nil {
			return nil, nil, e
		}
		tables, e := loot.Parse(c)
		if e != nil {
			return nil, nil, e
		}
		if c.Source.Checksum != characters.Catalog.Source.Checksum || bag.Source != c.Source.Checksum {
			return nil, nil, errors.New("loot source mismatch")
		}
		if startup.EquipmentCatalog == "" {
			return nil, nil, errors.New("loot requires -equipment-catalog or DFO_EQUIPMENT_CATALOG: without it every equipment award is silently dropped")
		}
		gear, e := pvfCatalogs.LoadEquipmentSelection(startup.EquipmentCatalog, c.Source.Checksum)
		if e != nil {
			return nil, nil, e
		}
		log.Printf("loaded equipment catalog: %d rows, %d droppable, from %s",
			len(gear.Rows), len(gear.DropPool()), startup.EquipmentCatalog)
		dropCatalog := c
		if pvfCatalogs.Items != nil {
			if err := pvfCatalogs.SupplementStackables(&c, ""); err != nil {
				return nil, nil, err
			}
			log.Printf("supplemented stackable catalog from native PVF (total items: %d)", len(c.Items))
		}
		if startup.EquipmentJournalRules != "" || pvfCatalogs.Journal != nil {
			jr, e := pvfCatalogs.LoadEquipmentJournal(startup.EquipmentJournalRules, c.Source.Checksum)
			if e != nil {
				return nil, nil, e
			}
			journalRules = &jr
			log.Printf("loaded equipment journal rules: max=%d limits=%d categories=%d groups=%d/%d",
				jr.Maximum, len(jr.MaximumByType), len(jr.Categories), len(jr.WeaponGroups), len(jr.PeculiarGroups))
		}
		if startup.EquipmentCreateCost != "" || pvfCatalogs.CreateCost != nil {
			cc, e := pvfCatalogs.LoadEquipmentCreateCost(startup.EquipmentCreateCost, c.Source.Checksum)
			if e != nil {
				return nil, nil, e
			}
			equipmentCreateCost = &cc
			items := 0
			for _, g := range cc.Groups {
				items += len(g.Items)
			}
			log.Printf("loaded equipment create cost: groups=%d itemRows=%d templates=%d",
				len(cc.Groups), items, len(cc.Templates()))
		}
		if pvfCatalogs.Transform != nil {
			ts, e := pvfCatalogs.LoadEquipmentTransformSystem()
			if e != nil {
				return nil, nil, e
			}
			equipmentTransformSystem = ts
			log.Printf("loaded equipment transform system: equipment=%d primer=%d rarities, refundRows %d/%d",
				len(ts.EquipmentNeed), len(ts.PrimerNeed),
				len(ts.EquipmentRefund), len(ts.PrimerRefund))
		}
		// 逐件积分表（setpointinfo.cos / oathpointinfo.cos）：服务端算角色 Set/Oath Point、
		// 推 NOTI2634 用。装载失败必须显式报错 —— 静默降级会让"誓约积分恒 0"再复现一次。
		if pvfCatalogs.Points != nil {
			pr, e := pvfCatalogs.LoadPointRules()
			if e != nil {
				return nil, nil, e
			}
			pointRules = pr
			log.Printf("loaded point rules: set=%d grades=%d oath=%d minOath=%d",
				len(pr.Set.Rules), len(pr.Set.Grades), len(pr.Oath.Rules), pr.Oath.MinOathPoint)
		}
		// Creation supplies are grant/move content, never ordinary drop entries.
		if progressionService != nil && progressionService.Odyssey != nil && progressionService.Odyssey.Creation != nil {
			creation := progressionService.Odyssey.Creation
			items := make(map[uint32]catalog.LootItem, len(c.Items)+len(creation.Supplies))
			for id, item := range c.Items {
				items[id] = item
			}
			for _, row := range creation.Supplies {
				items[row.Template] = creation.Items.Items[row.Template]
			}
			c.Items = items
		}
		lootService = &loot.Service{Catalog: c, DropCatalog: dropCatalog, Rules: r, BagRules: bag, Tables: tables, Equipment: gear}
		itemService = &inventory.ItemService{Model: r.Model, Catalog: c, BagRules: bag, Equipment: gear, AvatarDisjoint: pvfCatalogs.AvatarDisjoint, EmblemCompound: pvfCatalogs.EmblemCompound, AvatarSockets: pvfCatalogs.AvatarSockets, EmblemInlay: pvfCatalogs.EmblemInlay, Journal: journalRules, CreateCost: equipmentCreateCost, Transform: equipmentTransformSystem, Points: pointRules}
		if progressionService != nil {
			progressionService.CompletionAwarder = &inventory.Awarder{Catalog: c, Rules: bag, Equipment: gear}
		}
		// Event-triggered Lua rewards reuse the same catalog as the completion
		// awarder. The rule scripts are embedded in the binary.
		if rewards := buildRewardService(gameStore, &inventory.Awarder{Catalog: c, Rules: bag, Equipment: gear}); rewards != nil {
			if progressionService != nil {
				progressionService.Rewards = rewards
			}
			if characters != nil {
				characters.Rewards = rewards
			}
			rewardNotifier = rewards
		}
		shopService = &workflow.ShopService{Store: gameStore, ShopService: inventory.ShopService{Catalog: c, EventModel: r.Model, BagRules: bag, ItemMaterials: itemMaterials}}
		if pvfCatalogs.Mine != nil || startup.BleedingMineRewards != "" {
			mine, err := pvfCatalogs.LoadMine(startup.BleedingMineRewards)
			if err != nil {
				return nil, nil, err
			}
			if mine.Source != c.Source.Checksum {
				return nil, nil, errors.New("赤红铁矿奖励表与当前角色配置版本不一致")
			}
			lootService.BleedingMine = mine
		}
		if pvfCatalogs.Prices != nil || startup.ShopPrices != "" {
			shopService.Prices, e = pvfCatalogs.LoadShopPrices(startup.ShopPrices, c.Source.Checksum)
			if e != nil {
				return nil, nil, e
			}
			log.Printf("loaded %d NPC prices from native PVF", len(shopService.Prices.Items))
		} else {
			log.Printf("warning: PVF prices domain is not enabled; gold purchases and sales are refused")
		}
		if path := os.Getenv("DFO_ODYSSEY_COIN_RULES"); path != "" || pvfCatalogs.OdysseyCurrency != nil {
			lootService.Currency, e = pvfCatalogs.LoadOdysseyCurrency(path)
			if e != nil {
				return nil, nil, e
			}
		}
		cards, e := loot.LoadCardRules(startup.CardRules)
		if e != nil {
			return nil, nil, e
		}
		lootService.CardPolicy = &cards
		// 维纳斯终局翻牌第一排随机装备位池子（115 级魔法/神器常规部位）。
		flipGearPool, e := legion.LoadVenusFlipGearPool(startup.VenusFlipGear)
		if e != nil {
			return nil, nil, e
		}
		venusFlipGearPool = flipGearPool.Templates
		log.Printf("loaded venus flip gear pool: %d templates from %s", len(flipGearPool.Templates), startup.VenusFlipGear)
		if pvfCatalogs.Boxes != nil || startup.Boxes != "" {
			boxes, boxErr := pvfCatalogs.LoadBoxes(startup.Boxes, lootService.Catalog.Source.Checksum)
			if boxErr != nil {
				return nil, nil, boxErr
			}
			itemService.Boxes = boxes
			log.Printf("PVF boxes: %d tables, %d prize templates", boxes.TableCount(), boxes.RewardCount())
		}
	}
	responses := map[uint16][]byte{}
	if startup.QuestCatalog != "" || pvfCatalogs.Quests != nil {
		if worldService == nil {
			return nil, nil, errors.New("quests require world character sessions")
		}
		data, e := pvfCatalogs.LoadQuests(startup.QuestCatalog)
		if e != nil {
			return nil, nil, e
		}
		if data.Source.Checksum != characters.Catalog.Source.Checksum {
			return nil, nil, errors.New("quest/character source versions differ")
		}
		var odysseyGrowth *catalog.OdysseyGrowth
		if progressionService != nil {
			odysseyGrowth = progressionService.Odyssey
		}
		questService = &quest.Service{Store: gameStore, Catalog: data, Professions: characters.Catalog, Progression: progressionService, Odyssey: odysseyGrowth, Dungeons: dungeonCatalog}
		var sceneIssues []string
		townArrivalScenes, sceneIssues = catalog.TownArrivalSceneWhitelist(data, worldService.Catalog)
		for _, issue := range sceneIssues {
			log.Printf("town arrival scene excluded: %s", issue)
		}
		log.Printf("PVF town arrival scene whitelist: %d entries", len(townArrivalScenes))
		if startup.QuestEquipmentCatalog != "" {
			if lootService == nil {
				return nil, nil, errors.New("quest inventory requires the shared bag catalog")
			}
			equipment, e := pvfCatalogs.LoadEquipmentSelection(startup.QuestEquipmentCatalog, data.Source.Checksum)
			if e != nil {
				return nil, nil, e
			}
			questService.Inventory = &inventory.Awarder{Catalog: lootService.Catalog, Rules: lootService.BagRules, Equipment: equipment}
			if startup.EquipmentWearRules != "" {
				rulesPath := startup.EquipmentWearRules
				if override := os.Getenv("DFO_EQUIPMENT_WEAR_RULES"); override != "" {
					rulesPath = override
				}
				rules, err := inventory.LoadWearRules(rulesPath, data.Source.Checksum)
				if err != nil {
					return nil, nil, err
				}
				wearService = &workflow.WearService{Store: gameStore, WearService: inventory.WearService{PremiumStore: workflow.PremiumReader{Store: gameStore}, Catalog: equipment, Professions: characters.Catalog, BagRules: lootService.BagRules, Rules: rules, AvatarRecast: pvfCatalogs.AvatarRecast, AvatarRecastLoot: &lootService.Catalog}}

				// 装备变换要用「部位 → 装备类型」映射去**背包**里找源（客户端允许把背包装备放进
				// 界面「变换前」槽，请求只带部位码），所以把同一份 WearRules 也交给 inventory 物品服务。
				itemService.WearRules = rules

				if startup.KnightShieldCatalog != "" {
					shieldPath := knightShieldCatalogPath(startup.KnightShieldCatalog, rulesPath)
					shields, shieldErr := pvfCatalogs.LoadShields(shieldPath, data.Source.Checksum)
					if shieldErr != nil && !errors.Is(shieldErr, os.ErrNotExist) {
						return nil, nil, shieldErr
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
				if startup.EquipmentFullCatalog != "" || pvfCatalogs.Equipment != nil {
					full, err := pvfCatalogs.OpenFullEquipment(startup.EquipmentFullCatalog, data.Source.Checksum)
					if err != nil {
						return nil, nil, err
					}
					if full != pvfCatalogs.Equipment {
						resources.add(func() { full.Close() })
					}
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
					log.Printf("separate wear catalog: %d records; original reward/drop catalog: %d", full.RecordCount(), len(equipment.Rows))
				}
			}
			// The same source equipment catalog backs quest rewards and
			// monster gear drops; a drop only offers what a bag accepts.
			lootService.Equipment = equipment
			// Magic-seal unsealing (CMD393) rolls from the current random
			// option tables and reads each item's [random option] flag from
			// the full equipment catalog; without the full definitions the
			// sealed state cannot be proven, so the command stays unanswered.
			if equipment.Full != nil && (startup.RandomOptionCatalog != "" || pvfCatalogs.RandomOptions != nil) {
				options, err := pvfCatalogs.LoadRandomOptions(startup.RandomOptionCatalog, data.Source.Checksum)
				if err != nil {
					return nil, nil, err
				}
				unsealService = &workflow.UnsealService{Store: gameStore, Equipment: equipment, RandomOptions: options, Model: "current115-randomoption-v1"}
				log.Printf("magic-seal unsealing enabled: %d option groups", options.GroupCount())
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = gameStore.MigrateQuests(ctx)
		if e == nil {
			e = gameStore.MigrateQuestObjectives(ctx)
		}
		if e == nil && progressionService != nil {
			e = gameStore.MigrateQuestRewards(ctx)
		}
		cancel()
		if e != nil {
			return nil, nil, e
		}
	}
	// 存档身份归一（2026-10-01，next146 结构性根治）。过去这行身份被钉在**内层归档哈希**上
	// （每次重建都变），换一次客户端就全体进不去角色（quest %d requires source migration）。
	// 现在身份由**服务端契约**定义（internal/savecontract），本迁移以 IS DISTINCT FROM
	// 归一任意历史值（含旧批次标记、空串、NULL），不依赖历史哈希白名单。
	// 放在这里是因为前面的 Migrate* 才建出 character_quests/character_map_clears 等表。
	if characters != nil {
		identity := characters.Catalog.Source.SaveIdentity()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		n, rebaseErr := gameStore.MigrateSaveIdentity(ctx, identity)
		cancel()
		if rebaseErr != nil {
			return nil, nil, fmt.Errorf("save identity normalization: %v", rebaseErr)
		}
		// 无论 n 是否为 0 都要打：首版失败正是「命中 0 行时完全无声」，排查只能靠翻库。
		log.Printf("save identity normalized: %d stored row(s) -> contract %s (inner archive %s)",
			n, identity, characters.Catalog.Source.Checksum)
	}
	if startup.VaultRules != "" {
		if characters == nil {
			return nil, nil, errors.New("vault initialization requires characters")
		}
		rules, e := pvfCatalogs.LoadVaultRules(startup.VaultRules)
		if e != nil {
			return nil, nil, e
		}
		vaultService = &workflow.VaultService{Store: gameStore, VaultService: inventory.VaultService{Rules: rules}}
		if wearService != nil {
			vaultService.Equipment = wearService.Catalog
		}
		if startup.VaultPurchaseCandidate || startup.VaultPurchaseRelease || startup.ShopRelease {
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
					return nil, nil, e
				}
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = gameStore.MigrateVault(ctx)
		if e == nil {
			e = gameStore.UpgradeSecondaryVaultCapacity(ctx)
		}
		if e == nil {
			e = gameStore.MigrateAccountMaterials(ctx)
		}
		if e == nil && rules.Account != nil {
			e = gameStore.MigrateAccountVault(ctx)
		}
		cancel()
		if e != nil {
			return nil, nil, e
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
		choices, e := pvfCatalogs.LoadOdysseyWeapons(os.Getenv("DFO_ODYSSEY_WEAPON_BOX"))
		odysseyChoices = odysseyWeaponChoices(choices)
		if e != nil {
			return nil, nil, e
		}
		if vaultService != nil {
			items := make(map[uint32]catalog.LootItem, len(vaultService.Catalog.Items)+1)
			for id, item := range vaultService.Catalog.Items {
				items[id] = item
			}
			items[odysseyChoices.Template] = odysseyChoices.Item
			vaultService.Catalog.Items = items
		}
	}
	if path := os.Getenv("DFO_CLEAR_CUBE_SOURCE"); (path != "" || pvfCatalogs.ClearCube != nil) && vaultService != nil {
		var e error
		vaultService.Catalog, e = pvfCatalogs.WithClearCube(vaultService.Catalog, path)
		if e != nil {
			return nil, nil, e
		}
	}
	var boosterCatalog *BoosterCatalog
	if startup.BoosterCatalog != "" || pvfCatalogs.Boosters != nil || pvfCatalogs.Prepared("boosters") {
		var err error
		boosterCatalog, err = pvfCatalogs.LoadBooster(startup.BoosterCatalog, startup.ItemIndex)
		if err != nil {
			return nil, nil, err
		}
		log.Printf("loaded native booster catalog (%d definitions, %d item index entries)", len(boosterCatalog.Definitions), len(boosterCatalog.Items))
	} else if pvfCatalogs.Items != nil {
		boosterCatalog = &catalog.BoosterCatalog{Items: pvfCatalogs.Items.Items}

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
	if boosterCatalog != nil && (startup.ItemIndex != "" || pvfCatalogs.LotteryTables != nil) {
		var err error
		lotteryPools, err = loadRuntimeLotteryItems(pvfCatalogs, "", boosterCatalog.Items)
		if err != nil {
			return nil, nil, fmt.Errorf("prepare lottery item catalog: %w", err)
		} else {
			log.Printf("loaded lottery item catalog (%d verified pools)", len(lotteryPools.Pools))
			if wearService != nil && wearService.Catalog != nil {
				if count, loadErr := loadRuntimeLotteryEquipment(pvfCatalogs, "", boosterCatalog.Items, lotteryPools); loadErr != nil {
					return nil, nil, fmt.Errorf("prepare equipment lottery pools: %w", loadErr)
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
	if startup.SelectionBoxes != "" || pvfCatalogs.SelectionBoxes != nil {
		var err error
		selectionBoxes, err = pvfCatalogs.LoadSelectionBoxes(startup.SelectionBoxes)
		if err != nil {
			return nil, nil, err
		} else {
			log.Printf("loaded native selection boxes (%d boxes, %d mislabeled fixed)", len(selectionBoxes.Boxes), len(selectionBoxes.Fixed))
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
	if startup.ApocalypseCatalog != "" || pvfCatalogs.Apocalypse != nil {
		loaded, err := pvfCatalogs.LoadApocalypse(startup.ApocalypseCatalog)
		if err != nil {
			log.Printf("warning: load apocalypse catalog (%s): %v", startup.ApocalypseCatalog, err)
		} else if clock, err := legion.NewApocalypseClock(loaded); err != nil {
			log.Printf("warning: apocalypse clock (%s): %v", startup.ApocalypseCatalog, err)
		} else {
			apocalypseCatalog, apocalypseClock = loaded, clock
			log.Printf("loaded apocalypse table (%d records, %d operations, %d phases, %gs total) from %s",
				loaded.RecordCount, len(loaded.Operations), clock.Len(), clock.TotalSeconds(), startup.ApocalypseCatalog)
		}
	}
	if apocalypseCatalog == nil {
		log.Printf("warning: no apocalypse catalog; legion operation confirmations are not validated")
	}
	// 物品商店表：源用 [need material] 定价的商品（奥德赛商店的盒子要 100 个银币）
	// 必须按材料扣，否则一律按写死的金币单价白送。
	var itemShops *catalog.ItemShops
	if startup.ItemShop != "" || pvfCatalogs.ItemShops != nil {
		var err error
		shopSource := ""
		if pvfCatalogs.ItemShops != nil {
			shopSource = pvfCatalogs.ItemShops.Source.Checksum
		}
		if lootService != nil {
			shopSource = lootService.Catalog.Source.Checksum
		}
		itemShops, err = pvfCatalogs.LoadItemShops(startup.ItemShop, shopSource)
		if err != nil {
			if pvfCatalogs.ItemShops != nil {
				return nil, nil, err
			}
			log.Printf("warning: load item shops (%s): %v", startup.ItemShop, err)
		} else {
			log.Printf("loaded item shops (%d shops) from %s", len(itemShops.Shops), startup.ItemShop)
		}
	}
	if itemShops == nil {
		log.Printf("warning: no item shop catalog; material-priced purchases cannot be resolved")
	} else if shopService != nil {
		shopService.ItemShops = itemShops
	}
	// 章节盒掉落（手册 P3 子项 3）。整表默认 enabled=false；只有 profile 显式开启
	// 才会叠加目录与槽位，未开启时连掷骰种子都不消耗。
	if path := os.Getenv("DFO_ODYSSEY_CHAPTER_DROP"); path != "" || pvfCatalogs.OdysseyDrop != nil {
		chapterDrop, e := pvfCatalogs.LoadOdysseyDrop(path)
		if e != nil {
			return nil, nil, e
		}
		if e = chapterDrop.ValidateBoxes(selectionBoxes); e != nil {
			return nil, nil, e
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
	if startup.AttunementRewards != "" || pvfCatalogs.Attunement != nil {
		attunement, e := pvfCatalogs.LoadAttunement(startup.AttunementRewards)
		if e != nil {
			return nil, nil, e
		}
		if lootService == nil {
			return nil, nil, errors.New("attunement rewards need the loot service")
		}
		if e = attunement.ValidateTemplates(lootService.Catalog); e != nil {
			return nil, nil, e
		}
		// 调参层在**源校验之后**才动手：先证明「表读对了」，再谈「我们想改哪里」。
		// ApplyRebalance 自己会复核权重不变量（每份 drop list 仍恰好 1e6），所以
		// 改完的表与源表在结构上同样合法。
		if _, _, e = attunement.ApplyRebalance(attunementRebalance); e != nil {
			return nil, nil, e
		}
		logAttunementRebalance(attunement, attunementRebalance)
		// 展开一层要用的礼包目录。缺了它就只能把包装丢在地上，而那正是本功能要
		// 修的那个报告，所以这里硬失败而不是退化成旧行为。
		if boosterCatalog == nil || len(boosterCatalog.Definitions) == 0 {
			return nil, nil, errors.New("attunement rewards need -booster-catalog: the table pays wrappers, and without the box catalog they cannot be opened at drop time")
		}
		boxes := boosterBoxSource{catalog: boosterCatalog}
		empties, unopenable, e := attunement.ValidateBoxes(boxes)
		if e != nil {
			return nil, nil, e
		}
		if len(unopenable) > 0 {
			log.Printf("warning: attunement rewards name %d box(es) this build cannot open; they will not be paid: %v", len(unopenable), unopenable)
		}
		lootService.Attunement = attunement
		lootService.RewardBoxes = boxes
		// 征兆系统（omen）：**默认生效**，无开关 —— 它是玩法本身。
		// 与上面的固定玩法行为保持一致。
		if omenRewards {
			if e := attunement.ValidateOmen(); e != nil {
				return nil, nil, e
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
		// 数据源是 PVF 直读（etc/rewardboostinfo/**.ctp 各自声明 [dungeon index]）；
		// 只有非直读的 JSON 模式才会走到文件。日志按真实来源打，别让人误以为在读 JSON。
		source := "PVF direct (etc/rewardboostinfo/**.ctp)"
		if pvfCatalogs.Attunement == nil {
			source = startup.AttunementRewards
		}
		log.Printf("loaded attunement rewards (%d dungeons %v, %d reward templates) from %s",
			len(attunement.Dungeons()), attunement.Dungeons(), len(attunement.Templates()), source)

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
		if n := attunement.Coupons(); n > 0 {
			log.Printf("omen system live: %d [coupon drop table] row(s) = omen stages drive the accumulation and settlement", n)
		}
	} else {
		log.Printf("warning: no attunement reward table; boundary-of-attunement clears pay no exclusive reward")
	}
	if lootService != nil && boosterCatalog != nil && (startup.ItemIndex != "" || pvfCatalogs.BlackPurgatory != nil) {
		rewards, err := pvfCatalogs.LoadBlackPurgatory("", boosterBoxSource{catalog: boosterCatalog}, func(id uint32) (catalog.LootItem, bool) {
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
	if startup.Responses != "" {
		b, err := os.ReadFile(startup.Responses)
		if err != nil {
			return nil, nil, err
		}
		paths := map[uint16]string{}
		if err = json.Unmarshal(b, &paths); err != nil {
			return nil, nil, err
		}
		for id, path := range paths {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, nil, err
			}
			if err = wire.ValidateServer(b); err != nil {
				return nil, nil, err
			}
			responses[id] = b
		}
	}
	if err := os.MkdirAll(startup.Output, 0700); err != nil {
		return nil, nil, err
	}
	var raw []byte
	var err error
	if startup.Fixture != "" {
		raw, err = os.ReadFile(startup.Fixture)
		if err != nil {
			return nil, nil, fmt.Errorf("read fixture %q: %v", startup.Fixture, err)
		}
		if err = wire.ValidateServer(raw); err != nil {
			return nil, nil, fmt.Errorf("validate fixture %q: %v", startup.Fixture, err)
		}
	}
	hub := newLanHub()
	// 沉月湖单人配置一律从**直读**推导（业主 2026-10-02：直读模式下不新增 JSON，
	// 也不再需要 -moon-solo-config 这种手工配置档）。频道/城镇/翻牌张数与**翻牌池**
	// 全部来自 PVF（池 = 第二层声明的掉落组里可结算的堆叠物品，权重照抄源里）；
	// 只有源里确实没有的"测试剩余次数"由代码常量给出。
	var moonConfig *moonSoloConfig
	if worldService != nil && characters != nil && dungeonCatalog != nil && startup.ChannelRefreshConfig != "" && startup.EntryBasicProbe && startup.EntryAdditionProbe {
		if pvfCatalogs.ChannelDirectory == nil {
			return nil, nil, errors.New("Moon 需要频道目录的 PVF 直读投影（preparePVFChannels 未装载）")
		}
		var moonErr error
		moonConfig, moonErr = defaultMoonSoloConfig(pvfCatalogs.ChannelDirectory, pvfCatalogs.ChannelTowns, dungeonCatalog, lootService)
		if moonErr != nil {
			return nil, nil, moonErr
		}
		if err = validateMoonResources(dungeonCatalog, lootService, gameStore); err != nil {
			return nil, nil, err
		}
		if err = worldService.ValidatePosition(255, false, moonConfig.Entry); err != nil {
			return nil, nil, errors.New(fmt.Sprint("Moon source entry: ", err))
		}
	}
	if itemService != nil {
		itemService.Catalog = lootService.Catalog
		itemService.BagRules = lootService.BagRules
		itemService.Equipment = lootService.Equipment
	}

	if err := validateTownArrivalScenes(worldService, questService, townArrivalScenes); err != nil {
		return nil, nil, err
	}
	// Starter Boost 662 装配：目录来自 PVF 直读（preparePVFBoostUp），NOTI108 活动清单
	// 只在开关打开时冻结一次。表体是**频道门 + 活动行合并后的那一张**（见
	// event_info_variant.go）：客户端对 108 是整表替换，只发活动行的第二条会被
	// 进城那条频道门表抹掉，城里就没有活动礼物图标。
	var boostCatalog *boostup.Catalog
	var boostEventInfo []byte
	if startup.BoostUpEvent {
		if pvfCatalogs.BoostUp == nil {
			// 玩法开关默认生效、不留第二套内容源（§0.14）：活动目录只从 PVF 直读来。
			// JSON/部分直读域的运行方式拿不到真源，本树其它 PVF-only 特性同样是
			// "缺源就降级 + 记一条 warning"（见 selection box / apocalypse 的告警），
			// 这里保持一致：活动整体不装配，玩家照常进镇，不伪造内容。
			log.Printf("warning: Starter Boost 662 disabled; PVF direct-read boostup domain is not prepared (-pvf-catalogs 加 boostup 才开启)")
			startup.BoostUpEvent = false
			startup.BoostUpChallenge = false
		} else if characters == nil || lootService == nil || worldService == nil {
			return nil, nil, errors.New("Starter Boost 需要持久化角色、掉落与世界服务")
		} else {
			// 选角（CMD8）与进城 announce 发同一条表；参考实现
			// `活动Boost与胶囊教学-20260927` 的两个发送点用的也是同一个快照。
			rows, ok := buildTownEventInfoTable(startup.BoostUpChallenge)
			if !ok {
				return nil, nil, errors.New("Starter Boost 事件表合并失败（频道门表形状异常）")
			}
			boostCatalog, boostEventInfo = pvfCatalogs.BoostUp, rows
			characters.Boost = boostCatalog
			if progressionService != nil {
				progressionService.Boost = boostCatalog
			}
			// 教学封存谓词： activated 且训练未毕业 = 仍在训练轨道，禁售/丢/寄/入仓生效。
			inventory.SetInBoostTraining(func(raw json.RawMessage) bool {
				st, e := boostup.ReadState(raw)
				return e == nil && st.Activated && !st.Training.Finished
			})
			log.Printf("Starter Boost event info snapshot ready: NOTI108 rows=%d bytes=%d",
				binary.LittleEndian.Uint16(boostEventInfo), len(boostEventInfo))
		}
	}
	prepared = &gatewayRuntime{
		config:                startup,
		accountOptionsPayload: accountOptionsPayload,
		apocalypseCatalog:     apocalypseCatalog,
		apocalypseClock:       apocalypseClock,
		boosterCatalog:        boosterCatalog,
		boostCatalog:          boostCatalog,
		boostEventInfo:        boostEventInfo,
		characters:            characters,
		channelDirectory:      pvfCatalogs.ChannelDirectory,
		channelInfo:           pvfCatalogs.ChannelInfo,
		channelTowns:          pvfCatalogs.ChannelTowns,
		channelGuides:         channelGuidesFromDirectory(pvfCatalogs.ChannelDirectory),
		developmentAccount:    developmentAccount,
		dungeonCatalog:        dungeonCatalog,
		fatigueService:        fatigueService,
		gameHost:              gameHost,
		gameStore:             gameStore,
		hub:                   hub,
		itemService:           itemService,
		journalRules:          journalRules,
		lootService:           lootService,
		lotteryPools:          lotteryPools,
		moonConfig:            moonConfig,
		oathGradePair:         oathGradePair,
		oathGradeTable:        oathGradeTable,
		oathInjectSpecs:       oathInjectSpecs,
		oathProgressSet:       oathProgressSet,
		odysseyChoices:        odysseyChoices,
		omenInfoBytes:         omenInfoBytes,
		omenState:             omenState,
		progressionService:    progressionService,
		questService:          questService,
		raw:                   raw,
		responses:             responses,
		rewards:               rewardNotifier,
		selectProbe:           selectProbe,
		selectionBoxes:        selectionBoxes,
		shopPilot:             shopPilot,
		shopService:           shopService,
		skinCatalog:           skinCatalog,
		townArrivalScenes:     townArrivalScenes,
		townCatalog:           townCatalog,
		townPolicy:            townPolicy,
		tutorialDungeons:      tutorialDungeons,
		tutorialRoutes:        tutorialRoutes,
		unifiedCharacTemplate: unifiedCharacTemplate,
		unsealService:         unsealService,
		vaultService:          vaultService,
		wearService:           wearService,
		worldService:          worldService,
	}
	return prepared, resources.close, nil
}

// Empty whitelists are valid; nil means assembly failed to pass the whitelist.
func validateTownArrivalScenes(worldService *world.Service, questService *quest.Service, scenes map[uint32]catalog.TownArrivalScene) error {
	if worldService != nil && questService != nil && scenes == nil {
		return errors.New("town arrival scene whitelist was not passed to world sessions")
	}
	return nil
}

// Resources register during startup and close once, in reverse acquisition order.
// Registration finishes before cleanup is handed to the serving loop.
type runtimeCleanup struct {
	once    sync.Once
	actions []func()
}

func (r *runtimeCleanup) add(action func()) { r.actions = append(r.actions, action) }

func (r *runtimeCleanup) close() {
	r.once.Do(func() {
		for i := len(r.actions) - 1; i >= 0; i-- {
			r.actions[i]()
		}
		r.actions = nil
	})
}

// channelGuidesFromDirectory 抽取每个 SemiRaid/Legion 频道类型的
// [guide dungeon index]（clientchannelinfo.etc 直读）—— SemiRaid 频道红门
// 直接进这个副本（Azure 102 -> 100004131，月湖 101 -> 100004137）。
func channelGuidesFromDirectory(dir *catalog.ChannelDirectory) map[uint32]uint32 {
	if dir == nil {
		return nil
	}
	out := map[uint32]uint32{}
	for channelType, a := range dir.ByType {
		if a.GuideDungeon != 0 && (a.IsLegion || a.IsRaid || a.IsPreRaid || a.IsSemiRaid) {
			out[channelType] = a.GuideDungeon
		}
	}
	return out
}
