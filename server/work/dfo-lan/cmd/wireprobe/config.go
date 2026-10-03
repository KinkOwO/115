package main

import (
	"dfolan/internal/gamedata"
	"flag"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
)

// Config is the gateway's startup configuration. Tags declare each existing
// flag, default, environment alias and help text once. The launcher still owns
// profile files; game content is prepared separately from the native PVF.
//
// envmode preserves the legacy DFO contract: booleans accept only "1", except
// not-zero options which are disabled only by "0". byte bounds apply only to
// environment defaults; explicit flags retain the standard flag.Int behavior.
type Config struct {
	MoonSoloConfig                string `koanf:"moon-solo-config" env:"DFO_MOON_SOLO_CONFIG" help:"opt-in Moon Lake solo candidate, explicitly validated 2.38.3.25 profile"`
	Fixture                       string `koanf:"fixture" help:"verified-format server fixture to send after accept"`
	Output                        string `koanf:"output" default:"runtime/wireprobe" help:"capture directory"`
	GameListen                    string `koanf:"game-listen" default:"127.0.0.1:0" help:"game endpoint; use 0.0.0.0:PORT to accept clients from other machines"`
	AdvertiseHost                 string `koanf:"advertise-host" env:"DFO_ADVERTISE_HOST" help:"host the client dials for the game and channel directory; empty reuses the bound address, or auto-detects the LAN IPv4 when game-listen is a wildcard"`
	Responses                     string `koanf:"responses" help:"JSON mapping command IDs to response fixture paths"`
	CharacterStorage              string `koanf:"character-storage" help:"enable experimental persisted character handling with this local storage config"`
	CharacterCatalog              string `koanf:"character-catalog" default:"configs/characters.generated.json" help:"PVF-derived profession catalog"`
	PVFCatalogs                   string `koanf:"pvf-catalogs" env:"DFO_PVF_CATALOGS" help:"candidate direct-read domains: {pvf-domains}; empty keeps JSON"`
	PVFCheckCatalogs              bool   `koanf:"pvf-check-catalogs" default:"false" help:"prepare explicitly selected PVF catalogs and exit before storage, listeners or runtime setup"`
	PVFCheckHeapProfile           string `koanf:"pvf-check-heap-profile" help:"write a new post-GC heap profile only with pvf-check-catalogs"`
	PVFVerifyBaselines            bool   `koanf:"pvf-verify-baselines" default:"false" env:"DFO_PVF_VERIFY_BASELINES" help:"compare selected PVF domains with JSON baselines before storage; false removes the selected JSON startup dependency"`
	PVFEnhancementPolicy          string `koanf:"pvf-enhancement-policy" default:"configs/pvf-enhancement-policy.json" env:"DFO_PVF_ENHANCEMENT_POLICY" help:"independent enhancement server policies; required only for PVF enhancements"`
	PVFVaultPolicy                string `koanf:"pvf-vault-policy" default:"configs/pvf-vault-policy.json" env:"DFO_PVF_VAULT_POLICY" help:"client capacity/save policy independent of PVF account vault table"`
	PVFContentPolicy              string `koanf:"pvf-content-policy" default:"configs/pvf-content-policy.json" env:"DFO_PVF_CONTENT_POLICY" help:"independent enabled special-content selection; source tables from PVF"`
	PVFSelectionPolicy            string `koanf:"pvf-selection-policy" default:"configs/pvf-selection-policy.json" env:"DFO_PVF_SELECTION_POLICY" help:"bounded selection box templates; source categories from PVF"`
	PVFItemShopPolicy             string `koanf:"pvf-item-shop-policy" default:"configs/pvf-item-shop-policy.json" env:"DFO_PVF_ITEM_SHOP_POLICY" help:"existing service shop routes and purchase-limit policy; offers from native SHP"`
	PVFBoxPolicy                  string `koanf:"pvf-box-policy" default:"configs/pvf-box-policy.json" env:"DFO_PVF_BOX_POLICY" help:"enabled COS/box scope and existing placement defaults; rewards from native material bindings"`
	PVFCharacterPolicy            string `koanf:"pvf-character-policy" default:"configs/pvf-character-policy.json" env:"DFO_PVF_CHARACTER_POLICY" help:"saved source identity and default shortcut behavior; profession source fields from PVF"`
	PVFLayerRevisitPolicy         string `koanf:"pvf-layer-revisit-policy" default:"configs/pvf-layer-revisit-policy.json" env:"DFO_PVF_LAYER_REVISIT_POLICY" help:"verified layer revisit scope, record and cache restoration; source maps and landing from PVF"`
	PVFScriptWarpPolicy           string `koanf:"pvf-script-warp-policy" default:"configs/pvf-script-warp-policy.json" env:"DFO_PVF_SCRIPT_WARP_POLICY" help:"verified script warp scope and witnessed transition records; source routes from PVF"`
	PVFLotteryPolicy              string `koanf:"pvf-lottery-policy" help:"deprecated compatibility flag; PVF lottery scope is discovered from source and this path is ignored"`
	PVFScenePolicy                string `koanf:"pvf-scene-policy" default:"configs/pvf-scene-policy.json" env:"DFO_PVF_SCENE_POLICY" help:"independent entry town and training dungeon selection; source rules from PVF"`
	PVFDropPolicy                 string `koanf:"pvf-drop-policy" default:"configs/pvf-drop-policy.json" env:"DFO_PVF_DROP_POLICY" help:"existing basic equipment allowlist and maximum loot grade; source rules from PVF"`
	PVFCacheDir                   string `koanf:"pvf-cache-dir" default:"runtime/pvf-cache" env:"DFO_PVF_CACHE_DIR" help:"derived PVF cache directory; - disables caching"`
	PVFArchive                    string `koanf:"pvf-archive" env:"DFO_PVF_ARCHIVE" help:"explicit inner PVF path for candidate domains"`
	PVFSHA256                     string `koanf:"pvf-sha256" env:"DFO_PVF_SHA256" help:"expected inner PVF SHA256; must match existing character source"`
	CharacterRules                string `koanf:"character-rules" default:"configs/character-probe.json" help:"explicit local bootstrap settings"`
	SelectProbeConfig             string `koanf:"select-probe-config" help:"opt in to current-build SELECT parser experiment; does not initialize a town"`
	EntryBasicProbe               bool   `koanf:"entry-basic-probe" default:"false" help:"send experimental current-build minimum actor info after SELECT; does not initialize a town"`
	TownCatalog                   string `koanf:"town-catalog" help:"PVF-derived town-area catalog for the entry experiment"`
	TownEntryProbe                string `koanf:"town-entry-probe" help:"opt in to experimental town entry using this separate spawn policy"`
	WorldCatalog                  string `koanf:"world-catalog" help:"enable source-backed town transitions and saved positions"`
	WorldRules                    string `koanf:"world-rules" default:"configs/world-probe.json" help:"separate world movement policy"`
	EntryAdditionProbe            bool   `koanf:"entry-addition-probe" default:"false" help:"send current-build source attributes; optional inventory and skills remain pending"`
	QuestCatalog                  string `koanf:"quest-catalog" help:"enable source quest accept/abandon persistence; objectives and rewards are separate"`
	VaultRules                    string `koanf:"vault-rules" help:"source vault capacity and empty-state initialization"`
	SoleQualityNative             bool   `koanf:"sole-quality-native" default:"false" env:"DFO_SOLE_QUALITY_NATIVE" help:"秘宝精度按原版（国服）结算；默认关（单机口径 5..20）"`
	FatigueRules                  string `koanf:"fatigue-rules" default:"configs/fatigue-probe.json" help:"separate persisted fatigue and rollover policy (default: configs/fatigue-probe.json)"`
	FatigueFree                   bool   `koanf:"fatigue-free" default:"false" env:"DFO_FATIGUE_FREE" help:"关闭疲劳消耗（进本与房间一起归零）；默认关"`
	ProgressionCatalog            string `koanf:"progression-catalog" help:"current-source experience and growth catalog"`
	ProgressionRules              string `koanf:"progression-rules" default:"configs/experience.compat90.json" help:"separate reference compatibility formula settings"`
	LootCatalog                   string `koanf:"loot-catalog" help:"current gold/ordinary stackable source projection; equipment pending"`
	LootRules                     string `koanf:"loot-rules" default:"configs/drop.compat90.json" help:"explicit reference drop formula policy"`
	EquipmentCatalog              string `koanf:"equipment-catalog" env:"DFO_EQUIPMENT_CATALOG" help:"source equipment catalog a run selects gear from; required whenever loot is enabled"`
	EquipmentJournalRules         string `koanf:"equipment-journal-rules" env:"DFO_EQUIPMENT_JOURNAL_RULES" help:"装备库规则表（equipmentsetjournal.cos 的导出物）；留空则不登记"`
	EquipmentCreateCost           string `koanf:"equipment-create-cost" env:"DFO_EQUIPMENT_CREATE_COST" help:"装备生成成本表（[create cost] 的导出物）；留空则第二步只回窗口"`
	EquipmentCraftWindow          int    `koanf:"equipment-craft-window" default:"1" env:"DFO_EQUIPMENT_CRAFT_WINDOW" envmode:"byte" help:"CMD2259 应答的窗口选择字节（非 0 → 窗口 3937；0 → 窗口 2145）"`
	EquipmentCraftVariant         int    `koanf:"equipment-craft-variant" default:"0" env:"DFO_EQUIPMENT_CRAFT_VARIANT" envmode:"byte" help:"CMD2259 应答的子分支字节（仅当窗口字节为 0 时生效）"`
	EquipmentCraftConfirmWindow   int    `koanf:"equipment-craft-confirm-window" default:"0" env:"DFO_EQUIPMENT_CRAFT_CONFIRM_WINDOW" envmode:"byte" help:"CMD2259 第二步（确定）应答的窗口选择字节"`
	EquipmentCraftConfirmVariant  int    `koanf:"equipment-craft-confirm-variant" default:"0" env:"DFO_EQUIPMENT_CRAFT_CONFIRM_VARIANT" envmode:"byte" help:"CMD2259 第二步应答的子分支字节"`
	EquipmentCraftGenerateWindow  int    `koanf:"equipment-craft-generate-window" default:"0" env:"DFO_EQUIPMENT_CRAFT_GENERATE_WINDOW" envmode:"byte" help:"CMD2259 装备生成（[12]=0）应答的窗口选择字节"`
	EquipmentCraftGenerateVariant int    `koanf:"equipment-craft-generate-variant" default:"1" env:"DFO_EQUIPMENT_CRAFT_GENERATE_VARIANT" envmode:"byte" help:"CMD2259 装备生成应答的子分支字节（1 = 只落成功标志、不动窗口状态，默认；0 = 强制 setState 到状态 3，会让材料切换按钮失灵）"`
	EquipmentCraftExecute         bool   `koanf:"equipment-craft-execute" default:"true" env:"DFO_EQUIPMENT_CRAFT_EXECUTE" envmode:"not-zero" help:"CMD2259 是否执行装备生成（扣成本 + 发装备）"`
	EquipmentCraftExecuteOn       string `koanf:"equipment-craft-execute-on" default:"confirm" env:"DFO_EQUIPMENT_CRAFT_EXECUTE_ON" help:"CMD2259 何时执行装备生成：confirm / first / never"`
	EquipmentTransform            string `koanf:"equipment-transform" default:"apply" env:"DFO_EQUIPMENT_TRANSFORM_APPLY" help:"CMD2259 action=1（装备变换）如何执行：apply（真的换装）/ observe（只记日志）"`
	BagRules                      string `koanf:"bag-rules" default:"configs/inventory.compat90.json" help:"separate bag slot and missing stack limit policy"`
	Boxes                         string `koanf:"boxes" help:"imported open-box content tables; empty resolves boxes.json beside the bag rules"`
	CardRules                     string `koanf:"card-rules" default:"configs/cards.compat90.json" help:"separate compatible free-card policy"`
	SkillCatalog                  string `koanf:"skill-catalog" help:"current PVF learning metadata; enables manual learning and persisted skill slots"`
	ChannelRefreshConfig          string `koanf:"channel-refresh-config" help:"separate local channel directory service for native refresh"`
	ChannelIdentity               bool   `koanf:"channel-identity" default:"false" help:"candidate: synchronize NOTI2435 and all actor contexts with the connected channel"`
	QuestEquipmentCatalog         string `koanf:"quest-equipment-catalog" help:"source basic-equipment metadata for atomic quest rewards"`
	EquipmentWearRules            string `koanf:"equipment-wear-rules" help:"current-client equipment slots and persistent wear handling"`
	KnightShieldCatalog           string `koanf:"knight-shield-catalog" default:"equipment-knight-shield.full-candidate.json" help:"optional source-verified shield window side-car; relative to wear rules directory, empty disables"`
	EquipmentFullCatalog          string `koanf:"equipment-full-catalog" env:"DFO_EQUIPMENT_FULL_CATALOG" help:"separate indexed wear catalog prefix; does not widen drops"`
	ItemIndex                     string `koanf:"item-index" env:"DFO_ITEM_INDEX" help:"full stackable item index JSON (e.g. configs/items.index.json)"`
	BoosterCatalog                string `koanf:"booster-catalog" env:"DFO_BOOSTER_CATALOG" help:"booster definitions JSON"`
	SelectionBoxes                string `koanf:"selection-boxes" env:"DFO_SELECTION_BOXES" help:"source selection box JSON ([booster select category] boxes)"`
	ItemShop                      string `koanf:"item-shop" env:"DFO_ITEM_SHOP" help:"source item shop JSON (itemshop/**.shp; prices goods with [need material], e.g. the Odyssey shop's silver coins)"`
	ShopPrices                    string `koanf:"shop-prices" env:"DFO_SHOP_PRICES" help:"source NPC prices; empty resolves shop-prices.json beside the loot catalog"`
	BleedingMineRewards           string `koanf:"bleeding-mine-rewards" help:"赤红铁矿原版奖励表；默认读取掉落目录旁的 bleeding-mine-rewards.json"`
	SoloPartyBootstrap            bool   `koanf:"solo-party-bootstrap" default:"false" help:"initialize the owned actor in the current solo party roster"`
	AccountOptions                string `koanf:"account-options" help:"sparse current-client account option overrides; other defaults remain client-owned"`
	UnifiedCharacTemplate         string `koanf:"unified-charac-template" help:"override the built-in 3539 byte character option block sent as NOTI2827 (different client build only)"`
	SkillLockOffset               int    `koanf:"skill-lock-offset" default:"-1" help:"override the subtype 19 skill lock offset inside the character option block (default 2736)"`
	TutorialRoutes                string `koanf:"tutorial-routes" help:"source per-job starting route table"`
	TutorialDungeons              string `koanf:"tutorial-dungeons" help:"source starting-route dungeon catalog"`
	ShopRelease                   bool   `koanf:"shop-release" default:"false" env:"DFO_SHOP_RELEASE" help:"enable accepted ordinary shop in release profile"`
	VaultPurchaseCandidate        bool   `koanf:"vault-purchase-candidate" default:"false" env:"DFO_VAULT_PURCHASE_CANDIDATE" help:"enable isolated vault purchase candidate"`
	VaultPurchaseRelease          bool   `koanf:"vault-purchase-release" default:"false" env:"DFO_VAULT_PURCHASE_RELEASE" help:"enable accepted personal vault purchases in release profile"`
	RandomOptionCatalog           string `koanf:"random-option-catalog" env:"DFO_RANDOM_OPTION_CATALOG" help:"current-client magic-seal random option rules; enables CMD393 unsealing"`
	ApocalypseCatalog             string `koanf:"apocalypse-catalog" default:"configs/apocalypse.generated.json" help:"compiled apocalypse.ctp table (phase clock, operations, gates, rewards, duty skills)"`
	AttunementRewards             string `koanf:"attunement-rewards" env:"DFO_ATTUNEMENT_REWARDS" help:"boundary-of-attunement reward table generated from the source rewardboostinfo CTPs"`
	AttunementRebalance           bool   `koanf:"attunement-rebalance" default:"false" env:"DFO_ATTUNEMENT_REBALANCE" help:"本私服的掉落调参（**与官服的显式差异**）：征兆「无事发生」减半、fixed 池低档按比例向高档倾斜。见 internal/loot/attunement_rebalance.go"`
	AttunementFixedTilt           int    `koanf:"attunement-fixed-tilt" default:"25" env:"DFO_ATTUNEMENT_FIXED_TILT" help:"固定池倾斜幅度：普通/稀有各减这么多百分比权重，减掉的按高档现有比例补（1..99）。0 = 不动固定池；只在 -attunement-rebalance 打开时生效"`
	BoosterGageHide               bool   `koanf:"booster-gage-hide" default:"true" env:"DFO_BOOSTER_GAGE" envmode:"not-zero" help:"send NOTI398 booster-gage with displayValue=0 on town entry to hide the top-left Liberation Trace panel; disable with -booster-gage-hide=false or DFO_BOOSTER_GAGE=0"`
	OathGrades                    string `koanf:"oath-grades" env:"DFO_OATH_GRADES" help:"诊断覆盖：固定下发的引子/誓约档位 primer,oath（见 oath_info.go）。留空 = 按角色穿戴的誓约/引子装备算，这是正常路径"`
	OathGradesTable               string `koanf:"oath-grades-table" env:"DFO_OATH_GRADES_TABLE" help:"誓约/引子装备稀有度表（cmd/oathgradeimport 生成）；只在 -oath-grades-from-gear 打开时用"`
	OathGradesFromGear            bool   `koanf:"oath-grades-from-gear" default:"false" env:"DFO_OATH_GRADES_FROM_GEAR" help:"诊断：按角色穿戴的誓约/引子装备算档位（旧规则）。默认关 —— 客户端脱不下誓约槽，穿上 primeval 就永久 oath=45"`
	OathProgressClears            int    `koanf:"oath-progress-clears" env:"DFO_OATH_PROGRESS_CLEARS" help:"隐藏 BOSS 的保底场次：-oath-progress-dungeons 里的副本通关这么多场后，下一场下发 oath=45（必出一次）并在通关时归零；<=0 关闭保底"`
	OathProgressDungeons          string `koanf:"oath-progress-dungeons" env:"DFO_OATH_PROGRESS_DUNGEONS" help:"计入保底的副本号，逗号分隔（默认只有小深渊 100005014）"`
	OathInject                    string `koanf:"oath-inject" env:"DFO_OATH_INJECT" help:"诊断用：向客户端注入任意 noti 的候选列表，形式 id:size:fill;off:val,...（见 oath_probe.go）；默认空 = 关闭"`
	OmenHold                      int    `koanf:"omen-hold" default:"-1" env:"DFO_OMEN_HOLD" help:"诊断：把玩家直接放到指定征兆阶段(0-4)，-1 = 不动；会写回角色存档"`
	OmenInfo                      string `koanf:"omen-info" env:"DFO_OMEN_INFO" help:"诊断：直接指定 noti 2836「征兆队伍状态」的 69 字节载荷，用来点亮征兆 UI 并实测字段语义。写法见 cmd/wireprobe/omen_info.go；留空 = 按角色存档里的真实档数生成"`
	ScaleDeathFromHP              bool   `koanf:"scale-death-from-hp" default:"false" env:"DFO_SCALE_DEATH_FROM_HP" help:"诊断：定盘机关血量触底时由服务端兜底宣布死亡（默认关；noti 2838 修好后天平会自己死）"`
}

// loadConfig has no file, catalog, storage, listener or process side effects.
// Explicit flags (including empty strings and false) win over DFO environment
// defaults. Each call uses a private FlagSet and Koanf instance.
func loadConfig(args []string, getenv func(string) string, output io.Writer) (Config, error) {
	cfg := Config{
		OathProgressClears:   oathDefaultProgressClears,
		OathProgressDungeons: oathDefaultProgressDungeons,
	}
	fs := flag.NewFlagSet("wireprobe", flag.ContinueOnError)
	fs.SetOutput(output)
	defaults, environment := make(map[string]any), make(map[string]any)
	initial := reflect.ValueOf(cfg)
	for i := 0; i < initial.NumField(); i++ {
		field := initial.Type().Field(i)
		key := field.Tag.Get("koanf")
		value := initial.Field(i).Interface()
		if text, ok := field.Tag.Lookup("default"); ok {
			var err error
			value, err = configValue(field.Type.Kind(), text)
			if err != nil {
				return Config{}, fmt.Errorf("default for %s: %w", key, err)
			}
		}
		defaults[key] = value
		if alias := field.Tag.Get("env"); alias != "" {
			if text := getenv(alias); text != "" {
				value = configEnvironment(field, text, value)
				environment[key] = value
			}
		}
		help := strings.ReplaceAll(field.Tag.Get("help"), "{pvf-domains}", gamedata.SupportedDomains)
		switch value := value.(type) {
		case string:
			fs.String(key, value, help)
		case bool:
			fs.Bool(key, value, help)
		case int:
			fs.Int(key, value, help)
		default:
			return Config{}, fmt.Errorf("unsupported config field %s", field.Name)
		}
	}
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	explicit := make(map[string]any)
	fs.Visit(func(f *flag.Flag) { explicit[f.Name] = f.Value.(flag.Getter).Get() })
	k := koanf.NewWithConf(koanf.Conf{Delim: ".", StrictMerge: true})
	for _, layer := range []map[string]any{defaults, environment, explicit} {
		if err := k.Load(confmap.Provider(layer, "."), nil); err != nil {
			return Config{}, fmt.Errorf("merge startup config: %w", err)
		}
	}
	if err := k.Unmarshal("", &cfg); err != nil {
		return Config{}, fmt.Errorf("decode startup config: %w", err)
	}
	return cfg, nil
}

func configValue(kind reflect.Kind, text string) (any, error) {
	switch kind {
	case reflect.String:
		return text, nil
	case reflect.Bool:
		return strconv.ParseBool(text)
	case reflect.Int:
		return strconv.Atoi(text)
	default:
		return nil, fmt.Errorf("unsupported config type %s", kind)
	}
}

func configEnvironment(field reflect.StructField, text string, fallback any) any {
	switch field.Type.Kind() {
	case reflect.String:
		return text
	case reflect.Bool:
		if field.Tag.Get("envmode") == "not-zero" {
			return text != "0"
		}
		return text == "1"
	case reflect.Int:
		n, err := strconv.Atoi(text)
		if err != nil || field.Tag.Get("envmode") == "byte" && (n < 0 || n > 255) {
			return fallback
		}
		return n
	default:
		return fallback
	}
}

func (c Config) validate() error {
	if c.PVFCheckHeapProfile != "" && !c.PVFCheckCatalogs {
		return fmt.Errorf("pvf-check-heap-profile requires pvf-check-catalogs")
	}
	if c.PVFCheckCatalogs && strings.TrimSpace(c.PVFCatalogs) == "" {
		return fmt.Errorf("pvf-check-catalogs requires explicit pvf-catalogs")
	}
	return nil
}
