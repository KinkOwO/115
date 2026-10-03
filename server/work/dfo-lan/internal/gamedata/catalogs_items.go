package gamedata

import (
	"dfolan/internal/cashshop"
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
	"reflect"
	"strconv"
)

func readPVFBoxPolicy(path string) (inventory.BoxSourcePolicy, error) {
	var p inventory.BoxSourcePolicy
	f, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err = d.Decode(&p); err != nil {
		return p, err
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("box policy has trailing data")
	}
	return p, nil
}
func preparePVFBoxes(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["boxes"] {
		return nil
	}
	if c.Items == nil {
		return fmt.Errorf("native boxes require native item index")
	}
	p, err := readPVFBoxPolicy(i.BoxPolicyPath)
	if err != nil {
		return err
	}
	direct, err := s.Boxes(*c.Items, p)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		path := i.BoxesPath
		if path == "" {
			path = filepath.Join(filepath.Dir(i.IndexPath), "boxes.json")
		}
		old, err := inventory.LoadBoxes(path)
		if err != nil {
			return err
		}
		if err = auditPVFBoxes(old, direct); err != nil {
			return err
		}
	}
	c.Boxes = direct
	s.ReleaseReadCaches()
	log.Printf("PVF boxes prepared: tables=%d rewards=%d raw sources=%d; unique native COS material bindings and existing point/grant rules retained", direct.TableCount(), direct.RewardCount(), len(direct.Sources))
	return nil
}
func auditPVFBoxes(old, direct *inventory.BoxCatalog) error {
	if old == nil || direct == nil || direct.Source != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80" || (old.Source != "inner Script.pvf .cos content scripts" && old.Source != direct.Source) {
		return fmt.Errorf("unknown box source provenance")
	}
	// The old artifact omitted raw hashes. Audit every exported source field;
	// the native catalog additionally records exact archive/file identities.
	baseline, native := *old, *direct
	baseline.Sources = nil
	native.Sources = nil
	if err := auditPVFCatalog(baseline, native); err != nil {
		return fmt.Errorf("boxes: %w", err)
	}
	return nil
}
func (c *Catalogs) LoadBoxes(path, source string) (*inventory.BoxCatalog, error) {
	if err := c.RequireSelected("boxes", c.Boxes != nil); err != nil {
		return nil, err
	}
	if c.Boxes != nil {
		if c.Boxes.Source != source {
			return nil, fmt.Errorf("native box source differs from save catalog")
		}
		return c.Boxes, nil
	}
	return inventory.LoadBoxes(path)
}

func preparePVFCashShop(c *Catalogs, s *Source, i CatalogInputs) error {
	raw, err := s.CashShop()
	if err != nil {
		return err
	}
	raw.Release = i.CashshopRelease
	direct, err := cashshop.NewPilot(raw, s.Snapshot().Checksum)
	if err != nil {
		return err
	}
	c.CashShop = direct
	s.ReleaseReadCaches()
	log.Printf("PVF cashshop prepared: rows=%d enabled=%d release=%t; native prices, item cells and purchase policies retained", len(direct.Config.Entries), direct.EnabledCount(), direct.Config.Release)
	return nil
}

// LoadCashShop returns the prepared PVF catalog. There is no JSON fallback:
// the historical export was a baseline snapshot and its files are gone, so a
// selected-but-unprepared domain is an explicit error.
func (c *Catalogs) LoadCashShop(source string, release bool) (*cashshop.Pilot, error) {
	if err := c.RequireSelected("cashshop", c.CashShop != nil); err != nil {
		return nil, err
	}
	if c.CashShop == nil {
		return nil, fmt.Errorf("PVF cashshop catalog is not prepared")
	}
	if c.CashShop.Config.Source.Checksum != source || c.CashShop.Config.Release != release {
		return nil, fmt.Errorf("prepared cashshop source/release settings changed")
	}
	return c.CashShop, nil
}

func preparePVFLearning(c *Catalogs, s *Source, chars catalog.Characters) error {
	if chars.Source.Checksum == "" {
		// No characters domain selected: bind the learning source to a native
		// character catalog instead of a stale historical anchor.
		native, err := s.Characters("")
		if err != nil {
			return err
		}
		chars = native
	}
	direct, err := s.Learning(chars)
	if err != nil {
		return err
	}
	c.Learning = direct
	log.Printf("PVF learning prepared: %d definitions", len(direct.Rows))
	s.ReleaseReadCaches()
	return nil
}

func preparePVFCommerce(c *Catalogs, s *Source, selected map[string]bool, inputs CatalogInputs) error {
	if selected["prices"] {
		var direct *catalog.ShopPrices
		var err error
		if c.ItemBasics != nil {
			direct = c.ItemBasics.Prices
		} else {
			direct, err = s.ShopPrices(*c.Items)
		}
		if err != nil {
			return err
		}
		c.Prices = direct
		log.Printf("PVF prices prepared: %d definitions", len(direct.Items))
		s.ReleaseReadCaches()
	}
	if selected["materials"] {
		var direct *catalog.ItemMaterials
		var err error
		if c.ItemBasics != nil && c.ItemBasics.Materials != nil {
			direct = c.ItemBasics.Materials
		} else {
			direct, err = s.ItemMaterials(*c.Items)
		}
		if err != nil {
			return err
		}
		c.Materials = direct
		log.Printf("PVF materials prepared: %d definitions", len(direct.Items))
		s.ReleaseReadCaches()
	}
	if selected["boosters"] {
		var direct map[uint32]catalog.BoosterDefinition
		var err error
		if c.ItemBasics != nil && c.ItemBasics.Boosters != nil {
			direct = c.ItemBasics.Boosters
		} else {
			direct, err = s.Boosters(*c.Items)
		}
		if err != nil {
			return err
		}
		c.Boosters = direct
		log.Printf("PVF boosters prepared: %d definitions", len(direct))
		s.ReleaseReadCaches()
	}
	return nil
}

func (c *Catalogs) LoadLearning(path, checksum string) (*character.LearningCatalog, error) {
	if err := c.RequireSelected("skills", c.Learning != nil); err != nil {
		return nil, err
	}
	if c.Learning != nil {
		if c.Learning.Source.Checksum != checksum {
			return nil, fmt.Errorf("prepared learning source mismatch")
		}
		return c.Learning, nil
	}
	return nil, fmt.Errorf("skills require the native PVF skills domain")
}
func (c *Catalogs) LoadShopPrices(path, checksum string) (*catalog.ShopPrices, error) {
	if err := c.RequireSelected("prices", c.Prices != nil); err != nil {
		return nil, err
	}
	if c.Prices != nil {
		if c.Prices.Source != checksum {
			return nil, fmt.Errorf("prepared price source mismatch")
		}
		return c.Prices, nil
	}
	return nil, fmt.Errorf("NPC prices require the native PVF prices domain")
}
func (c *Catalogs) LoadItemMaterials(path string) (*catalog.ItemMaterials, error) {
	if err := c.RequireSelected("materials", c.Materials != nil); err != nil {
		return nil, err
	}
	if c.Materials != nil {
		return c.Materials, nil
	}
	return nil, fmt.Errorf("item material costs require the native PVF materials domain")
}

func preparePVFLoot(c *Catalogs, s *Source, inputs CatalogInputs) error {
	policy, err := inventory.ReadDropPolicy(inputs.DropPolicyPath)
	if err != nil {
		return err
	}
	direct, err := s.Loot(policy.MaximumLootGrade)
	if err != nil {
		return err
	}
	for _, id := range policy.ExcludedLootIDs {
		delete(direct.Items, id)
	}
	log.Printf("PVF loot selection exclusions retained: %v", policy.ExcludedLootIDs)
	direct.OrdinaryMonsterItemRate, err = loot.ParseMonsterItemDropPercent(os.Getenv("DFO_ORDINARY_MONSTER_ITEM_DROP_PERCENT"))
	if err != nil {
		return err
	}
	direct.MonsterItemExclusions = map[uint32]bool{}
	for _, id := range policy.ExcludedLootIDs {
		direct.MonsterItemExclusions[id] = true
	}
	log.Printf("ordinary MOB item compatibility rate=%d/10000 (DFO_ORDINARY_MONSTER_ITEM_DROP_PERCENT, default 10%%); source pairs retain selection weights", direct.OrdinaryMonsterItemRate)
	direct.OrdinaryWorldDropPercent, err = loot.ParseWorldDropPercent(os.Getenv("DFO_ORDINARY_WORLD_DROP_PERCENT"))
	if err != nil {
		return err
	}
	log.Printf("ordinary world drop compatibility multiplier=%d%% (DFO_ORDINARY_WORLD_DROP_PERCENT, default 100%%); source weights /100000", direct.OrdinaryWorldDropPercent)
	if inputs.checksBaselines() {
		path := inputs.LootPath
		if override := os.Getenv("DFO_LOOT_CATALOG"); override != "" {
			path = override
		}
		if path == "" {
			path = filepath.Join(filepath.Dir(inputs.IndexPath), "loot.next25.json")
		}
		legacy, err := catalog.LoadLoot(path)
		if err != nil {
			return err
		}
		if legacy.Source.Checksum != direct.Source.Checksum {
			return fmt.Errorf("loot baseline source mismatch")
		}
		if err := verifyPVFCatalog(legacy, direct); err != nil {
			return fmt.Errorf("loot: %w", err)
		}
	}
	c.Loot = &direct
	log.Printf("PVF loot prepared: maximum grade=%d stackable candidates=%d drop groups=%d dungeon indexes=%d; ordinary difficulty/creation weights from PVF", direct.MaximumGrade, len(direct.Items), len(direct.DropGroups), len(direct.DungeonDropInfo))
	s.ReleaseReadCaches()
	return nil
}

func preparePVFEquipmentSelection(c *Catalogs, s *Source, inputs CatalogInputs) error {
	policy, err := inventory.ReadDropPolicy(inputs.DropPolicyPath)
	if err != nil {
		return err
	}
	quests := c.Quests
	if quests == nil {
		native, err := s.Quests("")
		if err != nil {
			return err
		}
		quests = &native
	}
	direct, err := s.EquipmentSelection(*c.Items, *quests, policy)
	if err != nil {
		return err
	}
	if inputs.checksBaselines() {
		paths := []string{inputs.EquipmentPath, inputs.QuestEquipmentPath}
		if paths[0] == "" && paths[1] == "" {
			paths = []string{filepath.Join(filepath.Dir(inputs.IndexPath), "equipment.current37.json")}
		}
		seen := map[string]bool{}
		for _, path := range paths {
			if path == "" || seen[path] {
				continue
			}
			seen[path] = true
			legacy, err := inventory.LoadEquipmentCatalog(path, s.Snapshot().Checksum)
			if err != nil {
				return err
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("equipment selection %s: %w", path, err)
			}
			if err := verifyPVFCatalog(legacy.DropPool(), direct.DropPool()); err != nil {
				return fmt.Errorf("equipment drop pool: %w", err)
			}
		}
	}
	c.Selection = direct
	log.Printf("PVF equipment selection prepared: basic whitelist=%d source quest additions=%d total=%d legacy drop pool=%d ordinary source pool=%d", len(policy.BasicEquipmentIDs), len(direct.Rows)-len(policy.BasicEquipmentIDs), len(direct.Rows), len(direct.DropPool()), len(direct.OrdinaryPool))
	s.ReleaseReadCaches()
	return nil
}

func (c *Catalogs) LoadLoot(path string) (catalog.LootCatalog, error) {
	if err := c.RequireSelected("loot", c.Loot != nil); err != nil {
		return catalog.LootCatalog{}, err
	}
	if c.Loot != nil {
		return *c.Loot, nil
	}
	return catalog.LoadLoot(path)
}

func (c *Catalogs) LoadEquipmentSelection(path, source string) (*inventory.EquipmentCatalog, error) {
	if err := c.RequireSelected("equipment-selection", c.Selection != nil); err != nil {
		return nil, err
	}
	if c.Selection != nil {
		if source != c.Selection.Source.Checksum {
			return nil, fmt.Errorf("prepared equipment selection source mismatch")
		}
		copy := *c.Selection
		return &copy, nil
	}
	return inventory.LoadEquipmentCatalog(path, source)
}

func preparePVFEnhancements(c *Catalogs, s *Source, inputs CatalogInputs) error {
	direct := c.Enhancements
	var err error
	if direct == nil {
		direct, err = s.Enhancements(*c.Items, inputs.EnhancementPolicyPath)
	}
	if err != nil {
		return err
	}
	c.Enhancements = direct
	log.Printf("PVF enhancements prepared: reinforcement tickets=%d amplify tickets=%d grimoires=%d enchant beads=%d reinforcement levels=%d amplify levels=%d", len(direct.ReinforcementTickets), len(direct.AmplifyTickets), len(direct.Grimoires.Grimoires), len(direct.Enchant.Beads), len(direct.Gold.Levels), len(direct.Amplify.Levels))
	s.ReleaseReadCaches()
	return nil
}

func (c *Catalogs) LoadEnhancements(dir string) error {
	if err := c.RequireSelected("enhancements", c.Enhancements != nil); err != nil {
		return err
	}
	if c.Enhancements != nil {
		return c.Enhancements.Activate()
	}
	return fmt.Errorf("enhancements require the native PVF enhancements domain")
}

func preparePVFItemShops(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["item-shops"] {
		return nil
	}
	p, err := catalog.ReadItemShopSourcePolicy(i.ItemShopPolicyPath)
	if err != nil {
		return err
	}
	native, err := s.ItemShops(p)
	if err != nil {
		return err
	}
	direct := native.Catalog
	if i.checksBaselines() {
		name := i.ItemShopPath
		if name == "" {
			name = filepath.Join(filepath.Dir(i.IndexPath), "itemshop-candidate.json")
		}
		old, err := catalog.LoadItemShops(name)
		if err != nil {
			return err
		}
		if old.Source.Checksum != direct.Source.Checksum {
			return fmt.Errorf("item shop source identity changed")
		}
		if err = verifyPVFCatalog(old, direct); err != nil {
			return fmt.Errorf("item shops: %w", err)
		}
		for key, shop := range old.Shops {
			id64, _ := strconv.ParseUint(key, 10, 32)
			id := uint32(id64)
			for _, offer := range shop.Offers {
				a, al, ap := old.Materials(id, offer.Template)
				b, bl, bp := direct.Materials(id, offer.Template)
				if al != bl || ap != bp || !reflect.DeepEqual(a, b) || old.PurchaseAmount(id, offer.Template) != direct.PurchaseAmount(id, offer.Template) || old.Listed(id, offer.Template) != direct.Listed(id, offer.Template) {
					return fmt.Errorf("item shop %s effective payable lookup differs", key)
				}
				x1, x2, x3, x4 := old.PurchaseLimit(id, offer.Template)
				y1, y2, y3, y4 := direct.PurchaseLimit(id, offer.Template)
				if x1 != y1 || x2 != y2 || x3 != y3 || x4 != y4 {
					return fmt.Errorf("item shop %s limit behavior changed", key)
				}
			}
		}
	}
	c.ItemShops = direct
	s.ReleaseReadCaches()
	count := 0
	for _, shop := range direct.Shops {
		count += len(shop.Offers)
	}
	log.Printf("PVF item shops prepared: shops=%d offers=%d native routes=%d compatibility list routes=%d explicit service routes=%d native list SHA=%s; original first-payable and unlimited policy retained", len(direct.Shops), count, native.NativeRoutes, native.AliasedRoutes, native.ExplicitRoutes, native.IndexHash)
	return nil
}
func (c *Catalogs) LoadItemShops(name, source string) (*catalog.ItemShops, error) {
	if err := c.RequireSelected("item-shops", c.ItemShops != nil); err != nil {
		return nil, err
	}
	if c.ItemShops != nil {
		if c.ItemShops.Source.Checksum != source {
			return nil, fmt.Errorf("prepared item shop source differs from save catalog")
		}
		return c.ItemShops, nil
	}
	return catalog.LoadItemShops(name)
}

// preparePVFLottery discovers PVF pools and delegates gateway-only validation.
func preparePVFLottery(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs, adapters CatalogAdapters) error {
	if !selected["lottery"] {
		return nil
	}
	if c.Items == nil {
		return fmt.Errorf("native lotteries require native item index")
	}
	direct, scope, err := s.DiscoverLottery(*c.Items)
	if err != nil {
		return err
	}
	if adapters.ValidateLottery == nil {
		return fmt.Errorf("PVF lottery validation adapter is required")
	}
	if err := adapters.ValidateLottery(direct, *c.Items, "", false); err != nil {
		return fmt.Errorf("validate PVF lottery: %w", err)
	}
	c.LotteryTables = &direct
	for _, issue := range scope.Issues {
		log.Printf("PVF lottery unavailable: template=%d path=%s sha256=%s reason=%s", issue.Template, issue.Path, issue.SHA256, issue.Reason)
	}
	s.ReleaseReadCaches()
	log.Printf("PVF lotteries discovered: candidates=%d item pools=%d equipment pools=%d unavailable=%d; complete source odds retained", scope.Candidates, len(direct.Items.Pools), len(direct.Equipment.Pools), len(scope.Issues))
	return nil
}

// LoadLotteryItemPools exposes only the prepared native PVF projection.
func (c *Catalogs) LoadLotteryItemPools(_ string) (catalog.LotteryPoolCatalog, error) {
	if c.LotteryTables != nil {
		return c.LotteryTables.Items, nil
	}
	return catalog.LotteryPoolCatalog{}, fmt.Errorf("native PVF lottery catalog is not prepared")
}

func (c *Catalogs) LoadLotteryEquipmentPools(_ string) (catalog.LotteryPoolCatalog, error) {
	if c.LotteryTables != nil {
		return c.LotteryTables.Equipment, nil
	}
	return catalog.LotteryPoolCatalog{}, fmt.Errorf("native PVF lottery catalog is not prepared")
}

func preparePVFSelectionBoxes(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["selection-boxes"] {
		return nil
	}
	if c.Items == nil {
		return fmt.Errorf("native selection boxes require native item index")
	}
	policy, err := catalog.ReadSelectionBoxPolicy(i.SelectionPolicyPath)
	if err != nil {
		return err
	}
	direct, err := s.SelectionBoxes(*c.Items, policy)
	if err != nil {
		return err
	}
	c.SelectionBoxes = direct
	s.ReleaseReadCaches()
	log.Printf("PVF selection boxes prepared: boxes=%d fixed=%d unparsed=%d rejected=%d; source-discovered scope with policy whitelist", len(direct.Boxes), len(direct.Fixed), len(direct.Unparsed), len(direct.Rejected))
	return nil
}

func (c *Catalogs) LoadSelectionBoxes(path string) (*catalog.SelectionBoxes, error) {
	if err := c.RequireSelected("selection-boxes", c.SelectionBoxes != nil); err != nil {
		return nil, err
	}
	if c.SelectionBoxes != nil {
		return c.SelectionBoxes, nil
	}
	return nil, fmt.Errorf("selection boxes require the native PVF selection-boxes domain")
}
