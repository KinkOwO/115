package gamedata

import (
	"dfolan/internal/adventure"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

func preparePVFAdventure(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["adventure"] {
		return nil
	}
	if c.Items == nil {
		return fmt.Errorf("native adventure requires native item index")
	}
	direct, err := s.Adventure(*c.Items)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := adventure.EmbeddedRules()
		if err != nil {
			return err
		}
		if err := auditAdventureRules(old, direct); err != nil {
			return fmt.Errorf("adventure: %w", err)
		}
	}
	c.AdventureRules = direct
	s.ReleaseReadCaches()
	log.Printf("PVF adventure prepared: levels=%d shops=%d items=%d; limits, prices, experience and reset rules retained", len(direct.Experience), len(direct.Shops), len(direct.Items))
	return nil
}

// The embedded export named its old outer file. Normalize only that known
// provenance for a complete audit; every rule and raw hash still must match.
// Native runtime metadata and character/save identities are never rewritten.
func auditAdventureRules(old, direct *adventure.Rules) error {
	if old == nil || direct == nil || len(direct.SourceChecksum) != 64 {
		return fmt.Errorf("missing adventure source identity")
	}
	baseline := *old
	if baseline.SourceChecksum != direct.SourceChecksum {
		if baseline.SourceChecksum != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.SourceChecksum != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80" {
			return fmt.Errorf("adventure baseline has unknown source provenance")
		}
		baseline.SourceChecksum = direct.SourceChecksum
	}
	return auditPVFCatalog(&baseline, direct)
}

func (c *Catalogs) InstallAdventureRules() (func(), error) {
	if err := c.RequireSelected("adventure", c.AdventureRules != nil); err != nil {
		return nil, err
	}
	if c.AdventureRules == nil {
		return func() {}, nil
	}
	return adventure.InstallRules(c.AdventureRules)
}

// The legacy client hash is the exporter provenance, not a character save
// version. Normalize only this historical metadata field, after exact inner
// identity and all three native script hashes have been checked. No archive
// checksum alias is introduced for runtime or player saves.
func auditBlackPurgatory(legacy, direct loot.BlackPurgatoryRewards) error {
	if legacy.Source != direct.Source || direct.Source != catalog.OdysseySource || legacy.ScriptHash != direct.ScriptHash || legacy.Boss.GroupHash != direct.Boss.GroupHash || legacy.Boss.RoutingHash != direct.Boss.RoutingHash {
		return fmt.Errorf("Black Purgatory baseline source/definition mismatch")
	}
	if legacy.ClientSource != direct.ClientSource {
		if legacy.ClientSource != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.ClientSource != direct.Source {
			return fmt.Errorf("unknown Black Purgatory export provenance")
		}
		legacy.ClientSource = direct.ClientSource
	}
	return auditPVFCatalog(legacy, direct)
}

func blackPurgatoryLookup(index *catalog.ItemIndex) func(uint32) (catalog.LootItem, bool) {
	return func(id uint32) (catalog.LootItem, bool) {
		entry, ok := index.Items[id]
		return catalog.LootItem{ID: id, Kind: entry.Kind, StackableType: entry.StackableType, StackLimit: entry.StackLimit, Script: catalog.ScriptRecord{Path: entry.Path}}, ok
	}
}

func preparePVFBlackPurgatory(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs, adapters CatalogAdapters) error {
	if !selected["black-purgatory"] {
		return nil
	}
	policy, err := readPVFContentPolicy(i.ContentPolicyPath)
	if err != nil {
		return err
	}
	direct, err := s.BlackPurgatory(*c.Items, policy.BlackPurgatory)
	if err != nil {
		return err
	}
	if c.Boosters == nil {
		c.Boosters, err = s.Boosters(*c.Items)
		if err != nil {
			return err
		}
	}
	if adapters.RewardBoxes == nil {
		return fmt.Errorf("PVF black-purgatory reward-box adapter is required")
	}
	boxes := adapters.RewardBoxes(c.Boosters, c.Items.Items)
	bound, err := loot.NewBlackPurgatoryRewards(*direct, boxes, blackPurgatoryLookup(c.Items))
	if err != nil {
		return err
	}
	c.BlackPurgatory = bound
	log.Printf("PVF Black Purgatory prepared: card branches=%d VIP source-only=%d equipment groups=%d rates=%v/%d; operator probabilities retained", len(direct.Cards), len(direct.VIPSourceOnly), len(direct.Boss.Groups), direct.Boss.Rates, direct.Boss.Denominator)
	s.ReleaseReadCaches()
	return nil
}

func (c *Catalogs) LoadBlackPurgatory(path string, boxes loot.RewardBoxSource, lookup func(uint32) (catalog.LootItem, bool)) (*loot.BlackPurgatoryRewards, error) {
	if err := c.RequireSelected("black-purgatory", c.BlackPurgatory != nil); err != nil {
		return nil, err
	}
	if c.BlackPurgatory != nil {
		return loot.NewBlackPurgatoryRewards(*c.BlackPurgatory, boxes, lookup)
	}
	return nil, nativeContentRequired("black-purgatory")
}

func preparePVFClearCube(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["clear-cube"] {
		return nil
	}
	direct, err := s.ClearCube(*c.Items)
	if err != nil {
		return err
	}
	base := catalog.LootCatalog{Source: s.Snapshot()}
	if _, err := inventory.WithClearCubeItem(base, direct); err != nil {
		return err
	}
	if i.checksBaselines() {
		path := pvfOdysseyBaseline(i, i.ClearCubePath, "DFO_CLEAR_CUBE_SOURCE", "clear-cube-source.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var legacy catalog.LootItem
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return err
		}
		if _, err := inventory.WithClearCubeItem(base, legacy); err != nil {
			return err
		}
		if err := verifyPVFCatalog(legacy, direct); err != nil {
			return fmt.Errorf("clear cube: %w", err)
		}
	}
	c.ClearCube = &direct
	log.Printf("PVF clear cube prepared: template=3037; storage overlay and random-pool exclusion retained")
	s.ReleaseReadCaches()
	return nil
}

func (c *Catalogs) WithClearCube(base catalog.LootCatalog, path string) (catalog.LootCatalog, error) {
	if err := c.RequireSelected("clear-cube", c.Prepared("clear-cube")); err != nil {
		return catalog.LootCatalog{}, err
	}
	if c.ClearCube != nil {
		return inventory.WithClearCubeItem(base, *c.ClearCube)
	}
	return catalog.LootCatalog{}, nativeContentRequired("clear-cube")
}

func preparePVFMine(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["bleeding-mine"] {
		return nil
	}
	policy, err := readPVFContentPolicy(i.ContentPolicyPath)
	if err != nil {
		return err
	}
	direct, err := s.BleedingMine(*c.Items, policy.BleedingMine)
	if err != nil {
		return err
	}
	c.Mine = direct
	log.Printf("PVF mine rewards prepared: stages=%d boss entries=%d difficulty rewards=%d containers=%d items=%d; signed empty faces and source composition retained", len(direct.StageBoxes), len(direct.BossBoxes), len(direct.GroupBoxes), len(direct.Boxes), len(direct.Items))
	s.ReleaseReadCaches()
	return nil
}

func (c *Catalogs) LoadMine(path string) (*loot.BleedingMineRewards, error) {
	if err := c.RequireSelected("bleeding-mine", c.Mine != nil); err != nil {
		return nil, err
	}
	if c.Mine != nil {
		return c.Mine, nil
	}
	return nil, nativeContentRequired("bleeding-mine")
}

func pvfOdysseyBaseline(i CatalogInputs, explicit, env, fallback string) string {
	if explicit != "" {
		return explicit
	}
	if path := os.Getenv(env); path != "" {
		return path
	}
	return filepath.Join(filepath.Dir(i.IndexPath), fallback)
}

func preparePVFOdyssey(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["odyssey-growth"] && !selected["odyssey-chapters"] && !selected["odyssey-weapons"] && !selected["odyssey-drop"] && !selected["odyssey-currency"] {
		return nil
	}
	var policy pvfContentPolicy
	if selected["odyssey-growth"] || selected["odyssey-chapters"] {
		// Explicit operator-requested mapping; native metadata and chapter
		// progress remain authoritative. Never silently fall back.
		rewards, err := catalog.LoadOdysseyCompletionRewards(filepath.Join(filepath.Dir(i.ContentPolicyPath), "odyssey-completion-rewards.json"))
		if err != nil {
			return err
		}
		if c.Items == nil {
			return fmt.Errorf("Odyssey completion item index missing")
		}
		if err = rewards.ValidateItems(*c.Items); err != nil {
			return err
		}
		c.OdysseyCompletionRewards = rewards
	}
	if selected["odyssey-drop"] || selected["odyssey-currency"] {
		var err error
		policy, err = readPVFContentPolicy(i.ContentPolicyPath)
		if err != nil {
			return err
		}
	}
	if selected["odyssey-growth"] {
		direct, err := s.OdysseyGrowth(*c.Items)
		if err != nil {
			return err
		}
		if i.checksBaselines() {
			legacy, err := catalog.LoadOdysseyGrowth(pvfOdysseyBaseline(i, i.OdysseyGrowthPath, "DFO_ODYSSEY_GROWTH", "odyssey-growth-release.json"))
			if err != nil {
				return err
			}
			if legacy.Source != direct.Source {
				return fmt.Errorf("Odyssey growth baseline source mismatch")
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("Odyssey growth: %w", err)
			}
		}
		c.OdysseyGrowth = direct
		log.Printf("PVF Odyssey growth prepared: clear=%d entry=%d gifts=%d graduate=%d; graduation quests retained", len(direct.ClearLevels), len(direct.EntryLevels), len(direct.Gifts), direct.GraduateReward)
	}
	var chapters *catalog.OdysseyChapters
	if selected["odyssey-chapters"] || selected["odyssey-drop"] {
		var err error
		chapters, err = s.OdysseyChapters()
		if err != nil {
			return err
		}
		if selected["odyssey-chapters"] {
			if i.checksBaselines() {
				legacy, err := catalog.LoadOdysseyChapters(pvfOdysseyBaseline(i, i.OdysseyChapterPath, "DFO_ODYSSEY_CHAPTERS", "odyssey-chapters-release.json"))
				if err != nil {
					return err
				}
				if legacy.Source.Checksum != chapters.Source.Checksum {
					return fmt.Errorf("Odyssey chapters baseline source mismatch")
				}
				if err := verifyPVFCatalog(legacy, chapters); err != nil {
					return fmt.Errorf("Odyssey chapters: %w", err)
				}
			}
			c.OdysseyChapters = chapters
			log.Printf("PVF Odyssey chapters prepared: chapters=%d dungeons=%d reward templates=%d", chapters.ChapterCount, chapters.DungeonCount, chapters.RewardTemplate)
		}
	}
	if selected["odyssey-drop"] {
		direct, err := loot.ImportOdysseyChapterDrop(chapters, *c.Items, policy.OdysseyDrops)
		if err != nil {
			return err
		}
		if i.checksBaselines() {
			legacy, err := loot.LoadOdysseyChapterDrop(pvfOdysseyBaseline(i, i.OdysseyDropPath, "DFO_ODYSSEY_CHAPTER_DROP", "odyssey-chapter-drop-release.json"))
			if err != nil {
				return err
			}
			if legacy.Source.Checksum != direct.Source.Checksum {
				return fmt.Errorf("Odyssey drop baseline source mismatch")
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("Odyssey chapter drop: %w", err)
			}
		}
		c.OdysseyDrop = direct
		log.Printf("PVF Odyssey chapter drop prepared: %d rows; chapter activation and rates retained", len(direct.Drops))
	}
	if selected["odyssey-currency"] {
		direct, err := s.OdysseyCurrency(*c.Items, policy.OdysseyCurrency)
		if err != nil {
			return err
		}
		if i.checksBaselines() {
			legacy, err := loot.LoadOdysseyCurrency(pvfOdysseyBaseline(i, i.OdysseyCurrencyPath, "DFO_ODYSSEY_COIN_RULES", "odyssey-currency.json"))
			if err != nil {
				return err
			}
			if legacy.Source != direct.Source {
				return fmt.Errorf("Odyssey currency baseline source mismatch")
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("Odyssey currency: %w", err)
			}
		}
		c.OdysseyCurrency = direct
		log.Printf("PVF Odyssey currency prepared: %d templates; rates=%v operator policy retained", len(direct.Items), direct.Rates)
	}
	if selected["odyssey-weapons"] {
		direct, err := s.OdysseyWeapons(*c.Items)
		if err != nil {
			return err
		}
		if i.checksBaselines() {
			legacy, err := catalog.LoadOdysseyWeaponChoices(pvfOdysseyBaseline(i, i.OdysseyWeaponPath, "DFO_ODYSSEY_WEAPON_BOX", "odyssey-weapon-box-release.json"))
			if err != nil {
				return err
			}
			if legacy.Source != direct.Source {
				return fmt.Errorf("Odyssey weapon baseline source mismatch")
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("Odyssey weapons: %w", err)
			}
		}
		c.OdysseyWeapons = &direct
		log.Printf("PVF Odyssey creation weapons prepared: categories=%d; existing rewards activation retained", len(direct.Categories))
	}
	s.ReleaseReadCaches()
	return nil
}

func (c *Catalogs) LoadOdysseyGrowth(path string) (*catalog.OdysseyGrowth, error) {
	if err := c.RequireSelected("odyssey-growth", c.OdysseyGrowth != nil); err != nil {
		return nil, err
	}
	if c.OdysseyGrowth != nil {
		return c.OdysseyGrowth, nil
	}
	return nil, nativeContentRequired("odyssey-growth")
}
func (c *Catalogs) LoadOdysseyChapters(path string) (*catalog.OdysseyChapters, error) {
	if err := c.RequireSelected("odyssey-chapters", c.OdysseyChapters != nil); err != nil {
		return nil, err
	}
	if c.OdysseyChapters != nil {
		return c.OdysseyChapters, nil
	}
	return nil, nativeContentRequired("odyssey-chapters")
}
func (c *Catalogs) LoadOdysseyDrop(path string) (*loot.OdysseyChapterDrop, error) {
	if err := c.RequireSelected("odyssey-drop", c.OdysseyDrop != nil); err != nil {
		return nil, err
	}
	if c.OdysseyDrop != nil {
		return c.OdysseyDrop, nil
	}
	return nil, nativeContentRequired("odyssey-drop")
}
func (c *Catalogs) LoadOdysseyCurrency(path string) (*loot.OdysseyCurrency, error) {
	if err := c.RequireSelected("odyssey-currency", c.OdysseyCurrency != nil); err != nil {
		return nil, err
	}
	if c.OdysseyCurrency != nil {
		return c.OdysseyCurrency, nil
	}
	return nil, nativeContentRequired("odyssey-currency")
}
func (c *Catalogs) LoadOdysseyWeapons(path string) (catalog.OdysseyWeaponChoices, error) {
	if err := c.RequireSelected("odyssey-weapons", c.OdysseyWeapons != nil); err != nil {
		return catalog.OdysseyWeaponChoices{}, err
	}
	if c.OdysseyWeapons != nil {
		return *c.OdysseyWeapons, nil
	}
	return catalog.OdysseyWeaponChoices{}, nativeContentRequired("odyssey-weapons")
}

func preparePVFOdysseyRoutes(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["odyssey-routes"] {
		return nil
	}
	direct, err := s.OdysseyJournalRoutes()
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := character.EmbeddedOdysseyJournalRoutes()
		if err != nil {
			return err
		}
		if err := verifyPVFCatalog(old.Nodes, direct.Nodes); err != nil {
			return fmt.Errorf("Odyssey journal routes: %w", err)
		}
	}
	c.OdysseyRoutes = direct
	s.ReleaseReadCaches()
	log.Printf("PVF Odyssey journal routes prepared: nodes=%d dungeon references=50; prior-node membership and native destinations retained", len(direct.Nodes))
	return nil
}

func (c *Catalogs) BindOdysseyRoutes(s *character.ProgressionService) error {
	if err := c.RequireSelected("odyssey-routes", c.OdysseyRoutes != nil); err != nil {
		return err
	}
	if c.OdysseyRoutes == nil {
		return nil
	}
	if s == nil {
		return fmt.Errorf("missing progression service for Odyssey routes")
	}
	routes, err := catalog.NewOdysseyJournalRoutes(*c.OdysseyRoutes)
	if err != nil {
		return err
	}
	s.JournalRoutes = routes
	return nil
}

func preparePVFRecommended(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["adventure-recommended"] {
		return nil
	}
	direct, err := s.RecommendedDungeons()
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := adventure.EmbeddedRecommendedRules()
		if err != nil {
			return err
		}
		baseline := *old
		if baseline.SourceChecksum != direct.SourceChecksum {
			if baseline.SourceChecksum != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.SourceChecksum != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80" {
				return fmt.Errorf("recommended baseline has unknown source provenance")
			}
			baseline.SourceChecksum = direct.SourceChecksum
		}
		if err := verifyPVFCatalog(&baseline, direct); err != nil {
			return fmt.Errorf("recommended: %w", err)
		}
	}
	c.RecommendedRules = direct
	s.ReleaseReadCaches()
	log.Printf("PVF recommended dungeons prepared: ranges=%d excluded=%d ambiguous=%d unavailable worldmaps=%d; source eligibility retained", len(direct.Ranges), len(direct.Excluded), len(direct.AmbiguousDungeons), len(direct.UnavailableWorldmaps))
	return nil
}

func (c *Catalogs) InstallRecommendedRules() (func(), error) {
	if err := c.RequireSelected("adventure-recommended", c.RecommendedRules != nil); err != nil {
		return nil, err
	}
	if c.RecommendedRules == nil {
		return func() {}, nil
	}
	return adventure.InstallRecommendedRules(c.RecommendedRules)
}

func preparePVFSeason(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["season"] {
		return nil
	}
	if c.Items == nil {
		return fmt.Errorf("native season requires native item index")
	}
	direct, err := s.Season(*c.Items)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := adventure.EmbeddedSeasonRules()
		if err != nil {
			return err
		}
		baseline := *old
		if baseline.SourcePath != direct.SourcePath {
			return fmt.Errorf("season source path mismatch")
		}
		if baseline.SourceChecksum != direct.SourceChecksum {
			if baseline.SourceChecksum != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.SourceChecksum != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80" {
				return fmt.Errorf("season baseline has unknown provenance")
			}
			baseline.SourceChecksum = direct.SourceChecksum
		}
		if err := verifyPVFCatalog(&baseline, direct); err != nil {
			return fmt.Errorf("season: %w", err)
		}
	}
	c.SeasonRules = direct
	s.ReleaseReadCaches()
	log.Printf("PVF season rules prepared: levels=%d contents=%d penalties=%d capsules=%d reward items=%d oath equipment=%d; source experience and cost key retained", len(direct.Levels), len(direct.Contents), len(direct.Penalties), len(direct.Capsules), len(direct.Items), len(direct.OathEquipment))
	return nil
}

func (c *Catalogs) InstallSeasonRules() (func(), error) {
	if err := c.RequireSelected("season", c.SeasonRules != nil); err != nil {
		return nil, err
	}
	if c.SeasonRules == nil {
		return func() {}, nil
	}
	return adventure.InstallSeasonRules(c.SeasonRules)
}

type pvfContentPolicy struct {
	BleedingMine    loot.BleedingMinePolicy         `json:"bleeding_mine"`
	BlackPurgatory  loot.BlackPurgatoryPolicy       `json:"black_purgatory"`
	OdysseyDrops    []loot.OdysseyChapterDropPolicy `json:"odyssey_chapter_drops"`
	OdysseyCurrency loot.OdysseyCurrencyPolicy      `json:"odyssey_currency"`
	Version         int                             `json:"version"`
}

func readPVFContentPolicy(path string) (pvfContentPolicy, error) {
	if path == "" {
		return pvfContentPolicy{Version: 1}, nil
	}
	var p pvfContentPolicy
	f, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("content policy has trailing data")
	}
	if p.Version != 1 {
		return p, fmt.Errorf("invalid content selection policy")
	}
	return p, nil
}

func preparePVFSpecial(c *Catalogs, s *Source, selected map[string]bool, inputs CatalogInputs) error {
	if selected["apocalypse"] {
		direct, err := s.Apocalypse()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			path := inputs.ApocalypsePath
			if path == "" {
				path = filepath.Join(filepath.Dir(inputs.IndexPath), "apocalypse.generated.json")
			}
			legacy, err := catalog.LoadApocalypseCatalog(path)
			if err != nil {
				return err
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("apocalypse: %w", err)
			}
		}
		c.Apocalypse = direct
		log.Printf("PVF apocalypse prepared: records=%d operations=%d phases=%d duty records=%d; positional rewards and operation behavior retained", direct.RecordCount, len(direct.Operations), len(direct.PhaseClock), len(direct.Duties.Records))
		s.ReleaseReadCaches()
	}
	if selected["bakal-raid"] {
		// 巴卡尔规则真源是 contents/2022/bakalraid/etc/bakal.etc；没有历史
		// JSON 快照可比对（该团本为新建），严格形状校验由导入器内部完成。
		direct, err := s.BakalRaid()
		if err != nil {
			return err
		}
		c.Bakal = direct
		log.Printf("PVF bakal raid prepared: dungeons=%d locations=%d bosses=%d anger window=%ds settlement=%ds; raid rules retained", len(direct.Dungeons), len(direct.Locations), len(direct.Bosses), direct.NormalPhase.AngerWindow.Secs, direct.NormalPhase.SettlementTimer.Secs)
		s.ReleaseReadCaches()
	}
	if selected["attunement"] {
		// 副本范围来自源：etc/rewardboostinfo/**.ctp 各自声明 [dungeon index]，
		// 不再读 configs 的 attunement_dungeons（单一内容真源铁律，server/AGENTS.md §0）。
		direct, err := s.Attunement()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			path := inputs.AttunementPath
			if override := os.Getenv("DFO_ATTUNEMENT_REWARDS"); override != "" {
				path = override
			}
			if path == "" {
				path = filepath.Join(filepath.Dir(inputs.IndexPath), "attunement-rewards.generated.json")
			}
			legacy, err := loot.LoadAttunementRewards(path)
			if err != nil {
				return err
			}
			if legacy.Archive.Checksum != direct.Archive.Checksum {
				return fmt.Errorf("attunement baseline source mismatch")
			}
			legacy.Archive = direct.Archive
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("attunement: %w", err)
			}
		}
		c.Attunement = direct
		log.Printf("PVF attunement prepared: dungeons=%v templates=%d coupon rows=%d; rebalance and omen policy unchanged", direct.Dungeons(), len(direct.Templates()), direct.Coupons())
		s.ReleaseReadCaches()
	}
	return nil
}

func (c *Catalogs) LoadAttunement(path string) (*loot.AttunementRewards, error) {
	if err := c.RequireSelected("attunement", c.Attunement != nil); err != nil {
		return nil, err
	}
	if c.Attunement != nil {
		return c.Attunement.Clone()
	}
	return nil, nativeContentRequired("attunement")
}

func (c *Catalogs) LoadApocalypse(path string) (*catalog.ApocalypseCatalog, error) {
	if err := c.RequireSelected("apocalypse", c.Apocalypse != nil); err != nil {
		return nil, err
	}
	if c.Apocalypse != nil {
		return c.Apocalypse, nil
	}
	return nil, nativeContentRequired("apocalypse")
}

// LoadBakalRaid hands the prepared bakal rules to the wire layer; there is no
// legacy JSON projection for this raid, so unselected profiles are refused.
func (c *Catalogs) LoadBakalRaid() (*catalog.BakalRaidRules, error) {
	if err := c.RequireSelected("bakal-raid", c.Bakal != nil); err != nil {
		return nil, err
	}
	if c.Bakal != nil {
		return c.Bakal, nil
	}
	return nil, nativeContentRequired("bakal-raid")
}
