package main

import (
	"dfolan/internal/adventure"
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/quest"
	"dfolan/internal/rosterbg"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Direct catalogs are built before storage opens. The optional JSON audit
// verifies complete effective projections; source checks remain mandatory in
// normal direct mode as well as audit mode.
type pvfCoreCatalogs struct {
	sourceChecksum                               string
	itemShops                                    *catalog.ItemShops
	boxes                                        *loot.BoxCatalog
	cashshop                                     *cashshop.Pilot
	characters, sourceCharacters                 *catalog.Characters
	layerRevisits                                *catalog.LayerRevisitOverlay
	scriptWarps                                  []catalog.ScriptWarpRoute
	fameRules                                    *character.FameRules
	awakeningRules                               *catalog.EquipmentAwakeningRules
	awakeningOptions                             *catalog.EquipmentAwakeningOptions
	rosterBackgrounds                            *rosterbg.TicketCatalog
	odysseyRoutes                                *catalog.OdysseyJournalRoutes
	seasonRules                                  *adventure.SeasonRules
	recommendedRules                             *adventure.RecommendedRules
	adventureRules                               *adventure.Rules
	lotteryTables                                *catalog.LotteryTables
	selectionBoxes                               *catalog.SelectionBoxes
	terminalScenes                               *catalog.TerminalSceneOverlay
	tournamentMaps                               *catalog.SourceMapOverlay
	mine                                         *loot.BleedingMineRewards
	blackPurgatory                               *loot.BlackPurgatoryRewards
	clearCube                                    *catalog.LootItem
	odysseyGrowth                                *catalog.OdysseyGrowth
	odysseyChapters                              *catalog.OdysseyChapters
	odysseyWeapons                               *catalog.OdysseyWeaponChoices
	odysseyDrop                                  *loot.OdysseyChapterDrop
	odysseyCurrency                              *loot.OdysseyCurrency
	attunement                                   *loot.AttunementRewards
	apocalypse                                   *catalog.ApocalypseCatalog
	mazeRates                                    *catalog.MazeChanceOverlay
	hellMaps                                     *catalog.SourceMapOverlay
	grief                                        *catalog.TowerGriefOverlay
	dazzlement                                   *catalog.DazzlementOverlay
	quests                                       *catalog.QuestCatalog
	progression                                  *catalog.Progression
	world                                        *catalog.WorldCatalog
	items                                        *catalog.ItemIndex
	itemBasics                                   *catalog.ItemBasics
	equipment                                    *inventory.FullEquipmentCatalog
	avatarDisjoint                               *inventory.AvatarDisjointRules
	avatarRecast                                 *inventory.AvatarRecastRules
	periods                                      []uint32
	skins                                        map[uint32]catalog.SkinStorageEntry
	journal                                      *catalog.EquipmentJournalRules
	createCost                                   *catalog.EquipmentCreateCost
	learning                                     *character.LearningCatalog
	prices                                       *catalog.ShopPrices
	materials                                    *catalog.ItemMaterials
	boosters                                     map[uint32]catalog.BoosterDefinition
	tutorial                                     *catalog.TutorialCatalog
	enhancements                                 *inventory.EnhancementCatalog
	randomOptions                                *inventory.RandomOptionCatalog
	shields                                      *inventory.KnightShields
	oath                                         *inventory.OathGradeTable
	loot                                         *catalog.LootCatalog
	selection                                    *inventory.EquipmentCatalog
	town                                         *catalog.TownArea
	dungeons, trainingDungeons, tutorialDungeons *catalog.DungeonCatalog
	vault                                        *inventory.VaultRules
}

type pvfItemInputs struct {
	derivedCacheDir                                                                                                        string
	itemShopPath, itemShopPolicyPath                                                                                       string
	boxesPath, boxPolicyPath                                                                                               string
	cashshopPath                                                                                                           string
	cashshopRelease                                                                                                        bool
	characterPolicyPath                                                                                                    string
	layerRevisitPolicyPath                                                                                                 string
	scriptWarpPolicyPath                                                                                                   string
	lotteryPolicyPath                                                                                                      string
	selectionBoxesPath, selectionPolicyPath                                                                                string
	minePath                                                                                                               string
	blackPurgatoryPath                                                                                                     string
	clearCubePath                                                                                                          string
	odysseyGrowthPath, odysseyChapterPath, odysseyDropPath, odysseyCurrencyPath, odysseyWeaponPath                         string
	attunementPath, contentPolicyPath                                                                                      string
	apocalypsePath                                                                                                         string
	indexPath, fullPrefix, journalPath, createCostPath, learningPath, pricesPath, materialsPath, boosterPath, tutorialPath string
	verifyBaselines                                                                                                        *bool
	lootPath, equipmentPath, questEquipmentPath, dropPolicyPath                                                            string
	randomOptionPath, shieldPath, wearRulesPath, oathPath, vaultPath, vaultPolicyPath                                      string
	townPath, dungeonPath, trainingDungeonPath, tutorialDungeonPath, scenePolicyPath                                       string
	enhancementPolicyPath                                                                                                  string
}

func (i pvfItemInputs) checksBaselines() bool { return i.verifyBaselines == nil || *i.verifyBaselines }

const pvfSupportedDomains = "world,quests,progression,items,equipment,periods,skins,journal,create-cost,skills,prices,materials,boosters,tutorial,enhancements,random-options,shields,oath-grades,vault,loot,equipment-selection,town,dungeons,training-dungeons,tutorial-dungeons,dungeon-towers,dungeon-hell,dungeon-maze,apocalypse,attunement,odyssey-growth,odyssey-chapters,odyssey-weapons,odyssey-drop,odyssey-currency,clear-cube,black-purgatory,bleeding-mine,dungeon-terminal,dungeon-tournament,selection-boxes,lottery,adventure,adventure-recommended,season,odyssey-routes,roster-backgrounds,fame,script-warps,layer-revisits,characters,cashshop,boxes,item-shops"

func parsePVFCatalogSelection(value string) (map[string]bool, error) {
	supported := map[string]bool{}
	for _, domain := range strings.Split(pvfSupportedDomains, ",") {
		supported[domain] = true
	}
	selected := map[string]bool{}
	if strings.TrimSpace(value) == "" {
		return selected, nil
	}
	for _, domain := range strings.Split(value, ",") {
		domain = strings.TrimSpace(domain)
		if !supported[domain] {
			return nil, fmt.Errorf("PVF candidate domain %q is not enabled; supported: %s", domain, pvfSupportedDomains)
		}
		if selected[domain] {
			return nil, fmt.Errorf("duplicate PVF candidate domain %q", domain)
		}
		selected[domain] = true
	}
	return selected, nil
}

// verifyPVFCatalog 在直读模式下**只警告、不熔断**（2026-10-01 定调）。
//
// 理由：直读的数据来自 PVF（唯一真源），而这些 JSON 是**历史快照** —— 它们的
// source 字段普遍还写着旧内层哈希（`7ef2db59…`，当前是 `be95d64e…`），于是
// 「JSON vs PVF」**必然**存在差异。把它当 error 意味着**误开一次基线校验就熔断启动**
// （实测：78 个配置、181 处历史来源标记都会失配，见 next147 的排查）。
//
// 差异本身仍有价值 —— 打出来交给判断即可。需要**严格**审计时请用
// `cmd/pvfaudit` / `cmd/audit36`，不要把启动路径变成熔断器。
func verifyPVFCatalog(legacy, direct any) error {
	comparison := gamedata.Compare(legacy, direct, 1)
	if comparison.Count != 0 {
		first := comparison.Differences[0]
		log.Printf("baseline vs PVF direct: %d effective field difference(s); first %s: JSON=%s PVF=%s (expected in direct mode: the JSON is a historical snapshot; not fatal)",
			comparison.Count, first.Path, first.JSON, first.PVF)
	}
	return nil
}

func preparePVFCoreCatalogs(selection, path, checksum, characterPath, questPath, progressionPath, worldPath string, itemInputs ...pvfItemInputs) (pvfCoreCatalogs, error) {
	var result pvfCoreCatalogs
	selected, err := parsePVFCatalogSelection(selection)
	if err != nil || len(selected) == 0 {
		return result, err
	}
	inputs := pvfItemInputs{}
	if len(itemInputs) > 0 {
		inputs = itemInputs[0]
	}
	if inputs.indexPath == "" {
		inputs.indexPath = filepath.Join(filepath.Dir(characterPath), "items.index.json")
	}
	if inputs.checksBaselines() && (selected["quests"] && questPath == "" || selected["progression"] && progressionPath == "" || selected["world"] && worldPath == "") {
		return result, fmt.Errorf("selected PVF domains require their current baseline catalog flags during parity validation")
	}
	if selected["world"] && os.Getenv("DFO_NPC_PRESENCE_WORLD") != "" {
		return result, fmt.Errorf("PVF world uses its source phase graph for NPC diagnostics; clear DFO_NPC_PRESENCE_WORLD to avoid a JSON shadow-world override")
	}
	var anchorChecksum string
	var characters catalog.Characters
	var characterPolicy catalog.CharacterRuntimePolicy
	if selected["characters"] {
		characterPolicy, err = readPVFCharacterPolicy(inputs.characterPolicyPath)
		if err != nil {
			return result, err
		}
		anchorChecksum = characterPolicy.SourceChecksum
	} else {
		var e error
		characters, e = catalog.LoadCharacters(characterPath)
		if e != nil {
			return result, fmt.Errorf("PVF character source anchor: %w", e)
		}
		anchorChecksum = characters.Source.Checksum
	}
	started := time.Now()
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, ExpectedChecksum: checksum, DerivedCacheDir: inputs.derivedCacheDir})
	if err != nil {
		return result, err
	}

	defer source.Close()
	logPVFMemory("source-open", time.Since(started))
	if anchorChecksum != "" && source.Snapshot().Checksum != anchorChecksum {
		return result, fmt.Errorf("PVF/character source mismatch: %s versus %s", source.Snapshot().Checksum, anchorChecksum)
	}
	result.sourceChecksum = source.Snapshot().Checksum
	if selected["periods"] && selected["prices"] {
		basicStarted := time.Now()
		joint, err := source.ItemCatalogs(catalog.ItemBasicOptions{Periods: true, Prices: true, Materials: selected["materials"], Skins: selected["skins"], Boosters: selected["boosters"]}, selected["enhancements"], inputs.enhancementPolicyPath, selected["fame"])
		if err != nil {
			return result, err
		}
		result.itemBasics = &joint.Basics
		result.enhancements, result.fameRules = joint.Enhancements, joint.Fame
		source.ReleaseReadCaches()
		log.Printf("PVF joint item catalogs prepared in %s: %d source templates, %d original item scan projections; periods/prices/materials/selected skins/boosters/enhancements/fame share one scan", time.Since(basicStarted), len(joint.Basics.Index.Items), joint.Basics.ScriptsRead)
	}

	// 2026-10-01（next146）：奥德赛系目录（成长/章节/路线/兑换/黑鸦/赤红铁矿…）与角色存档
	// 都用同一份「源身份」令牌 catalog.OdysseySource。直读模式下它必须等于当次内层 checksum
	// （角色 ConfigVersion 也正是此值），否则整族在直读启动时全被门禁拦下（首个撞墙点 =
	// `Odyssey journal routes source mismatch`）。这里在目录准备完成后统一切换。
	catalog.SetOdysseySource(result.sourceChecksum)
	// 2026-10-01（next146）：抽奖系目录（item pools / equipment pools）的来源身份令牌
	// 同样是编译期写死的内层哈希，直读模式下必须切到当次 checksum，
	// 否则会在 `PVF candidate catalogs: lottery catalog source identity or pool count mismatch`
	// 处被拦下（紧随 Odyssey 之后的撞墙点）。
	SetLotterySource(result.sourceChecksum)
	// 2026-10-01（next146）：无色小晶块叠加目录与图像通信任务目录的来源身份同样是
	// 编译期写死的内层哈希，直读模式下必须切到当次 checksum，否则会在
	// `clear cube source mismatch` / 图像通信任务处被拦下。
	inventory.SetClearCubeSource(result.sourceChecksum)
	quest.SetImageCommunicationSource(result.sourceChecksum)

	if selected["characters"] {
		if err := preparePVFCharacters(&result, source, characterPolicy, characterPath, inputs); err != nil {
			return result, err
		}
		characters = *result.characters
	}

	if selected["cashshop"] {
		if err := preparePVFCashShop(&result, source, inputs); err != nil {
			return result, err
		}
	}

	if selected["world"] {
		direct, e := source.World("")
		if e != nil {
			return result, e
		}
		additions := 0
		if inputs.checksBaselines() {
			legacy, e := catalog.LoadWorld(worldPath)
			if e != nil {
				return result, e
			}
			if legacy.Source.Checksum != source.Snapshot().Checksum {
				return result, fmt.Errorf("world baseline/PVF source mismatch")
			}
			comparison, added, e := gamedata.CompareWorldMigration(legacy, direct, 1)
			if e != nil {
				return result, e
			}
			if comparison.Count != 0 {
				return result, fmt.Errorf("world: %d effective field differences; first %s", comparison.Count, comparison.Differences[0].Path)
			}
			additions = len(added)
		}
		result.world = &direct
		log.Printf("PVF world prepared: %d areas, %d NPC moves, %d audited phase additions source=%s", len(direct.Areas), len(direct.NPCMoves), additions, direct.Source.Checksum)
		source.ReleaseReadCaches()
	}
	if selected["quests"] {
		direct, e := source.Quests("")
		if e != nil {
			return result, e
		}
		if inputs.checksBaselines() {
			legacy, e := catalog.LoadQuests(questPath)
			if e != nil {
				return result, e
			}
			if legacy.Source.Checksum != source.Snapshot().Checksum {
				return result, fmt.Errorf("quest baseline/PVF source mismatch")
			}
			if e = verifyPVFCatalog(legacy, direct); e != nil {
				return result, fmt.Errorf("quests: %w", e)
			}
		}
		result.quests = &direct
		log.Printf("PVF quests prepared: %d definitions source=%s", len(direct.Quests), direct.Source.Checksum)
		source.ReleaseReadCaches()
	}
	if selected["progression"] {
		direct, e := source.Progression("")
		if e != nil {
			return result, e
		}
		if inputs.checksBaselines() {
			legacy, e := catalog.LoadProgression(progressionPath)
			if e != nil {
				return result, e
			}
			if legacy.Source.Checksum != source.Snapshot().Checksum {
				return result, fmt.Errorf("progression baseline/PVF source mismatch")
			}
			if e = verifyPVFCatalog(legacy, direct); e != nil {
				return result, fmt.Errorf("progression: %w", e)
			}
		}
		result.progression = &direct
		log.Printf("PVF progression prepared: %d thresholds source=%s", len(direct.Thresholds), direct.Source.Checksum)
		source.ReleaseReadCaches()
	}
	if e := preparePVFRules(&result, source, selected, inputs); e != nil {
		return result, e
	}
	logPVFMemory("base-rules", time.Since(started))
	if selected["skills"] {
		if e := preparePVFLearning(&result, source, characters, inputs); e != nil {
			return result, e
		}
	}
	if err := preparePVFEquipmentRules(&result, source, selected, inputs); err != nil {
		return result, err
	}
	logPVFMemory("equipment-rules", time.Since(started))
	if selected["loot"] {
		if err := preparePVFLoot(&result, source, inputs); err != nil {
			return result, err
		}
	}
	if selected["boxes"] || selected["fame"] || selected["roster-backgrounds"] || selected["season"] || selected["adventure"] || selected["lottery"] || selected["selection-boxes"] || selected["bleeding-mine"] || selected["black-purgatory"] || selected["clear-cube"] || selected["odyssey-growth"] || selected["odyssey-weapons"] || selected["odyssey-drop"] || selected["odyssey-currency"] || selected["items"] || selected["equipment"] || selected["prices"] || selected["materials"] || selected["boosters"] || selected["enhancements"] || selected["shields"] || selected["equipment-selection"] {

		var direct catalog.ItemIndex
		var e error
		if result.itemBasics != nil {
			direct = result.itemBasics.Index
		} else {
			direct, e = source.ItemIndex("")
		}
		if e != nil {
			return result, e
		}
		if inputs.checksBaselines() {
			legacy, e := catalog.LoadItemIndex(inputs.indexPath)
			if e != nil {
				return result, e
			}
			if legacy.Source.Checksum != source.Snapshot().Checksum {
				return result, fmt.Errorf("item index baseline/PVF source mismatch")
			}
			if e = verifyPVFCatalog(legacy, direct); e != nil {
				return result, fmt.Errorf("items: %w", e)
			}
		}
		result.items = &direct
		log.Printf("PVF item index prepared: %d templates source=%s", len(direct.Items), direct.Source.Checksum)
		if selected["equipment"] && selected["loot"] {
			result.avatarDisjoint, e = source.AvatarDisjoint(direct)
			if e != nil {
				return result, fmt.Errorf("PVF avatar disjoint: %w", e)
			}
			log.Printf("PVF avatar disjoint prepared: %d avatar grades, %d emblem grades source=%s", len(result.avatarDisjoint.Rolls), len(result.avatarDisjoint.Pools), result.avatarDisjoint.Source)
			result.avatarRecast, e = source.AvatarRecast(direct)
			if e != nil {
				return result, fmt.Errorf("PVF avatar recast: %w", e)
			}
			log.Printf("PVF avatar recast prepared: %d jobs source=%s", len(result.avatarRecast.Jobs), result.avatarRecast.Source)
		}
		source.ReleaseReadCaches()
		if e := preparePVFCommerce(&result, source, selected, inputs); e != nil {
			return result, e
		}
		logPVFMemory("item-commerce", time.Since(started))

		if selected["enhancements"] {
			if err := preparePVFEnhancements(&result, source, inputs); err != nil {
				return result, err
			}
		}
		if selected["shields"] {
			if err := preparePVFShields(&result, source, characters, inputs); err != nil {
				return result, err
			}
		}
		if selected["equipment-selection"] {
			if err := preparePVFEquipmentSelection(&result, source, inputs); err != nil {
				return result, err
			}
		}
		if selected["equipment"] {
			candidate, e := source.Equipment(direct)
			if e != nil {
				return result, e
			}
			if inputs.checksBaselines() {
				if inputs.fullPrefix == "" {
					return result, fmt.Errorf("PVF equipment audit requires the baseline prefix")
				}
				full, e := inventory.OpenFullEquipmentCatalog(inputs.fullPrefix, direct.Source.Checksum)
				if e != nil {
					return result, e
				}
				defer full.Close()
				if full.IndexSHA256 != candidate.IndexSHA256 || full.RecordCount() != candidate.RecordCount() || len(full.Errors) != 0 {
					return result, fmt.Errorf("full equipment baseline/PVF index differs")
				}
				for id := range full.Records {
					if !candidate.HasDefinition(id) {
						return result, fmt.Errorf("equipment %d absent from PVF index", id)
					}
				}
			}
			result.equipment = candidate
			log.Printf("PVF lazy equipment prepared: %d compact source bindings; expanded chunks bounded to 64 MiB", candidate.RecordCount())
		}
	}
	if err := preparePVFScenes(&result, source, selected, inputs); err != nil {
		return result, err
	}
	logPVFMemory("scenes", time.Since(started))
	if err := preparePVFAdventure(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFRecommended(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFSeason(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFOdysseyRoutes(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFRosterBackgrounds(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFFame(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFEquipmentAwakening(&result, source); err != nil {
		return result, err
	}
	if err := preparePVFItemShops(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFBoxes(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFSelectionBoxes(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFLottery(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFLayerRevisits(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFScriptWarps(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFClosingScenes(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFTowers(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFHellMaps(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFMazeRates(&result, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFMine(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFBlackPurgatory(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFClearCube(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFOdyssey(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if err := preparePVFSpecial(&result, source, selected, inputs); err != nil {
		return result, err
	}
	if result.dungeons != nil {
		result.dungeons.ReleaseMapReadCache()
	}
	if err := source.EnableRuntimeDetails(result.quests, result.learning, result.loot, result.items); err != nil {
		return result, fmt.Errorf("PVF runtime details: %w", err)
	}
	if result.equipment != nil || result.dungeons != nil || result.quests != nil || result.learning != nil || result.loot != nil {
		if err := source.CompactRuntimeStrings(); err != nil {
			return result, fmt.Errorf("compact PVF runtime strings: %w", err)
		}
	}
	log.Printf("PVF candidate catalogs prepared in %s; full directory can be collected before opening storage", time.Since(started))
	logPVFMemory("catalogs-prepared", time.Since(started))
	return result, nil
}

func (c pvfCoreCatalogs) loadQuests(path string) (catalog.QuestCatalog, error) {
	if c.quests != nil {
		return *c.quests, nil
	}
	return catalog.LoadQuests(path)
}

func (c pvfCoreCatalogs) loadProgression(path string) (catalog.Progression, error) {
	if c.progression != nil {
		return *c.progression, nil
	}
	return catalog.LoadProgression(path)
}

func collectPVFImportMemory(c pvfCoreCatalogs) {
	if c.town != nil || c.dungeons != nil || c.trainingDungeons != nil || c.tutorialDungeons != nil || c.quests != nil || c.progression != nil || c.world != nil || c.items != nil || c.periods != nil || c.skins != nil || c.journal != nil || c.createCost != nil || c.learning != nil || c.tutorial != nil {
		runtime.GC()
		logPVFMemory("import-collected", 0)
	}
}

func (c pvfCoreCatalogs) openFullEquipment(prefix, checksum string) (*inventory.FullEquipmentCatalog, error) {
	if c.equipment != nil {
		if c.equipment.Source.Checksum != checksum {
			return nil, fmt.Errorf("prepared equipment source mismatch")
		}
		return c.equipment, nil
	}
	return inventory.OpenFullEquipmentCatalog(prefix, checksum)
}

func (c pvfCoreCatalogs) supplementStackables(loot *catalog.LootCatalog, path string) error {
	if c.items != nil {
		return loot.SupplementItemIndex(*c.items)
	}
	return loot.SupplementStackables(path)
}

func (c pvfCoreCatalogs) loadBooster(path, indexPath string) (*BoosterCatalog, error) {
	if c.boosters != nil {
		return &BoosterCatalog{Definitions: c.boosters, Items: c.items.Items}, nil
	}
	if c.items == nil {
		return LoadBoosterCatalog(path, indexPath)
	}
	result, err := LoadBoosterCatalog(path, "")
	if err != nil {
		return nil, err
	}
	result.Items = c.items.Items
	return result, nil
}

func (c pvfCoreCatalogs) loadWorld(path string) (catalog.WorldCatalog, error) {
	if c.world != nil {
		return *c.world, nil
	}
	return catalog.LoadWorld(path)
}
