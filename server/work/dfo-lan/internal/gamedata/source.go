// Package gamedata selects the source of static game catalogs. It never opens
// storage or rewrites source versions in player saves.
package gamedata

import (
	"dfolan/internal/adventure"
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/rosterbg"
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"
	"strings"
)

type Mode string

const (
	JSON            Mode  = "json"
	PVF             Mode  = "pvf"
	DefaultMaxBytes int64 = 1024 * 1024 * 1024
)

type Options struct {
	Mode        Mode
	ArchivePath string
	// ExpectedChecksum 为空 = 自动派生：信任内层归档自身算出的 SHA256。
	// 非空 = 显式校验（发布 / 审计场景钉死某一版）。
	// 为什么允许为空：内层 PVF 是本地按需生成的产物（见 scripts/ensure_inner_pvf.py），
	// 手写常量会与文件脱钩 —— 自愈更新了文件、常量没更新就启动失败（next142 的事故）。
	// 自动派生不额外读一遍归档：OpenReadOnly完整流式计算一次SHA256并复用它。
	ExpectedChecksum string
	MaxBytes         int64
	// Empty or "-" disables derived files. The archive is verified even on hits.
	DerivedCacheDir string
}

// Source owns one read-only archive shared by sequential catalog imports.
type Source struct {
	mode       Mode
	archive    *pvf.Archive
	cacheDir   string
	cacheStats derivedCacheCounters
	cacheFiles map[string]bool
}

func (s *Source) VisitItemDisplay(index catalog.ItemIndex, visit func(catalog.ItemDisplay) error) error {
	return catalog.VisitItemDisplay(s.archive, index, visit)
}

func (s *Source) ItemShops(policy catalog.ItemShopSourcePolicy) (catalog.NativeItemShops, error) {
	return catalog.ImportItemShops(s.archive, policy)
}

func (s *Source) Boxes(index catalog.ItemIndex, policy loot.BoxSourcePolicy) (*loot.BoxCatalog, error) {
	return loot.ImportBoxes(s.archive, index, policy)
}

func (s *Source) AvatarDisjoint(index catalog.ItemIndex) (*inventory.AvatarDisjointRules, error) {
	return inventory.ImportAvatarDisjointRules(s.archive, index)
}

func (s *Source) AvatarRecast(index catalog.ItemIndex) (*inventory.AvatarRecastRules, error) {
	return inventory.ImportAvatarRecastRules(s.archive, index)
}

func (s *Source) CashShop() (cashshop.PilotConfig, error) {
	return cashshop.ImportPilot(s.archive)
}

func (s *Source) Adventure(index catalog.ItemIndex) (*adventure.Rules, error) {
	return adventure.ImportRules(s.archive, index)
}

func (s *Source) RecommendedDungeons() (*adventure.RecommendedRules, error) {
	return adventure.ImportRecommendedRules(s.archive)
}

func (s *Source) Season(index catalog.ItemIndex) (*adventure.SeasonRules, error) {
	return cachedProjection(s, "season", itemIndexIdentity(index), func() (*adventure.SeasonRules, error) { return adventure.ImportSeasonRules(s.archive, index) }, func(r *adventure.SeasonRules) (*adventure.SeasonRules, error) {
		if r == nil || r.SourceChecksum != s.Snapshot().Checksum {
			return nil, fmt.Errorf("season cache source mismatch")
		}
		return adventure.NewSeasonRules(*r)
	})
}

func (s *Source) OdysseyJournalRoutes() (*catalog.OdysseyJournalRoutes, error) {
	return catalog.ImportOdysseyJournalRoutes(s.archive)
}

func (s *Source) RosterBackgrounds(index catalog.ItemIndex) (*rosterbg.TicketCatalog, error) {
	return cachedProjection(s, "roster", itemIndexIdentity(index), func() (*rosterbg.TicketCatalog, error) {
		return catalog.ImportRosterBackgroundTickets(s.archive, index)
	}, func(r *rosterbg.TicketCatalog) (*rosterbg.TicketCatalog, error) {
		if r == nil || r.Source != s.Snapshot().Checksum {
			return nil, fmt.Errorf("background cache source mismatch")
		}
		return rosterbg.NewTicketCatalog(*r)
	})
}

func (s *Source) Fame(index catalog.ItemIndex) (*character.FameRules, error) {
	return character.ImportFameRules(s.archive, index)
}

func (s *Source) Lottery(index catalog.ItemIndex, policy catalog.LotteryPolicy) (catalog.LotteryTables, error) {
	return catalog.ImportLotteryTables(s.archive, index, policy)
}

func (s *Source) DiscoverLottery(index catalog.ItemIndex) (catalog.LotteryTables, catalog.LotteryScope, error) {
	return catalog.DiscoverLotteryTables(s.archive, index)
}

func (s *Source) SelectionBoxes(index catalog.ItemIndex, policy catalog.SelectionBoxPolicy) (*catalog.SelectionBoxes, error) {
	return catalog.ImportSelectionBoxes(s.archive, index, policy)
}

func (s *Source) AuditSelectionScope(index catalog.ItemIndex, issueLimit int) (catalog.SelectionScopeAudit, error) {
	return catalog.AuditSelectionScope(s.archive, index, issueLimit)
}

func (s *Source) TerminalScenes(d catalog.DungeonCatalog, q catalog.QuestCatalog) (catalog.TerminalSceneOverlay, error) {
	return cachedProjection(s, "terminal", struct {
		D catalog.DungeonCatalog
		Q catalog.QuestCatalog
	}{stableDungeonInput(d), stableQuestInput(q)}, func() (catalog.TerminalSceneOverlay, error) { return catalog.ImportTerminalScenes(s.archive, d, q) }, func(r catalog.TerminalSceneOverlay) (catalog.TerminalSceneOverlay, error) {
		copy := d
		if err := catalog.ApplyTerminalScenes(&copy, r); err != nil {
			return r, err
		}
		r.Source.Checksum = s.Snapshot().Checksum
		return r, nil
	})
}

func (s *Source) TournamentQuestMaps(d catalog.DungeonCatalog) (catalog.SourceMapOverlay, error) {
	return catalog.ImportTournamentQuestMaps(s.archive, d)
}

func Open(options Options) (*Source, error) {
	if options.Mode == "" {
		options.Mode = JSON
	}
	if options.Mode != JSON && options.Mode != PVF {
		return nil, fmt.Errorf("unknown catalog source %q; expected json or pvf", options.Mode)
	}
	s := &Source{mode: options.Mode, cacheDir: options.DerivedCacheDir}
	if options.Mode == JSON {
		return s, nil
	}
	if strings.TrimSpace(options.ArchivePath) == "" {
		return nil, fmt.Errorf("PVF source requires an explicit inner archive path")
	}
	// 空 ExpectedChecksum = 自动派生（见 Options 注释）。显式给出时必须是一个
	// 合法的 32 字节 SHA256，避免把拼错的常量静默当作"没给"。
	expected := strings.ToLower(strings.TrimSpace(options.ExpectedChecksum))
	if expected != "" {
		decoded, err := hex.DecodeString(expected)
		if err != nil || len(decoded) != 32 {
			return nil, fmt.Errorf("PVF source requires the expected SHA256 of the inner archive")
		}
	}
	path, err := filepath.Abs(options.ArchivePath)
	if err != nil {
		return nil, err
	}
	if options.MaxBytes == 0 {
		options.MaxBytes = DefaultMaxBytes
	}
	parser := ""
	if options.DerivedCacheDir != "" && options.DerivedCacheDir != "-" {
		parser, err = derivedParserIdentity()
		if err != nil {
			log.Printf("PVF metadata cache unavailable; native parse: %v", err)
		}
	}
	a, err := pvf.OpenReadOnlyCached(pvf.Options{Path: path, MaxBytes: options.MaxBytes}, expected, options.DerivedCacheDir, parser)
	if err != nil {
		return nil, fmt.Errorf("open inner PVF: %w", err)
	}
	s.archive = a
	return s, nil
}

func (s *Source) Close() error {
	if s == nil || s.archive == nil {
		return nil
	}
	return s.archive.Close()
}

func (s *Source) CompactRuntimeStrings() error {
	if s == nil || s.archive == nil {
		return nil
	}
	if err := s.archive.CompactRuntimeStrings(); err != nil {
		return err
	}
	s.pruneDerivedCaches()
	return nil
}

func (s *Source) EnableRuntimeDetails(q *catalog.QuestCatalog, l *character.LearningCatalog, items *catalog.LootCatalog, index *catalog.ItemIndex) (err error) {
	defer func() {
		if err != nil {
			if q != nil {
				q.Close()
			}
			if l != nil {
				l.Close()
			}
			if items != nil {
				items.CloseDetails()
			}
		}
	}()
	if q != nil {
		if err := q.EnableRuntimeDetails(s.archive); err != nil {
			return err
		}
	}
	if l != nil {
		if err := l.EnableRuntimeDetails(s.archive); err != nil {
			return err
		}
	}
	if items != nil && index != nil {
		if err := items.EnableRuntimeDetails(s.archive, *index); err != nil {
			return err
		}
	}
	return nil
}

func (s *Source) Snapshot() pvf.ArchiveSnapshot {
	if s.archive == nil {
		return pvf.ArchiveSnapshot{}
	}
	return s.archive.Snapshot()
}

// ReleaseReadCaches may be called between import stages. Imported catalogs own
// their decoded values; clearing temporary reads does not alter those catalogs.
func (s *Source) ReleaseReadCaches() {
	if s.archive != nil {
		s.archive.ReleaseReadCaches()
	}
}

func (s *Source) Characters(path string) (catalog.Characters, error) {
	if s.mode == JSON {
		return catalog.LoadCharacters(path)
	}
	return catalog.ImportCharacters(s.archive)
}

func (s *Source) World(path string) (catalog.WorldCatalog, error) {
	if s.mode == JSON {
		return catalog.LoadWorld(path)
	}
	return catalog.ImportWorldRuntime(s.archive)
}

func (s *Source) Quests(path string) (catalog.QuestCatalog, error) {
	if s.mode == JSON {
		return catalog.LoadQuests(path)
	}
	return catalog.ImportQuests(s.archive)
}

func (s *Source) Progression(path string) (catalog.Progression, error) {
	if s.mode == JSON {
		return catalog.LoadProgression(path)
	}
	return catalog.ImportProgression(s.archive)
}

func (s *Source) ItemIndex(path string) (catalog.ItemIndex, error) {
	if s.mode == JSON {
		return catalog.LoadItemIndex(path)
	}
	return catalog.ImportItemIndex(s.archive)
}

func (s *Source) ItemBasics(options catalog.ItemBasicOptions) (catalog.ItemBasics, error) {
	if s.mode != PVF {
		return catalog.ItemBasics{}, fmt.Errorf("joint item import requires PVF")
	}
	return catalog.ImportItemBasics(s.archive, options)
}

type JointItemCatalogs struct {
	Basics       catalog.ItemBasics
	Enhancements *inventory.EnhancementCatalog
	Fame         *character.FameRules
}

// ItemCatalogs streams common script bytes into selected domain consumers.
// It publishes nothing globally and returns no partial catalog on failure.
func (s *Source) ItemCatalogs(options catalog.ItemBasicOptions, enhancements bool, policyPath string, fame bool) (JointItemCatalogs, error) {
	return s.cachedItemCatalogs(options, enhancements, policyPath, fame)
}

func (s *Source) importItemCatalogs(options catalog.ItemBasicOptions, enhancements bool, policyPath string, fame bool) (JointItemCatalogs, error) {
	var out JointItemCatalogs
	if s.mode != PVF {
		return out, fmt.Errorf("joint item import requires PVF")
	}
	var enhancementImport *inventory.EnhancementItemImport
	var fameImport *character.FameItemImport
	var err error
	if enhancements {
		enhancementImport, err = inventory.NewEnhancementItemImport(s.archive, policyPath)
		if err != nil {
			return out, err
		}
		options.Consumers = append(options.Consumers, enhancementImport.Consume)
	}
	if fame {
		fameImport, err = character.NewFameItemImport(s.archive)
		if err != nil {
			return out, err
		}
		options.Consumers = append(options.Consumers, fameImport.Consume)
	}
	out.Basics, err = s.ItemBasics(options)
	if err != nil {
		return JointItemCatalogs{}, err
	}
	if enhancementImport != nil {
		out.Enhancements, err = enhancementImport.Finish(out.Basics.Index)
		if err != nil {
			return JointItemCatalogs{}, err
		}
	}
	if fameImport != nil {
		out.Fame, err = fameImport.Finish()
		if err != nil {
			return JointItemCatalogs{}, err
		}
	}
	return out, nil
}

func (s *Source) Equipment(index catalog.ItemIndex) (*inventory.FullEquipmentCatalog, error) {
	if s.mode != PVF {
		return nil, fmt.Errorf("lazy PVF equipment requires a PVF source")
	}
	p, err := cachedProjection(s, "equipment", itemIndexIdentity(index), func() (inventory.PVFEquipmentProjection, error) {
		return inventory.ProjectPVFEquipment(s.archive, index)
	}, func(p inventory.PVFEquipmentProjection) (inventory.PVFEquipmentProjection, error) {
		return p, inventory.ValidatePVFEquipmentProjection(s.archive, p)
	})
	if err != nil {
		return nil, err
	}
	return inventory.RestorePVFEquipment(s.archive, p)
}

func (s *Source) Learning(c catalog.Characters) (*character.LearningCatalog, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("learning import requires PVF")
	}
	return character.ImportLearningCatalog(s.archive, c)
}

func (s *Source) ShopPrices(index catalog.ItemIndex) (*catalog.ShopPrices, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("price import requires PVF")
	}
	return catalog.ImportShopPrices(s.archive, index)
}

func (s *Source) ItemMaterials(index catalog.ItemIndex) (*catalog.ItemMaterials, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("material import requires PVF")
	}
	return catalog.ImportItemMaterials(s.archive, index)
}

func (s *Source) Boosters(index catalog.ItemIndex) (map[uint32]catalog.BoosterDefinition, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("booster import requires PVF")
	}
	return catalog.ImportBoosters(s.archive, index)
}

func (s *Source) ItemPeriods() (catalog.ItemPeriodCatalog, error) {
	if s.archive == nil {
		return catalog.ItemPeriodCatalog{}, fmt.Errorf("item periods import requires PVF")
	}
	return catalog.ImportItemPeriods(s.archive)
}

func (s *Source) SkinStorage() (catalog.SkinStorageCatalog, error) {
	if s.archive == nil {
		return catalog.SkinStorageCatalog{}, fmt.Errorf("skin storage import requires PVF")
	}
	return catalog.ImportSkinStorage(s.archive)
}

func (s *Source) EquipmentJournal() (catalog.EquipmentJournalRules, error) {
	if s.archive == nil {
		return catalog.EquipmentJournalRules{}, fmt.Errorf("journal import requires PVF")
	}
	return catalog.ImportEquipmentJournalRules(s.archive)
}

func (s *Source) EquipmentCreateCost() (catalog.EquipmentCreateCost, error) {
	if s.archive == nil {
		return catalog.EquipmentCreateCost{}, fmt.Errorf("create cost import requires PVF")
	}
	return catalog.ImportEquipmentCreateCost(s.archive)
}

// EquipmentAwakening 直读装备调适规则（CMD2258）。
// 源 = etc/115lvability/equipmentawakeningoptionsystem.cos，不经过任何导出 JSON。
func (s *Source) EquipmentAwakening() (*catalog.EquipmentAwakeningRules, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("equipment awakening import requires PVF")
	}
	rules, err := catalog.ImportEquipmentAwakeningRules(s.archive)
	if err != nil {
		return nil, err
	}
	return &rules, nil
}

// EquipmentAwakeningOptions 直读调适选项索引表（`[equipment awakening option]` 的 ID → 加成表）。
func (s *Source) EquipmentAwakeningOptions() (*catalog.EquipmentAwakeningOptions, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("equipment awakening options import requires PVF")
	}
	options, err := catalog.ImportEquipmentAwakeningOptions(s.archive)
	if err != nil {
		return nil, err
	}
	return &options, nil
}

func (s *Source) Tutorials() (catalog.TutorialCatalog, error) {
	if s.archive == nil {
		return catalog.TutorialCatalog{}, fmt.Errorf("tutorial import requires PVF")
	}
	return catalog.ImportTutorials(s.archive)
}

func (s *Source) Enhancements(index catalog.ItemIndex, policyPath string) (*inventory.EnhancementCatalog, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("enhancement import requires PVF")
	}
	return inventory.ImportEnhancements(s.archive, index, policyPath)
}

func (s *Source) RandomOptions() (inventory.RandomOptionData, error) {
	if s.archive == nil {
		return inventory.RandomOptionData{}, fmt.Errorf("random options require PVF")
	}
	return inventory.ImportRandomOptionData(s.archive)
}

func (s *Source) KnightShields(index catalog.ItemIndex, jobs catalog.Characters, rules inventory.WearRules) (*inventory.KnightShields, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("knight shields require PVF")
	}
	return inventory.ImportKnightShields(s.archive, index, jobs, rules)
}

func (s *Source) OathGrades() (*inventory.OathGradeTable, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("oath grades require PVF")
	}
	return inventory.ImportOathGrades(s.archive)
}

func (s *Source) VaultRules(policyPath string) (inventory.VaultRules, error) {
	if s.archive == nil {
		return inventory.VaultRules{}, fmt.Errorf("vault rules require PVF")
	}
	return inventory.ImportVaultRules(s.archive, policyPath)
}

func (s *Source) Loot(maximumGrade uint32) (catalog.LootCatalog, error) {
	if s.archive == nil {
		return catalog.LootCatalog{}, fmt.Errorf("loot requires PVF")
	}
	return cachedProjection(s, "loot", maximumGrade, func() (catalog.LootCatalog, error) {
		direct, err := catalog.ImportLoot(s.archive, maximumGrade)
		if err != nil {
			return direct, err
		}
		return catalog.ValidateLoot(direct)
	}, func(r catalog.LootCatalog) (catalog.LootCatalog, error) {
		if r.Source.Checksum != s.Snapshot().Checksum || r.MaximumGrade != maximumGrade {
			return r, fmt.Errorf("loot cache source mismatch")
		}
		r.Source = s.Snapshot()
		// ValidateLoot appends diagnostics; keep serialized diagnostics unchanged.
		checked := r
		checked.Skipped = append([]string(nil), r.Skipped...)
		_, err := catalog.ValidateLoot(checked)
		return r, err
	})
}

func (s *Source) EquipmentSelection(index catalog.ItemIndex, quests catalog.QuestCatalog, policy inventory.DropPolicy) (*inventory.EquipmentCatalog, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("equipment selection requires PVF")
	}
	return inventory.ImportEquipmentSelection(s.archive, index, quests, policy)
}

func (s *Source) Town(town, area uint32) (catalog.TownArea, error) {
	if s.archive == nil {
		return catalog.TownArea{}, fmt.Errorf("town import requires PVF")
	}
	return catalog.ImportTownArea(s.archive, town, area)
}

func (s *Source) FullDungeons(world catalog.WorldCatalog, excluded []uint32) (catalog.DungeonCatalog, error) {
	if s.archive == nil {
		return catalog.DungeonCatalog{}, fmt.Errorf("dungeon import requires PVF")
	}
	return catalog.ImportFullDungeons(s.archive, world, excluded)
}

func (s *Source) RuntimeFullDungeons(world catalog.WorldCatalog, excluded []uint32) (catalog.DungeonCatalog, error) {
	if s.archive == nil {
		return catalog.DungeonCatalog{}, fmt.Errorf("dungeon import requires PVF")
	}
	return cachedProjection(s, "dungeons", struct {
		World    catalog.WorldCatalog
		Excluded []uint32
	}{stableWorldInput(world), excluded}, func() (catalog.DungeonCatalog, error) {
		return catalog.ImportRuntimeFullDungeons(s.archive, world, excluded)
	}, func(r catalog.DungeonCatalog) (catalog.DungeonCatalog, error) {
		if r.Source.Checksum != s.Snapshot().Checksum {
			return r, fmt.Errorf("dungeon cache source mismatch")
		}
		r.Source = s.Snapshot()
		return catalog.RestoreRuntimeDungeons(s.archive, r)
	})
}

func (s *Source) Dungeons(ids []uint32) (catalog.DungeonCatalog, error) {
	if s.archive == nil {
		return catalog.DungeonCatalog{}, fmt.Errorf("dungeon import requires PVF")
	}
	c, err := catalog.ImportDungeons(s.archive, ids)
	if err != nil {
		return c, err
	}
	return catalog.ValidateDungeons(c)
}

func (s *Source) TowerGrief() (catalog.TowerGriefOverlay, error) {
	if s.archive == nil {
		return catalog.TowerGriefOverlay{}, fmt.Errorf("tower import requires PVF")
	}
	return catalog.ImportTowerGriefOverlay(s.archive)
}

func (s *Source) TowerDazzlement() (catalog.DazzlementOverlay, error) {
	if s.archive == nil {
		return catalog.DazzlementOverlay{}, fmt.Errorf("tower import requires PVF")
	}
	return catalog.ImportDazzlementOverlay(s.archive)
}

func (s *Source) HellPartyMaps(c catalog.DungeonCatalog) (catalog.SourceMapOverlay, []uint32, error) {
	if s.archive == nil {
		return catalog.SourceMapOverlay{}, nil, fmt.Errorf("Hell Party maps require PVF")
	}
	return catalog.ImportHellPartyMaps(s.archive, c)
}

func (s *Source) Apocalypse() (*catalog.ApocalypseCatalog, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("apocalypse import requires PVF")
	}
	return catalog.ImportApocalypse(s.archive)
}

// Attunement 的副本范围由源决定：etc/rewardboostinfo/**.ctp 各自声明 [dungeon index]。
// 不再接受外部清单（单一内容真源铁律，server/AGENTS.md §0）。
func (s *Source) Attunement() (*loot.AttunementRewards, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("attunement import requires PVF")
	}
	return loot.ImportAttunementRewards(s.archive)
}

func (s *Source) OdysseyGrowth(index catalog.ItemIndex, supplemental []uint32) (*catalog.OdysseyGrowth, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("Odyssey requires PVF")
	}
	return catalog.ImportOdysseyGrowth(s.archive, index, supplemental)
}
func (s *Source) OdysseyChapters() (*catalog.OdysseyChapters, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("Odyssey requires PVF")
	}
	return catalog.ImportOdysseyChapters(s.archive)
}
func (s *Source) OdysseyWeapons(index catalog.ItemIndex) (catalog.OdysseyWeaponChoices, error) {
	if s.archive == nil {
		return catalog.OdysseyWeaponChoices{}, fmt.Errorf("Odyssey requires PVF")
	}
	return catalog.ImportOdysseyWeaponChoices(s.archive, index)
}
func (s *Source) OdysseyCurrency(index catalog.ItemIndex, policy loot.OdysseyCurrencyPolicy) (*loot.OdysseyCurrency, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("Odyssey requires PVF")
	}
	return loot.ImportOdysseyCurrency(s.archive, index, policy)
}

func (s *Source) ClearCube(index catalog.ItemIndex) (catalog.LootItem, error) {
	if s.archive == nil {
		return catalog.LootItem{}, fmt.Errorf("clear cube requires PVF")
	}
	return catalog.ImportClearCube(s.archive, index)
}

func (s *Source) BlackPurgatory(index catalog.ItemIndex, policy loot.BlackPurgatoryPolicy) (*loot.BlackPurgatoryRewards, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("Black Purgatory requires PVF")
	}
	return loot.ReadBlackPurgatoryRewards(s.archive, index, policy)
}

func (s *Source) BleedingMine(index catalog.ItemIndex, policy loot.BleedingMinePolicy) (*loot.BleedingMineRewards, error) {
	if s.archive == nil {
		return nil, fmt.Errorf("mine requires PVF")
	}
	return loot.ImportBleedingMineRewards(s.archive, index, policy)
}

func (s *Source) ScriptWarpRoutes(d catalog.DungeonCatalog, p catalog.ScriptWarpPolicy) ([]catalog.ScriptWarpRoute, error) {
	return cachedProjection(s, "warps", struct {
		D catalog.DungeonCatalog
		P catalog.ScriptWarpPolicy
	}{stableDungeonInput(d), p}, func() ([]catalog.ScriptWarpRoute, error) { return catalog.ImportScriptWarpRoutes(s.archive, d, p) }, func(r []catalog.ScriptWarpRoute) ([]catalog.ScriptWarpRoute, error) {
		if len(r) != len(p.Routes) {
			return nil, fmt.Errorf("warp cache count mismatch")
		}
		for _, route := range r {
			if route.Source != s.Snapshot().Checksum || route.DungeonSHA256 != d.Dungeons[route.Dungeon].Script.SHA256 || route.MapSHA256 != d.Maps[route.From].SHA256 || len(route.ActionSHA256) != 64 {
				return nil, fmt.Errorf("warp cache source mismatch")
			}
		}
		return r, nil
	})
}

func (s *Source) LayerRevisits(d catalog.DungeonCatalog, p catalog.LayerRevisitPolicy) (catalog.LayerRevisitOverlay, error) {
	return catalog.ImportLayerRevisits(s.archive, d, p)
}
