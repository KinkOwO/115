package gamedata

import (
	"dfolan/internal/adventure"
	"dfolan/internal/boostup"
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

// Direct catalogs are built before storage opens. The optional JSON audit
// verifies complete effective projections; source checks remain mandatory in
// normal direct mode as well as audit mode.
type Catalogs struct {
	selected                     map[string]bool
	prepared                     map[string]bool
	SourceChecksum               string
	ItemShops                    *catalog.ItemShops
	Boxes                        *inventory.BoxCatalog
	CashShop                     *cashshop.Pilot
	Characters, SourceCharacters *catalog.Characters
	LayerRevisits                *catalog.LayerRevisitOverlay
	ScriptWarps                  []catalog.ScriptWarpRoute
	FameRules                    *character.FameRules
	AwakeningRules               *catalog.EquipmentAwakeningRules
	AwakeningOptions             *catalog.EquipmentAwakeningOptions
	SoleRules                    *catalog.SoleEquipmentRules
	ChannelDirectory             *catalog.ChannelDirectory
	ChannelInfo                  *catalog.ChannelInfo
	ChannelTowns                 map[uint32]catalog.TownArea
	RaidEntrances                map[uint32]catalog.RaidEntrance
	RosterBackgrounds            *character.RosterBackgroundTicketCatalog
	OdysseyRoutes                *catalog.OdysseyJournalRoutes
	SeasonRules                  *adventure.SeasonRules
	RecommendedRules             *adventure.RecommendedRules
	AdventureRules               *adventure.Rules
	LotteryTables                *catalog.LotteryTables
	SelectionBoxes               *catalog.SelectionBoxes
	TerminalScenes               *catalog.TerminalSceneOverlay
	TournamentMaps               *catalog.SourceMapOverlay
	Mine                         *loot.BleedingMineRewards
	BlackPurgatory               *loot.BlackPurgatoryRewards
	ClearCube                    *catalog.LootItem
	OdysseyGrowth                *catalog.OdysseyGrowth
	OdysseyChapters              *catalog.OdysseyChapters
	OdysseyCompletionRewards     *catalog.OdysseyCompletionRewards
	OdysseyWeapons               *catalog.OdysseyWeaponChoices
	OdysseyDrop                  *loot.OdysseyChapterDrop
	OdysseyCurrency              *loot.OdysseyCurrency
	Attunement                   *loot.AttunementRewards
	Apocalypse                   *catalog.ApocalypseCatalog
	MazeRates                    *catalog.MazeChanceOverlay
	HellMaps                     *catalog.SourceMapOverlay
	HellRules                    *catalog.HellPartyRules
	Grief                        *catalog.TowerGriefOverlay
	Dazzlement                   *catalog.DazzlementOverlay
	Quests                       *catalog.QuestCatalog
	Progression                  *catalog.Progression
	World                        *catalog.WorldCatalog
	Items                        *catalog.ItemIndex
	ItemBasics                   *catalog.ItemBasics
	Equipment                    *inventory.FullEquipmentCatalog
	AvatarDisjoint               *inventory.AvatarDisjointRules
	AvatarSockets                *inventory.AvatarSocketRules
	EmblemInlay                  *inventory.EmblemInlayRules
	AvatarRecast                 *inventory.AvatarRecastRules
	EmblemCompound               *inventory.EmblemCompoundRules
	Periods                      []uint32
	Skins                        map[uint32]catalog.SkinStorageEntry
	Journal                      *catalog.EquipmentJournalRules
	CreateCost                   *catalog.EquipmentCreateCost
	Transform                    *catalog.EquipmentTransformSystem
	// Points 是逐件「套装积分 / 誓约积分」表（setpointinfo.cos / oathpointinfo.cos），
	// 与 transform 同域装载；服务端算角色总分、推 NOTI2634 时用。
	Points                                       *catalog.PointRules
	Learning                                     *character.LearningCatalog
	Prices                                       *catalog.ShopPrices
	Materials                                    *catalog.ItemMaterials
	Boosters                                     map[uint32]catalog.BoosterDefinition
	Tutorial                                     *catalog.TutorialCatalog
	Enhancements                                 *inventory.EnhancementCatalog
	RandomOptions                                *inventory.RandomOptionCatalog
	Shields                                      *inventory.KnightShields
	Oath                                         *inventory.OathGradeTable
	Loot                                         *catalog.LootCatalog
	Selection                                    *inventory.EquipmentCatalog
	Town                                         *catalog.TownArea
	Dungeons, TrainingDungeons, TutorialDungeons *catalog.DungeonCatalog
	Vault                                        *inventory.VaultRules
	// BoostUp 是活动 662（新手成长胶囊）的原生目录；nil = 该直读域未选中。
	BoostUp *boostup.Catalog
}

type CatalogInputs struct {
	Selection, ArchivePath, ArchiveChecksum                                                        string
	CharacterPath, QuestPath, ProgressionPath, WorldPath                                           string
	BaselineDir                                                                                    string
	DerivedCacheDir                                                                                string
	ItemShopPath, ItemShopPolicyPath                                                               string
	BoxesPath, BoxPolicyPath                                                                       string
	CashshopRelease                                                                                bool
	CharacterPolicyPath                                                                            string
	LayerRevisitPolicyPath                                                                         string
	ScriptWarpPolicyPath                                                                           string
	LotteryPolicyPath                                                                              string
	SelectionPolicyPath                                                                            string
	MinePath                                                                                       string
	BlackPurgatoryPath                                                                             string
	ClearCubePath                                                                                  string
	OdysseyGrowthPath, OdysseyChapterPath, OdysseyDropPath, OdysseyCurrencyPath, OdysseyWeaponPath string
	AttunementPath, ContentPolicyPath                                                              string
	ApocalypsePath                                                                                 string
	IndexPath, FullPrefix, JournalPath, CreateCostPath, MaterialsPath, TutorialPath                string
	VerifyBaselines                                                                                bool
	LootPath, EquipmentPath, QuestEquipmentPath, DropPolicyPath                                    string
	RandomOptionPath, ShieldPath, WearRulesPath, OathPath, VaultPath, VaultPolicyPath              string
	TownPath, DungeonPath, TrainingDungeonPath, TutorialDungeonPath, ScenePolicyPath               string
	EnhancementPolicyPath                                                                          string
	// BoostChallenge 打开活动 662 毕业后的可选挑战（665）源绑定；关闭时不解析，挑战路径 fail-closed。
	BoostChallenge bool
}

type CatalogAdapters struct {
	ValidatedSource func(string)
	ValidateLottery func(catalog.LotteryTables, catalog.ItemIndex, string, bool) error
	RewardBoxes     func(map[uint32]catalog.BoosterDefinition, map[uint32]catalog.ItemIndexEntry) loot.RewardBoxSource
}

func (i CatalogInputs) checksBaselines() bool { return i.VerifyBaselines }

const SupportedDomains = "world,quests,progression,items,equipment,periods,skins,journal,create-cost,transform,skills,prices,materials,boosters,tutorial,enhancements,random-options,shields,oath-grades,vault,loot,equipment-selection,town,dungeons,training-dungeons,tutorial-dungeons,dungeon-towers,dungeon-hell,dungeon-maze,apocalypse,attunement,odyssey-growth,odyssey-chapters,odyssey-weapons,odyssey-drop,odyssey-currency,clear-cube,black-purgatory,bleeding-mine,dungeon-terminal,dungeon-tournament,selection-boxes,lottery,adventure,adventure-recommended,season,odyssey-routes,roster-backgrounds,fame,script-warps,layer-revisits,characters,cashshop,boxes,item-shops,boostup"

func (c *Catalogs) Selected(domain string) bool { return c != nil && c.selected[domain] }
func (c *Catalogs) Prepared(domain string) bool { return c != nil && c.prepared[domain] }

// RequireSelected keeps selected PVF domains from silently loading their legacy projection.
func (c *Catalogs) RequireSelected(domain string, ready bool) error {
	if !c.Selected(domain) {
		return nil
	}
	if !c.Prepared(domain) || !ready {
		return fmt.Errorf("selected PVF %s projection is not prepared", domain)
	}
	return nil
}

func nativeContentRequired(domain string) error {
	return fmt.Errorf("%s requires a prepared native PVF domain; JSON runtime catalogs are retired", domain)
}

func parsePVFCatalogSelection(value string) (map[string]bool, error) {
	supported := map[string]bool{}
	for _, domain := range strings.Split(SupportedDomains, ",") {
		supported[domain] = true
	}
	selected := map[string]bool{}
	if strings.TrimSpace(value) == "" {
		return selected, nil
	}
	for _, domain := range strings.Split(value, ",") {
		domain = strings.TrimSpace(domain)
		if !supported[domain] {
			return nil, fmt.Errorf("PVF candidate domain %q is not enabled; supported: %s", domain, SupportedDomains)
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
// `auditPVFCatalog` 或 `cmd/pvfaudit` / `cmd/audit36`，不要把启动路径变成熔断器。
func verifyPVFCatalog(legacy, direct any) error {
	comparison := Compare(legacy, direct, 1)
	if comparison.Count != 0 {
		first := comparison.Differences[0]
		log.Printf("baseline vs PVF direct: %d effective field difference(s); first %s: JSON=%s PVF=%s (expected in direct mode: the JSON is a historical snapshot; not fatal)",
			comparison.Count, first.Path, first.JSON, first.PVF)
	}
	return nil
}

// auditPVFCatalog 是严格版比较：任一有效字段差异即返回 error，绝不静默通过。
//
// 与 `verifyPVFCatalog` 的告警语义相反，它供审计助手（`auditAdventureRules` /
// `auditBlackPurgatory` / `auditPVFBoxes` / `auditPVFEnhancements`）与奇偶性测试
// 使用。这些调用点会先归一化已知的历史来源标记，剩下的任何差异都是真实内容漂移，
// 必须熔断 —— 否则「JSON 换了内容却静默通过」正是直读模式整片失效的根因。
func auditPVFCatalog(legacy, direct any) error {
	comparison := Compare(legacy, direct, 1)
	if comparison.Count != 0 {
		first := comparison.Differences[0]
		return fmt.Errorf("PVF candidate has %d effective field differences; first %s: JSON=%s PVF=%s", comparison.Count, first.Path, first.JSON, first.PVF)
	}
	return nil
}

func (c *Catalogs) validateSelectedProjections() error {
	for domain := range c.selected {
		var ready bool
		switch domain {
		case "world":
			ready = c.World != nil
		case "quests":
			ready = c.Quests != nil
		case "progression":
			ready = c.Progression != nil
		case "items":
			ready = c.Items != nil
		case "equipment":
			ready = c.Equipment != nil && c.Items != nil
		case "journal":
			ready = c.Journal != nil
		case "create-cost":
			ready = c.CreateCost != nil
		case "transform":
			ready = c.Transform != nil
		case "skills":
			ready = c.Learning != nil
		case "prices":
			ready = c.Prices != nil
		case "materials":
			ready = c.Materials != nil
		case "tutorial":
			ready = c.Tutorial != nil
		case "enhancements":
			ready = c.Enhancements != nil
		case "random-options":
			ready = c.RandomOptions != nil
		case "shields":
			ready = c.Shields != nil
		case "oath-grades":
			ready = c.Oath != nil
		case "vault":
			ready = c.Vault != nil
		case "loot":
			ready = c.Loot != nil && c.Items != nil
		case "equipment-selection":
			ready = c.Selection != nil && c.Items != nil
		case "town":
			ready = c.Town != nil
		case "dungeons":
			ready = c.Dungeons != nil
		case "training-dungeons":
			ready = c.TrainingDungeons != nil
		case "tutorial-dungeons":
			ready = c.TutorialDungeons != nil
		case "dungeon-towers":
			ready = c.Grief != nil && c.Dazzlement != nil
		case "dungeon-hell":
			ready = c.HellMaps != nil
		case "dungeon-maze":
			ready = c.MazeRates != nil
		case "apocalypse":
			ready = c.Apocalypse != nil
		case "attunement":
			ready = c.Attunement != nil && c.Items != nil
		case "odyssey-growth":
			ready = c.OdysseyGrowth != nil && c.Items != nil
		case "odyssey-chapters":
			ready = c.OdysseyChapters != nil
		case "odyssey-weapons":
			ready = c.OdysseyWeapons != nil && c.Items != nil
		case "odyssey-drop":
			ready = c.OdysseyDrop != nil && c.Items != nil
		case "odyssey-currency":
			ready = c.OdysseyCurrency != nil && c.Items != nil
		case "clear-cube":
			ready = c.ClearCube != nil && c.Items != nil
		case "black-purgatory":
			ready = c.BlackPurgatory != nil && c.Items != nil
		case "bleeding-mine":
			ready = c.Mine != nil && c.Items != nil
		case "dungeon-terminal":
			ready = c.TerminalScenes != nil
		case "dungeon-tournament":
			ready = c.TournamentMaps != nil
		case "selection-boxes":
			ready = c.SelectionBoxes != nil && c.Items != nil
		case "lottery":
			ready = c.LotteryTables != nil && c.Items != nil
		case "adventure":
			ready = c.AdventureRules != nil
		case "adventure-recommended":
			ready = c.RecommendedRules != nil
		case "season":
			ready = c.SeasonRules != nil
		case "odyssey-routes":
			ready = c.OdysseyRoutes != nil
		case "roster-backgrounds":
			ready = c.RosterBackgrounds != nil && c.Items != nil
		case "fame":
			ready = c.FameRules != nil && c.Items != nil
		case "layer-revisits":
			ready = c.LayerRevisits != nil
		case "characters":
			ready = c.Characters != nil
		case "cashshop":
			ready = c.CashShop != nil
		case "boxes":
			ready = c.Boxes != nil && c.Items != nil
		case "item-shops":
			ready = c.ItemShops != nil && c.Items != nil
		case "boostup":
			ready = c.BoostUp != nil && c.Items != nil
		// Empty slices/maps are valid for these native domains. Their selected
		// membership records that the source import completed successfully.
		case "boosters":
			ready = c.Items != nil
		case "periods", "skins", "script-warps":
			ready = true
		default:
			ready = false
		}
		if !ready {
			return fmt.Errorf("selected PVF %s projection is missing", domain)
		}
	}
	return nil
}

func PrepareCatalogs(inputs CatalogInputs, adapters CatalogAdapters) (*Catalogs, error) {
	result := Catalogs{}
	selection := inputs.Selection
	path := inputs.ArchivePath
	checksum := inputs.ArchiveChecksum
	characterPath := inputs.CharacterPath
	selected, err := parsePVFCatalogSelection(selection)
	if err != nil {
		return nil, err
	}
	result.selected = selected
	if len(selected) == 0 {
		return &result, nil
	}

	if inputs.IndexPath == "" {
		inputs.IndexPath = filepath.Join(filepath.Dir(characterPath), "items.index.json")
	}
	if selected["world"] && os.Getenv("DFO_NPC_PRESENCE_WORLD") != "" {
		return &result, fmt.Errorf("PVF world uses its source phase graph for NPC diagnostics; clear DFO_NPC_PRESENCE_WORLD to avoid a JSON shadow-world override")
	}
	var anchorChecksum string
	var characters catalog.Characters
	var characterPolicy catalog.CharacterRuntimePolicy
	if selected["characters"] {
		characterPolicy, err = readPVFCharacterPolicy(inputs.CharacterPolicyPath)
		if err != nil {
			return &result, err
		}
		anchorChecksum = characterPolicy.SourceChecksum
	}
	started := time.Now()
	source, err := Open(Options{Mode: PVF, ArchivePath: path, ExpectedChecksum: checksum, DerivedCacheDir: inputs.DerivedCacheDir})
	if err != nil {
		return &result, err
	}

	defer source.Close()
	logPVFMemory("source-open", time.Since(started))
	if anchorChecksum != "" && source.Snapshot().Checksum != anchorChecksum {
		return &result, fmt.Errorf("PVF/character source mismatch: %s versus %s", source.Snapshot().Checksum, anchorChecksum)
	}
	result.SourceChecksum = source.Snapshot().Checksum
	// Bind runtime source identity after verification and before native imports.
	if adapters.ValidatedSource != nil {
		adapters.ValidatedSource(result.SourceChecksum)
	}
	if selected["periods"] && selected["prices"] {
		basicStarted := time.Now()
		joint, err := source.ItemCatalogs(catalog.ItemBasicOptions{Periods: true, Prices: true, Materials: selected["materials"], Skins: selected["skins"], Boosters: selected["boosters"]}, selected["enhancements"], inputs.EnhancementPolicyPath, selected["fame"])
		if err != nil {
			return &result, err
		}
		result.ItemBasics = &joint.Basics
		result.Enhancements, result.FameRules = joint.Enhancements, joint.Fame
		source.ReleaseReadCaches()
		log.Printf("PVF joint item catalogs prepared in %s: %d source templates, %d original item scan projections; periods/prices/materials/selected skins/boosters/enhancements/fame share one scan", time.Since(basicStarted), len(joint.Basics.Index.Items), joint.Basics.ScriptsRead)
	}

	if selected["characters"] {
		if err := preparePVFCharacters(&result, source, characterPolicy, characterPath, inputs); err != nil {
			return &result, err
		}
		characters = *result.Characters
	}

	if selected["cashshop"] {
		if err := preparePVFCashShop(&result, source, inputs); err != nil {
			return &result, err
		}
	}

	if selected["world"] {
		direct, e := source.World("")
		if e != nil {
			return &result, e
		}
		result.World = &direct
		log.Printf("PVF world prepared: %d areas, %d NPC moves source=%s", len(direct.Areas), len(direct.NPCMoves), direct.Source.Checksum)
		source.ReleaseReadCaches()
	}
	if selected["quests"] {
		direct, e := source.Quests("")
		if e != nil {
			return &result, e
		}
		result.Quests = &direct
		log.Printf("PVF quests prepared: %d definitions source=%s", len(direct.Quests), direct.Source.Checksum)
		source.ReleaseReadCaches()
	}
	if selected["progression"] {
		direct, e := source.Progression("")
		if e != nil {
			return &result, e
		}
		result.Progression = &direct
		log.Printf("PVF progression prepared: %d thresholds source=%s", len(direct.Thresholds), direct.Source.Checksum)
		source.ReleaseReadCaches()
	}
	if e := preparePVFRules(&result, source, selected, inputs); e != nil {
		return &result, e
	}
	logPVFMemory("base-rules", time.Since(started))
	if selected["skills"] {
		if e := preparePVFLearning(&result, source, characters); e != nil {
			return &result, e
		}
	}
	if err := preparePVFEquipmentRules(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	logPVFMemory("equipment-rules", time.Since(started))
	if selected["loot"] {
		if err := preparePVFLoot(&result, source, inputs); err != nil {
			return &result, err
		}
	}
	if selected["boxes"] || selected["fame"] || selected["roster-backgrounds"] || selected["season"] || selected["adventure"] || selected["lottery"] || selected["selection-boxes"] || selected["bleeding-mine"] || selected["black-purgatory"] || selected["clear-cube"] || selected["odyssey-growth"] || selected["odyssey-weapons"] || selected["odyssey-drop"] || selected["odyssey-currency"] || selected["items"] || selected["equipment"] || selected["prices"] || selected["materials"] || selected["boosters"] || selected["enhancements"] || selected["shields"] || selected["equipment-selection"] {

		var direct catalog.ItemIndex
		var e error
		if result.ItemBasics != nil {
			direct = result.ItemBasics.Index
		} else {
			direct, e = source.ItemIndex("")
		}
		if e != nil {
			return &result, e
		}
		result.Items = &direct
		log.Printf("PVF item index prepared: %d templates source=%s", len(direct.Items), direct.Source.Checksum)
		if selected["loot"] {
			result.EmblemCompound, e = source.EmblemCompound(direct)
			if e != nil {
				return &result, fmt.Errorf("PVF emblem compound: %w", e)
			}
			log.Printf("PVF emblem compound prepared: %d combinations, %d emblem grades source=%s", len(result.EmblemCompound.Rolls), len(result.EmblemCompound.Pools), result.EmblemCompound.Source)
		}
		if selected["equipment"] && selected["loot"] {
			result.AvatarDisjoint, e = source.AvatarDisjoint(direct)
			if e != nil {
				return &result, fmt.Errorf("PVF avatar disjoint: %w", e)
			}
			log.Printf("PVF avatar disjoint prepared: %d avatar grades, %d emblem grades source=%s", len(result.AvatarDisjoint.Rolls), len(result.AvatarDisjoint.Pools), result.AvatarDisjoint.Source)
			result.EmblemInlay, e = source.EmblemInlay(direct)
			if e != nil {
				return &result, fmt.Errorf("PVF avatar emblem rules: %w", e)
			}
			log.Printf("PVF avatar emblem rules prepared: %d templates source=%s", len(result.EmblemInlay.Masks), result.EmblemInlay.Source)
			result.AvatarSockets, e = source.AvatarSockets(direct)
			if e != nil {
				return &result, fmt.Errorf("PVF avatar sockets: %w", e)
			}
			log.Printf("PVF avatar socket devices prepared: %d templates source=%s", len(result.AvatarSockets.Devices), result.AvatarSockets.Source)
			result.AvatarRecast, e = source.AvatarRecast(direct)
			if e != nil {
				return &result, fmt.Errorf("PVF avatar recast: %w", e)
			}
			log.Printf("PVF avatar recast prepared: %d jobs source=%s", len(result.AvatarRecast.Jobs), result.AvatarRecast.Source)
		}
		source.ReleaseReadCaches()
		if e := preparePVFCommerce(&result, source, selected, inputs); e != nil {
			return &result, e
		}
		logPVFMemory("item-commerce", time.Since(started))

		if selected["enhancements"] {
			if err := preparePVFEnhancements(&result, source, inputs); err != nil {
				return &result, err
			}
		}
		if selected["shields"] {
			if err := preparePVFShields(&result, source, characters, inputs); err != nil {
				return &result, err
			}
		}
		if selected["equipment-selection"] {
			if err := preparePVFEquipmentSelection(&result, source, inputs); err != nil {
				return &result, err
			}
		}
		if selected["equipment"] {
			candidate, e := source.Equipment(direct)
			if e != nil {
				return &result, e
			}
			result.Equipment = candidate
			log.Printf("PVF lazy equipment prepared: %d compact source bindings; expanded chunks bounded to 64 MiB", candidate.RecordCount())
		}
	}
	if err := preparePVFScenes(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	logPVFMemory("scenes", time.Since(started))
	if err := preparePVFAdventure(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFRecommended(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFSeason(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFOdysseyRoutes(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFRosterBackgrounds(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFFame(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFBoostUp(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFEquipmentAwakening(&result, source); err != nil {
		return &result, err
	}
	if err := preparePVFSoleEquipment(&result, source); err != nil {
		return &result, err
	}
	if err := preparePVFChannels(&result, source); err != nil {
		return &result, err
	}
	if err := preparePVFItemShops(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFBoxes(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFSelectionBoxes(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFLottery(&result, source, selected, inputs, adapters); err != nil {
		return &result, err
	}
	if err := preparePVFLayerRevisits(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFScriptWarps(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFClosingScenes(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFTowers(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFHellMaps(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFMazeRates(&result, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFMine(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFBlackPurgatory(&result, source, selected, inputs, adapters); err != nil {
		return &result, err
	}
	if err := preparePVFClearCube(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFOdyssey(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if err := preparePVFSpecial(&result, source, selected, inputs); err != nil {
		return &result, err
	}
	if result.Dungeons != nil {
		result.Dungeons.ReleaseMapReadCache()
	}
	if err := source.EnableRuntimeDetails(result.Quests, result.Learning, result.Loot, result.Items); err != nil {
		return &result, fmt.Errorf("PVF runtime details: %w", err)
	}
	if result.Equipment != nil || result.Dungeons != nil || result.Quests != nil || result.Learning != nil || result.Loot != nil {
		if err := source.CompactRuntimeStrings(); err != nil {
			return &result, fmt.Errorf("compact PVF runtime strings: %w", err)
		}
	}
	if err := result.validateSelectedProjections(); err != nil {
		return &result, err
	}
	result.prepared = make(map[string]bool, len(selected))
	for domain := range selected {
		result.prepared[domain] = true
	}
	log.Printf("PVF candidate catalogs prepared in %s; full directory can be collected before opening storage", time.Since(started))
	logPVFMemory("catalogs-prepared", time.Since(started))
	return &result, nil
}

func (c *Catalogs) LoadQuests(path string) (catalog.QuestCatalog, error) {
	if err := c.RequireSelected("quests", c.Quests != nil); err != nil {
		return catalog.QuestCatalog{}, err
	}
	if c.Quests != nil {
		return *c.Quests, nil
	}
	return catalog.QuestCatalog{}, fmt.Errorf("quests require the native PVF quests domain")
}

func (c *Catalogs) LoadProgression(path string) (catalog.Progression, error) {
	if err := c.RequireSelected("progression", c.Progression != nil); err != nil {
		return catalog.Progression{}, err
	}
	if c.Progression != nil {
		return *c.Progression, nil
	}
	return catalog.Progression{}, fmt.Errorf("progression requires the native PVF progression domain")
}

func (c *Catalogs) CollectImportMemory() {
	if c.Town != nil || c.Dungeons != nil || c.TrainingDungeons != nil || c.TutorialDungeons != nil || c.Quests != nil || c.Progression != nil || c.World != nil || c.Items != nil || c.Periods != nil || c.Skins != nil || c.Journal != nil || c.CreateCost != nil || c.Learning != nil || c.Tutorial != nil {
		runtime.GC()
		logPVFMemory("import-collected", 0)
	}
}

func (c *Catalogs) OpenFullEquipment(prefix, checksum string) (*inventory.FullEquipmentCatalog, error) {
	if c.Equipment != nil {
		if c.Equipment.Source.Checksum != checksum {
			return nil, fmt.Errorf("prepared equipment source mismatch")
		}
		return c.Equipment, nil
	}
	return nil, fmt.Errorf("full equipment requires the native PVF equipment domain")
}

func (c *Catalogs) SupplementStackables(loot *catalog.LootCatalog, path string) error {
	if c.Items != nil {
		return loot.SupplementItemIndex(*c.Items)
	}
	return fmt.Errorf("item supplementation requires the native PVF items domain")
}

func (c *Catalogs) LoadBooster(path, indexPath string) (*catalog.BoosterCatalog, error) {
	if err := c.RequireSelected("boosters", c.Prepared("boosters") || c.Boosters != nil); err != nil {
		return nil, err
	}
	if c.Boosters != nil || c.Prepared("boosters") {
		var items map[uint32]catalog.ItemIndexEntry
		if c.Items != nil {
			items = c.Items.Items
		}
		return &catalog.BoosterCatalog{Definitions: c.Boosters, Items: items}, nil
	}
	return nil, fmt.Errorf("booster definitions require the native PVF boosters domain")
}

func (c *Catalogs) LoadWorld(path string) (catalog.WorldCatalog, error) {
	if err := c.RequireSelected("world", c.World != nil); err != nil {
		return catalog.WorldCatalog{}, err
	}
	if c.World != nil {
		return *c.World, nil
	}
	return catalog.WorldCatalog{}, fmt.Errorf("world requires the native PVF world domain")
}

func (c *Catalogs) WriteHeapProfile(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	err = pprof.WriteHeapProfile(f)
	closeErr := f.Close()
	runtime.KeepAlive(c)
	if err != nil {
		return err
	}
	return closeErr
}

// checkReport is called only after source preparation and before installing
// runtime globals, opening storage, creating capture files or listeners.
func (c *Catalogs) CheckReport(selection string) (map[string]any, error) {
	domains, err := parsePVFCatalogSelection(selection)
	if err != nil {
		return nil, err
	}
	checksum, err := hex.DecodeString(c.SourceChecksum)
	if err != nil || len(checksum) != 32 || len(domains) == 0 {
		return nil, fmt.Errorf("no prepared PVF source for catalog check")
	}
	ordered := make([]string, 0, len(domains))
	for _, domain := range strings.Split(SupportedDomains, ",") {
		if domains[domain] {
			ordered = append(ordered, domain)
		}
	}
	r := map[string]any{"source": c.SourceChecksum, "domain_count": len(domains), "domains": ordered, "storage_accessed": false, "runtime_started": false}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	r["memory"] = map[string]any{"heap_alloc_bytes": memory.HeapAlloc, "heap_inuse_bytes": memory.HeapInuse, "heap_sys_bytes": memory.HeapSys, "total_alloc_bytes": memory.TotalAlloc, "gc_count": memory.NumGC}
	if c.Characters != nil {
		r["professions"] = len(c.Characters.Professions)
	}
	if c.Quests != nil {
		r["quests"] = len(c.Quests.Quests)
	}
	if c.Items != nil {
		r["items"] = len(c.Items.Items)
	}
	if c.BoostUp != nil {
		r["boost_steps"] = len(c.BoostUp.Steps)
		r["boost_gifts"] = len(c.BoostUp.Gifts)
		r["boost_capsules"] = len(c.BoostUp.Capsules)
	}
	if c.Equipment != nil {
		r["equipment_bindings"] = c.Equipment.RecordCount()
	}
	if c.Selection != nil {
		r["equipment_selection"] = len(c.Selection.Rows)
	}
	if c.Dungeons != nil {
		r["dungeons"] = len(c.Dungeons.Dungeons)
	}
	if c.CashShop != nil {
		r["cashshop_products"] = c.CashShop.EnabledCount()
	}
	if c.Boxes != nil {
		r["boxes"] = c.Boxes.TableCount()
	}
	if c.ItemShops != nil {
		r["item_shops"] = len(c.ItemShops.Shops)
	}
	return r, nil
}

func logPVFMemory(stage string, elapsed time.Duration) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("PVF memory stage=%s elapsed=%s heap=%.1fMiB inuse=%.1fMiB reserved=%.1fMiB total_alloc=%.1fMiB gc=%d", stage, elapsed, float64(m.HeapAlloc)/(1<<20), float64(m.HeapInuse)/(1<<20), float64(m.HeapSys)/(1<<20), float64(m.TotalAlloc)/(1<<20), m.NumGC)
}