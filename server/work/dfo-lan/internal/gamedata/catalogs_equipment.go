package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// 装备调适（CMD2258）的直读准备与安装。
//
// 源（唯一内容真源 = 内层 PVF）：
//
//	etc/115lvability/equipmentawakeningoptionsystem.cos   规则表（阶段上限/成本/返还/成功率/升品映射）
//	etc/115lvability/equipmentawakeningoption.lst          选项索引（`[equipment awakening option]` 的 ID → 加成表）
//
// 与其它直读目录一致：**不读任何 configs/*.json**，解析失败直接报错（不静默回落）。
func preparePVFEquipmentAwakening(c *Catalogs, s *Source) error {
	rules := c.AwakeningRules
	if rules == nil {
		imported, err := s.EquipmentAwakening()
		if err != nil {
			return err
		}
		rules = imported
	}
	options := c.AwakeningOptions
	if options == nil {
		imported, err := s.EquipmentAwakeningOptions()
		if err != nil {
			return err
		}
		options = imported
	}
	c.AwakeningRules, c.AwakeningOptions = rules, options
	s.ReleaseReadCaches()
	log.Printf("PVF equipment awakening prepared: max=%d conditions=%d stage-tables=%d upgrade-sources=%d options=%d",
		rules.MaxLevel, len(rules.Infos), awakeningStageTables(rules), len(rules.Templates()), len(options.Entries))
	return nil
}

// awakeningStageTables 统计规则里声明的成本行总数（启动日志的规模核对用）。
func awakeningStageTables(rules *catalog.EquipmentAwakeningRules) int {
	total := 0
	for _, info := range rules.Infos {
		for _, group := range info.Groups {
			total += len(group.Rows)
		}
	}
	return total
}

// installEquipmentAwakening 把直读投影装进 inventory 的运行期注入点。
func (c *Catalogs) InstallEquipmentAwakening() (func(), error) {
	if c.AwakeningRules == nil {
		return func() {}, nil
	}
	inventory.SetEquipmentAwakeningRules(c.AwakeningRules)
	return func() { inventory.SetEquipmentAwakeningRules(nil) }, nil
}

func readPVFCharacterPolicy(path string) (catalog.CharacterRuntimePolicy, error) {
	var policy catalog.CharacterRuntimePolicy
	f, err := os.Open(path)
	if err != nil {
		return policy, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err = d.Decode(&policy); err != nil {
		return policy, err
	}
	if err = d.Decode(new(any)); err != io.EOF {
		return policy, fmt.Errorf("character runtime policy has trailing data")
	}
	return policy, nil
}
func preparePVFCharacters(c *Catalogs, s *Source, policy catalog.CharacterRuntimePolicy, path string, i CatalogInputs) error {
	raw, err := s.Characters("")
	if err != nil {
		return err
	}
	direct, err := catalog.ProjectCharacterRuntime(raw, policy)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := catalog.LoadCharacters(path)
		if err != nil {
			return err
		}
		if old.Source.Checksum != direct.Source.Checksum {
			return fmt.Errorf("character baseline source identity changed")
		}
		if err = verifyPVFCatalog(old, direct); err != nil {
			return fmt.Errorf("characters: %w", err)
		}
	}
	c.SourceCharacters, c.Characters = &raw, &direct
	s.ReleaseReadCaches()
	log.Printf("PVF characters prepared: %d professions; raw growtype views retained; existing shortcut/default-command policy and save source identity preserved", len(direct.Professions))
	return nil
}
func (c *Catalogs) LoadCharacters(path string) (catalog.Characters, error) {
	if err := c.RequireSelected("characters", c.Characters != nil); err != nil {
		return catalog.Characters{}, err
	}
	if c.Characters != nil {
		return *c.Characters, nil
	}
	return catalog.LoadCharacters(path)
}

func preparePVFEquipmentRules(c *Catalogs, s *Source, selected map[string]bool, inputs CatalogInputs) error {
	dir := filepath.Dir(inputs.IndexPath)
	if selected["random-options"] {
		direct, err := s.RandomOptions()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			path := inputs.RandomOptionPath
			if path == "" {
				path = filepath.Join(dir, "randomoption.current37.json")
			}
			legacy, err := inventory.ReadRandomOptionData(path)
			if err != nil {
				return err
			}
			if legacy.Source.Checksum != direct.Source.Checksum {
				return fmt.Errorf("random options source mismatch")
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("random options: %w", err)
			}
		}
		runtime, err := inventory.NewRandomOptionCatalog(direct, s.Snapshot().Checksum)
		if err != nil {
			return err
		}
		c.RandomOptions = runtime
		log.Printf("PVF random options prepared: ratios=%d quantity rows=%d groups=%d choices=%d costs=%d", len(direct.ValueRatios), len(direct.OptionQuantities), len(direct.OptionGroups), len(direct.GroupChoices), len(direct.BreakSealCosts))
		s.ReleaseReadCaches()
	}
	if selected["oath-grades"] {
		direct, err := s.OathGrades()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			path := inputs.OathPath
			if path == "" {
				path = filepath.Join(dir, "oath-grades.json")
			}
			legacy, err := inventory.LoadOathGradeTable(path)
			if err != nil {
				return err
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("oath grades: %w", err)
			}
		}
		c.Oath = direct
		log.Printf("PVF oath grades prepared: %d entries; diagnostic enable policy unchanged", direct.Len())
		s.ReleaseReadCaches()
	}
	if selected["vault"] {
		direct, err := s.VaultRules(inputs.VaultPolicyPath)
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			path := inputs.VaultPath
			if path == "" {
				path = filepath.Join(dir, "vault.generated.json")
			}
			legacy, err := inventory.LoadVaultRules(path)
			if err != nil {
				return err
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("vault: %w", err)
			}
		}
		c.Vault = &direct
		log.Printf("PVF account vault prepared: level=%d upgrades=%d; client capacity save source retained=%s", direct.Account.RequiredLevel, len(direct.Account.Upgrades), direct.SourceSHA256)
		s.ReleaseReadCaches()
	}
	return nil
}

func preparePVFShields(c *Catalogs, s *Source, jobs catalog.Characters, inputs CatalogInputs) error {
	if jobs.Source.Checksum == "" {
		// No characters domain selected: bind shields to a native character
		// catalog instead of a stale historical anchor.
		native, err := catalog.ImportCharacters(s.archive)
		if err != nil {
			return err
		}
		jobs = native
	}
	wear := inputs.WearRulesPath
	if override := os.Getenv("DFO_EQUIPMENT_WEAR_RULES"); override != "" {
		wear = override
	}
	rules, err := inventory.LoadWearRules(wear, s.Snapshot().Checksum)
	if err != nil {
		return err
	}
	direct, err := s.KnightShields(*c.Items, jobs, rules)
	if err != nil {
		return err
	}
	if inputs.checksBaselines() {
		path := inputs.ShieldPath
		if path == "" {
			path = "equipment-knight-shield.full-candidate.json"
		}
		if path != "" && !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(wear), path)
		}
		legacy, err := inventory.LoadKnightShields(path, s.Snapshot().Checksum)
		if err != nil {
			return err
		}
		if err := verifyPVFCatalog(legacy, direct); err != nil {
			return fmt.Errorf("shields: %w", err)
		}
	}
	c.Shields = direct
	log.Printf("PVF knight shields prepared: %d source window rows, profession=%d; quest refusal retained", len(direct.Rows), direct.Profession)
	s.ReleaseReadCaches()
	return nil
}

func (c *Catalogs) LoadRandomOptions(path, source string) (*inventory.RandomOptionCatalog, error) {
	if err := c.RequireSelected("random-options", c.RandomOptions != nil); err != nil {
		return nil, err
	}
	if c.RandomOptions != nil {
		return c.RandomOptions, c.RandomOptions.ValidateSource(source)
	}
	return inventory.LoadRandomOptionCatalog(path, source)
}
func (c *Catalogs) LoadShields(path, source string) (*inventory.KnightShields, error) {
	if err := c.RequireSelected("shields", c.Shields != nil); err != nil {
		return nil, err
	}
	if c.Shields != nil {
		return c.Shields, c.Shields.Validate(source)
	}
	return inventory.LoadKnightShields(path, source)
}
func (c *Catalogs) LoadOathGrades(path string) (*inventory.OathGradeTable, error) {
	if err := c.RequireSelected("oath-grades", c.Oath != nil); err != nil {
		return nil, err
	}
	if c.Oath != nil {
		return c.Oath, nil
	}
	if path == "" {
		path = "configs/oath-grades.json"
		if exe, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exe), "..", path)
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
			}
		}
	}
	return inventory.LoadOathGradeTable(path)
}
func (c *Catalogs) LoadVaultRules(path string) (inventory.VaultRules, error) {
	if err := c.RequireSelected("vault", c.Vault != nil); err != nil {
		return inventory.VaultRules{}, err
	}
	if c.Vault != nil {
		return *c.Vault, nil
	}
	return inventory.LoadVaultRules(path)
}

func preparePVFFame(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["fame"] {
		return nil
	}
	if c.Items == nil {
		return fmt.Errorf("native fame requires native item index")
	}
	direct := c.FameRules
	var err error
	if direct == nil {
		direct, err = s.Fame(*c.Items)
	}
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := character.EmbeddedFameRules()
		if err != nil {
			return err
		}
		if old.Source != direct.Source && (old.Source != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.Source != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80") {
			return fmt.Errorf("fame baseline has unknown provenance")
		}
		if err := verifyPVFCatalog(old, direct); err != nil {
			return fmt.Errorf("fame: %w", err)
		}
	}
	c.FameRules = direct
	s.ReleaseReadCaches()
	log.Printf("PVF fame rules prepared: tables=%d items=%d sets=%d item points=%d awakening templates=%d sources=%d; existing client formulas retained", len(direct.Tables), len(direct.Items), len(direct.Sets), len(direct.ItemPoints), len(direct.Awakening), len(direct.Sources))
	return nil
}

func (c *Catalogs) InstallFameRules() (func(), error) {
	if err := c.RequireSelected("fame", c.FameRules != nil); err != nil {
		return nil, err
	}
	if c.FameRules == nil {
		return func() {}, nil
	}
	return character.InstallFameRules(c.FameRules)
}

func preparePVFRosterBackgrounds(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["roster-backgrounds"] {
		return nil
	}
	if c.Items == nil {
		return fmt.Errorf("native background tickets require native item index")
	}
	direct, err := s.RosterBackgrounds(*c.Items)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := character.EmbeddedRosterBackgroundTickets()
		if err != nil {
			return err
		}
		if old.Source != direct.Source && (old.Source != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.Source != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80") {
			return fmt.Errorf("background ticket baseline has unknown provenance")
		}
		if err := verifyPVFCatalog(old.Items, direct.Items); err != nil {
			return fmt.Errorf("background tickets: %w", err)
		}
		resources := map[character.RosterBackground]bool{}
		for _, b := range direct.Backgrounds {
			resources[b] = true
		}
		for category := 0; category < 256; category++ {
			for id := 0; id <= 65535; id++ {
				b := character.RosterBackground{Category: uint8(category), ID: uint16(id)}
				if resources[b] != old.ValidRosterBackground(b) {
					return fmt.Errorf("native background boundary differs at %v", b)
				}
			}
		}
	}
	c.RosterBackgrounds = direct
	s.ReleaseReadCaches()
	log.Printf("PVF roster backgrounds prepared: tickets=%d resources=%d; independent item deletion and background authorization dates retained", len(direct.Items), len(direct.Backgrounds))
	return nil
}

func (c *Catalogs) InstallRosterBackgrounds() (func(), error) {
	if err := c.RequireSelected("roster-backgrounds", c.RosterBackgrounds != nil); err != nil {
		return nil, err
	}
	if c.RosterBackgrounds == nil {
		return func() {}, nil
	}
	return character.InstallRosterBackgroundTickets(c.RosterBackgrounds)
}

func readPVFRuleBaseline(path, checksum string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var anchor struct {
		Source struct {
			Checksum string `json:"checksum"`
		} `json:"source"`
	}
	if err = json.Unmarshal(b, &anchor); err != nil {
		return err
	}
	if anchor.Source.Checksum != checksum {
		return fmt.Errorf("rule baseline/PVF source mismatch: %s", path)
	}
	return json.Unmarshal(b, out)
}

func preparePVFRules(c *Catalogs, s *Source, selected map[string]bool, inputs CatalogInputs) error {
	checksum := s.Snapshot().Checksum
	if selected["tutorial"] {
		direct, err := s.Tutorials()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			legacy, err := catalog.LoadTutorialRoutes(inputs.TutorialPath, checksum)
			if err != nil {
				return err
			}
			if err = verifyPVFCatalog(*legacy, direct); err != nil {
				return fmt.Errorf("tutorial: %w", err)
			}
		}
		c.Tutorial = &direct
		log.Printf("PVF tutorial prepared: %d source flows", len(direct.Flows))
		s.ReleaseReadCaches()
	}
	if selected["periods"] {
		var direct catalog.ItemPeriodCatalog
		var err error
		if c.ItemBasics != nil {
			direct = *c.ItemBasics.Periods
		} else {
			direct, err = s.ItemPeriods()
		}
		if err != nil {
			return err
		}
		c.Periods = direct.Templates
		log.Printf("PVF item periods prepared: %d templates", len(c.Periods))
		s.ReleaseReadCaches()
	}
	if selected["skins"] {
		var direct catalog.SkinStorageCatalog
		var err error
		if c.ItemBasics != nil && c.ItemBasics.Skins != nil {
			direct = *c.ItemBasics.Skins
		} else {
			direct, err = s.SkinStorage()
		}
		if err != nil {
			return err
		}
		c.Skins = make(map[uint32]catalog.SkinStorageEntry, len(direct.Entries))
		for _, e := range direct.Entries {
			c.Skins[e.Template] = e
		}
		log.Printf("PVF skin storage prepared: %d templates, %d unresolved source skins", len(c.Skins), len(direct.MissingSkins))
		s.ReleaseReadCaches()
	}
	if selected["journal"] {
		direct, err := s.EquipmentJournal()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			legacy, err := catalog.LoadEquipmentJournalRules(inputs.JournalPath, checksum)
			if err != nil {
				return err
			}
			if err = verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("journal: %w", err)
			}
		}
		c.Journal = &direct
		log.Printf("PVF equipment journal prepared: %d categories", len(direct.Categories))
		s.ReleaseReadCaches()
	}
	if selected["create-cost"] {
		direct, err := s.EquipmentCreateCost()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			legacy, err := catalog.LoadEquipmentCreateCost(inputs.CreateCostPath, checksum)
			if err != nil {
				return err
			}
			if err = verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("create-cost: %w", err)
			}
		}
		c.CreateCost = &direct
		log.Printf("PVF equipment creation costs prepared: %d groups", len(direct.Groups))
		s.ReleaseReadCaches()
	}
	return nil
}

func (c *Catalogs) LoadItemPeriods(path, checksum string) ([]uint32, error) {
	if err := c.RequireSelected("periods", c.Prepared("periods")); err != nil {
		return nil, err
	}
	if c.Periods != nil || c.Prepared("periods") {
		return c.Periods, nil
	}
	return nil, fmt.Errorf("item periods require the native PVF periods domain")
}
func (c *Catalogs) LoadSkinStorage(path, checksum string) (map[uint32]catalog.SkinStorageEntry, error) {
	if err := c.RequireSelected("skins", c.Prepared("skins")); err != nil {
		return nil, err
	}
	if c.Skins != nil || c.Prepared("skins") {
		return c.Skins, nil
	}
	return nil, fmt.Errorf("skin storage requires the native PVF skins domain")
}
func (c *Catalogs) LoadEquipmentJournal(path, checksum string) (catalog.EquipmentJournalRules, error) {
	if err := c.RequireSelected("journal", c.Journal != nil); err != nil {
		return catalog.EquipmentJournalRules{}, err
	}
	if c.Journal != nil {
		return *c.Journal, nil
	}
	return catalog.LoadEquipmentJournalRules(path, checksum)
}
func (c *Catalogs) LoadEquipmentCreateCost(path, checksum string) (catalog.EquipmentCreateCost, error) {
	if err := c.RequireSelected("create-cost", c.CreateCost != nil); err != nil {
		return catalog.EquipmentCreateCost{}, err
	}
	if c.CreateCost != nil {
		return *c.CreateCost, nil
	}
	return catalog.LoadEquipmentCreateCost(path, checksum)
}

func (c *Catalogs) LoadTutorialRoutes(path, checksum string) (*catalog.TutorialCatalog, error) {
	if err := c.RequireSelected("tutorial", c.Tutorial != nil); err != nil {
		return nil, err
	}
	if c.Tutorial != nil {
		if c.Tutorial.Source.Checksum != checksum {
			return nil, fmt.Errorf("prepared tutorial source mismatch")
		}
		return c.Tutorial, nil
	}
	return catalog.LoadTutorialRoutes(path, checksum)
}

// 秘宝精度提升（CMD2288）的直读准备与安装。
//
// 源（唯一内容真源 = 内层 PVF）：
//
//	etc/115lvability/soleequipmentsystem.cos   规则表（每件秘宝的 [item index] /
//	                                           [max quality] / [quality need materials] / [quality group]）
//
// 与其它直读目录一致：**不读任何 configs/*.json**，解析失败直接报错（不静默回落）。
func preparePVFSoleEquipment(c *Catalogs, s *Source) error {
	rules := c.SoleRules
	if rules == nil {
		imported, err := s.SoleEquipment()
		if err != nil {
			return err
		}
		rules = imported
	}
	c.SoleRules = rules
	s.ReleaseReadCaches()
	log.Printf("PVF sole equipment prepared: items=%d %s", len(rules.Items), soleItemsSummary(rules))
	return nil
}

// soleItemsSummary 把每件秘宝的模板与上限拼成一行（启动日志的规模核对用）。
func soleItemsSummary(rules *catalog.SoleEquipmentRules) string {
	var b strings.Builder
	for _, template := range rules.Templates() {
		info, _ := rules.Info(template)
		b.WriteString(" ")
		b.WriteString(strconv.FormatUint(uint64(template), 10))
		b.WriteString("(max=")
		b.WriteString(strconv.Itoa(info.MaxQuality))
		b.WriteString(",groups=")
		b.WriteString(strconv.Itoa(len(info.Groups)))
		b.WriteString(",create=")
		b.WriteString(strconv.Itoa(len(info.CreateGroups)))
		b.WriteString(")")
	}
	return b.String()
}

// installSoleEquipment 把直读投影装进 inventory 的运行期注入点。
func (c *Catalogs) InstallSoleEquipment() (func(), error) {
	if c.SoleRules == nil {
		return func() {}, nil
	}
	inventory.SetSoleEquipmentRules(c.SoleRules)
	return func() { inventory.SetSoleEquipmentRules(nil) }, nil
}
