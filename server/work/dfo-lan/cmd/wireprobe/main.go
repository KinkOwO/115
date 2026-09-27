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

func main() {
	fixture := flag.String("fixture", "", "verified-format server fixture to send after accept")
	dir := flag.String("output", "runtime/wireprobe", "capture directory")
	gameListen := flag.String("game-listen", "127.0.0.1:0", "game endpoint; use 0.0.0.0:PORT to accept clients from other machines")
	advertiseHost := flag.String("advertise-host", os.Getenv("DFO_ADVERTISE_HOST"), "host the client dials for the game and channel directory; empty reuses the bound address, or auto-detects the LAN IPv4 when game-listen is a wildcard")
	responseFile := flag.String("responses", "", "JSON mapping command IDs to response fixture paths")
	characterStorage := flag.String("character-storage", "", "enable experimental persisted character handling with this local storage config")
	characterCatalog := flag.String("character-catalog", "configs/characters.generated.json", "PVF-derived profession catalog")
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
	bagRulesFile := flag.String("bag-rules", "configs/inventory.compat90.json", "separate bag slot and missing stack limit policy")
	boxesFile := flag.String("boxes", "", "imported open-box content tables; empty resolves boxes.json beside the bag rules")
	cardRulesFile := flag.String("card-rules", "configs/cards.compat90.json", "separate compatible free-card policy")
	learningFile := flag.String("skill-catalog", "", "current PVF learning metadata; enables manual learning and persisted skill slots")
	channelRefreshFile := flag.String("channel-refresh-config", "", "separate local channel directory service for native refresh")
	channelIdentityEnabled := flag.Bool("channel-identity", false, "candidate: synchronize NOTI2435 and all actor contexts with the connected channel")
	equipmentRewardFile := flag.String("quest-equipment-catalog", "", "source basic-equipment metadata for atomic quest rewards")
	wearRulesFile := flag.String("equipment-wear-rules", "", "current-client equipment slots and persistent wear handling")
	fullEquipmentFile := flag.String("equipment-full-catalog", os.Getenv("DFO_EQUIPMENT_FULL_CATALOG"), "separate indexed wear catalog prefix; does not widen drops")
	itemIndexFile := flag.String("item-index", os.Getenv("DFO_ITEM_INDEX"), "full stackable item index JSON (e.g. configs/items.index.json)")
	boosterCatalogFile := flag.String("booster-catalog", os.Getenv("DFO_BOOSTER_CATALOG"), "booster definitions JSON")
	selectionBoxFile := flag.String("selection-boxes", os.Getenv("DFO_SELECTION_BOXES"), "source selection box JSON ([booster select category] boxes)")
	itemShopFile := flag.String("item-shop", os.Getenv("DFO_ITEM_SHOP"), "source item shop JSON (itemshop/**.shp; prices goods with [need material], e.g. the Odyssey shop's silver coins)")
	shopPricesFile := flag.String("shop-prices", os.Getenv("DFO_SHOP_PRICES"), "source NPC prices; empty resolves shop-prices.json beside the loot catalog")
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
	oathGradePair, oathGradesErr := parseOathGrades(*oathGrades)
	if oathGradesErr != nil {
		log.Fatalf("bad -oath-grades: %v", oathGradesErr)
	}
	// 档位表只服务「按穿戴装备算档位」这条诊断路径（-oath-grades-from-gear）。
	// 默认的保底路径不需要它，所以默认配置下**不加载、也不会因为缺表拒绝启动**。
	var oathGradeTable *inventory.OathGradeTable
	if *oathFromGear {
		table, tableErr := loadOathGradeTable(*oathGradesTable)
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
	if *randomOptionFile == "" {
		if _, err := os.Stat("configs/randomoption.current37.json"); err == nil {
			*randomOptionFile = "configs/randomoption.current37.json"
		}
	}
	if *shopPilotFile == "" {
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
	if *itemShopFile == "" {
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
		if !*entryBasicProbe || *townCatalogFile == "" {
			log.Fatal("town probe requires basic actor and town catalog")
		}
		var e error
		townCatalog, e = catalog.LoadTownArea(*townCatalogFile)
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
	var vaultService *inventory.VaultService
	var fatigueService *character.FatigueService
	var developmentAccount int64
	var dungeonCatalog *catalog.DungeonCatalog
	var progressionService *character.ProgressionService
	var lootService *loot.Service
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
		if e = s.MigrateTutorial(ctx); e != nil {
			log.Fatal(e)
		}
		// Account/character unified options (CMD2377 0x01/0x05) persist here;
		// the account block restores through NOTI2826, character settings are
		// stored until the NOTI2827 layout is reversed.
		if e = s.MigrateUnifiedOptions(ctx); e != nil {
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
		if e = s.MigrateMailbox(ctx); e != nil {
			log.Fatal(e)
		}
		// Per-(character,dungeon) hidden-boss pity counter. The client's tier
		// ladder has no roll, so this table is the only place "rare" can live.
		if e = s.MigrateOathProgress(ctx); e != nil {
			log.Fatal(e)
		}
		// Per-(character,dungeon) omen save slot. The omen is not an item: it is a
		// character-save marker the client reads out of NOTI2836 (see omen_state.go).
		if e = s.MigrateOmenState(ctx); e != nil {
			log.Fatal(e)
		}
		data, e := catalog.LoadCharacters(*characterCatalog)
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
		if os.Getenv("DFO_MAX_ITEM_PERIOD") == "1" {
			if *itemIndexFile == "" {
				log.Fatal("DFO_MAX_ITEM_PERIOD requires -item-index")
			}
			periodFile := filepath.Join(filepath.Dir(*itemIndexFile), "item-period-tags.json")
			templates, periodErr := catalog.LoadItemPeriods(periodFile, data.Source.Checksum)
			if periodErr != nil {
				log.Fatalf("DFO_MAX_ITEM_PERIOD: %v", periodErr)
			}
			protocol.ConfigureMaxItemPeriods(templates)
			log.Printf("maximum item period enabled for %d PVF templates", len(templates))
		}
		// Skin-cargo registration (CMD507 action 169, `[add skin storage]`) reads
		// the skin key straight from PVF and persists the unlock per account.
		if *itemIndexFile != "" {
			skinFile := filepath.Join(filepath.Dir(*itemIndexFile), "skin-storage-items.json")
			entries, skinErr := catalog.LoadSkinStorage(skinFile, data.Source.Checksum)
			if skinErr != nil {
				log.Printf("skin storage registration disabled: %v", skinErr)
			} else if e = s.MigrateSkinCargo(ctx); e != nil {
				log.Fatal(e)
			} else if e = s.MigrateSkinSelection(ctx); e != nil {
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
		if *shopPilotFile != "" {
			var database string
			if e = s.DB.QueryRow(ctx, "SELECT current_database()").Scan(&database); e == nil {
				if database != "dfo_swordmaster_pilot_20260916" && !*shopRelease {
					log.Printf("shop purchase pilot running on database: %s", database)
				}
			}
			shopPilot, e = cashshop.LoadPilot(*shopPilotFile, data.Source.Checksum, *shopRelease)
			if e != nil {
				log.Fatal(e)
			}
			if e = s.MigrateCashShop(ctx); e != nil {
				log.Fatal(e)
			}
			log.Printf("PVF shop enabled: %d ordinary products", shopPilot.EnabledCount())
			log.Printf("商城配置：%s，发布模式：%t", *shopPilotFile, shopPilot.Config.Release)
		}
		if *learningFile != "" {
			characters.Learning, e = character.LoadLearningCatalog(*learningFile, data.Source.Checksum)
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
	if *worldCatalogFile != "" {
		if characters == nil || *townProbeFile == "" {
			log.Fatal("world requires persisted characters and a spawn policy")
		}
		data, e := catalog.LoadWorld(*worldCatalogFile)
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
	if *dungeonCatalogFile != "" {
		if candidate := os.Getenv("DFO_ODYSSEY_DUNGEON_CATALOG"); candidate != "" {
			*dungeonCatalogFile = candidate
		}
		if worldService == nil {
			log.Fatal("dungeons require world sessions")
		}
		data, e := catalog.LoadDungeons(*dungeonCatalogFile)
		if e != nil {
			log.Fatal(e)
		}
		trainingRoomPath := os.Getenv("DFO_TRAINING_ROOM_CATALOG")
		if trainingRoomPath == "" {
			trainingRoomPath = filepath.Join(filepath.Dir(*dungeonCatalogFile), "dungeons.training-room.json")
		}
		trainingRooms, e := catalog.LoadDungeons(trainingRoomPath)
		if e != nil {
			log.Fatal(e)
		}
		if e = catalog.MergeDungeonCatalog(&data, trainingRooms); e != nil {
			log.Fatal(e)
		}
		if filepath.Base(*dungeonCatalogFile) == "dungeons.full.json" {
			path := filepath.Join(filepath.Dir(*dungeonCatalogFile), "dungeons.tournament-quest-maps.json")
			if e = catalog.AttachTournamentQuestMaps(&data, path); e != nil {
				log.Fatal(e)
			}
			path = filepath.Join(filepath.Dir(*dungeonCatalogFile), "dungeons.tower-of-dazzlement-maps.json")
			if e = catalog.AttachDazzlementMaps(&data, path); e != nil {
				log.Fatal(e)
			}
			path = filepath.Join(filepath.Dir(*dungeonCatalogFile), "dungeons.maze-chance-rates.json")
			if e = catalog.AttachMazeChanceRates(&data, path); e != nil {
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
	if *progressionCatalogFile != "" {
		if characters == nil || dungeonCatalog == nil {
			log.Fatal("progression requires source characters and dungeon sessions")
		}
		data, e := catalog.LoadProgression(*progressionCatalogFile)
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
		if path := os.Getenv("DFO_ODYSSEY_GROWTH"); path != "" {
			progressionService.Odyssey, e = catalog.LoadOdysseyGrowth(path)
			if e != nil {
				log.Fatal(e)
			}
		}
		if path := os.Getenv("DFO_ODYSSEY_CHAPTERS"); path != "" {
			progressionService.Chapters, e = catalog.LoadOdysseyChapters(path)
			if e != nil {
				log.Fatal(e)
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = characters.Store.MigrateCharacterEvents(ctx)
		if e == nil {
			e = characters.Store.MigrateCharacterNotices(ctx)
		}
		if e == nil {
			e = characters.Store.MigrateSkillLocks(ctx)
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
		if *tutorialDungeonsFile == "" {
			log.Fatal("starting routes require their own dungeon catalog")
		}
		routes, e := catalog.LoadTutorialRoutes(*tutorialRoutesFile, characters.Catalog.Source.Checksum)
		if e != nil {
			log.Fatal(e)
		}
		data, e := catalog.LoadDungeons(*tutorialDungeonsFile)
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
		if err := inventory.LoadReinforcementTickets(filepath.Join(filepath.Dir(lootPath), "reinforcement-tickets.json")); err != nil {
			log.Fatal(err)
		}
		c, e := catalog.LoadLoot(lootPath)
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
		gear, e := inventory.LoadEquipmentCatalog(*equipmentCatalogFile, c.Source.Checksum)
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
		if itemIndexPath != "" {
			if err := c.SupplementStackables(itemIndexPath); err != nil {
				log.Printf("warning: supplement stackables from %s: %v", itemIndexPath, err)
			} else {
				log.Printf("supplemented stackable catalog from %s (total items: %d)", itemIndexPath, len(c.Items))
			}
		}
		lootService = &loot.Service{Store: characters.Store, Catalog: c, DropCatalog: dropCatalog, Rules: r, BagRules: bag, Tables: tables, Equipment: gear}
		pricesPath := *shopPricesFile
		if pricesPath == "" {
			pricesPath = filepath.Join(filepath.Dir(lootPath), "shop-prices.json")
		}
		if _, err := os.Stat(pricesPath); err == nil || *shopPricesFile != "" {
			lootService.Prices, e = catalog.LoadShopPrices(pricesPath, c.Source.Checksum)
			if e != nil {
				log.Fatal(e)
			}
			log.Printf("loaded %d source NPC prices from %s", len(lootService.Prices.Items), pricesPath)
		} else {
			log.Printf("warning: no source NPC prices (%s); gold purchases and sales are refused", pricesPath)
		}
		if path := os.Getenv("DFO_ODYSSEY_COIN_RULES"); path != "" {
			lootService.Currency, e = loot.LoadOdysseyCurrency(path)
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
		if _, statErr := os.Stat(boxesPath); statErr == nil {
			boxes, boxErr := loot.LoadBoxes(boxesPath)
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
	if *questCatalogFile != "" {
		if worldService == nil {
			log.Fatal("quests require world character sessions")
		}
		data, e := catalog.LoadQuests(*questCatalogFile)
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
		if *equipmentRewardFile != "" {
			if lootService == nil {
				log.Fatal("quest inventory requires the shared bag catalog")
			}
			equipment, e := inventory.LoadEquipmentCatalog(*equipmentRewardFile, data.Source.Checksum)
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
				// 创建期的初始装备投影共用同一份装备目录与部位槽映射，避免另立编号。
				characters.Equipment = equipment
				characters.WearRules = rules
				if *fullEquipmentFile != "" {
					full, err := inventory.OpenFullEquipmentCatalog(*fullEquipmentFile, data.Source.Checksum)
					if err != nil {
						log.Fatal(err)
					}
					defer full.Close()
					wearCatalog := *equipment
					wearCatalog.Full = full
					wearService.Catalog = &wearCatalog
					equipment.Full = full
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
			if equipment.Full != nil && *randomOptionFile != "" {
				options, err := inventory.LoadRandomOptionCatalog(*randomOptionFile, data.Source.Checksum)
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
		rules, e := inventory.LoadVaultRules(*vaultRulesFile)
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
		odysseyChoices, e = loadOdysseyWeaponChoices(os.Getenv("DFO_ODYSSEY_WEAPON_BOX"))
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
	if path := os.Getenv("DFO_CLEAR_CUBE_SOURCE"); path != "" && vaultService != nil {
		var e error
		vaultService.Catalog, e = inventory.WithClearCube(vaultService.Catalog, path)
		if e != nil {
			log.Fatal(e)
		}
	}
	var boosterCatalog *BoosterCatalog
	if *boosterCatalogFile != "" || *itemIndexFile != "" {
		var err error
		boosterCatalog, err = LoadBoosterCatalog(*boosterCatalogFile, *itemIndexFile)
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
	if boosterCatalog != nil && *itemIndexFile != "" {
		lotteryPath := filepath.Join(filepath.Dir(*itemIndexFile), "lottery-item-pools.json")
		var err error
		lotteryPools, err = loadLotteryItemCatalog(lotteryPath, boosterCatalog.Items)
		if err != nil {
			log.Printf("warning: lottery item catalog disabled: %v", err)
		} else {
			log.Printf("loaded lottery item catalog (%d verified pools)", len(lotteryPools.Pools))
			if wearService != nil && wearService.Catalog != nil {
				equipmentPath := filepath.Join(filepath.Dir(*itemIndexFile), "lottery-equipment-pools.json")
				if count, loadErr := loadLotteryEquipmentPools(equipmentPath, boosterCatalog.Items, lotteryPools); loadErr != nil {
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
	if *selectionBoxFile != "" {
		var err error
		selectionBoxes, err = catalog.LoadSelectionBoxes(*selectionBoxFile)
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
	if *apocalypseCatalogFile != "" {
		loaded, err := catalog.LoadApocalypseCatalog(*apocalypseCatalogFile)
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
	if *itemShopFile != "" {
		var err error
		itemShops, err = catalog.LoadItemShops(*itemShopFile)
		if err != nil {
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
	if path := os.Getenv("DFO_ODYSSEY_CHAPTER_DROP"); path != "" {
		chapterDrop, e := loot.LoadOdysseyChapterDrop(path)
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
	if *attunementRewardsFile != "" {
		attunement, e := loot.LoadAttunementRewards(*attunementRewardsFile)
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
	raw, err := os.ReadFile(*fixture)
	if err != nil {
		log.Fatal(err)
	}
	if err = wire.ValidateServer(raw); err != nil {
		log.Fatal(err)
	}
	hub := newLanHub()
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
		if *channelIdentityEnabled {
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
		var skillState skillSession
		var cubeContractState cubeContractSession
		var equipmentState equipmentSession
		var sortState sortSession
		var legionState legionSession
		legionState.catalog = apocalypseCatalog
		legionState.clock = apocalypseClock
		legionState.channelType = channelTypes[channel]
		if worldService != nil {
			worldState = &worldSession{characters: characters, service: worldService, account: developmentAccount, flags: townPolicy.Flags, dungeons: dungeonCatalog, tutorials: tutorialRoutes, tutorialDungeons: tutorialDungeons, professions: characters.Catalog, fatigue: fatigueService, quests: questService, progression: progressionService, loot: lootService, selectionBoxes: selectionBoxes, vault: vaultService, skinCatalog: skinCatalog, soloPartyBootstrap: *soloPartyBootstrap, hub: hub, scaleDeathFromHP: *scaleDeathFromHP, oathGrades: oathGradePair, oathTable: oathGradeTable, oathFromGear: *oathFromGear, oathProgressClears: *oathProgressClears, oathProgressDungeons: oathProgressSet, oathInject: oathInjectSpecs, omenHold: *omenHold, omenState: *omenState, omenInfo: omenInfoBytes}
			worldState.serverID = channelCfg.ServerID
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
		for {
			var incoming clientRead
			select {
			case incoming = <-frames:
			case <-mailTicker.C:
				select {
				case mailChanges <- struct{}{}:
				default:
				}
				continue
			case <-mailChanges:
				if bootstrapped && selectedCharacterID != 0 && worldState != nil && worldState.characters != nil {
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
				plan, favorErr := worldState.giveFavor(plaintext)
				if favorErr != nil {
					event(map[string]any{"kind": "npc_favor_refused", "attempt": "1/3", "character_id": selectedCharacterID, "reason": favorErr.Error(), "plain_hex": hex.EncodeToString(plaintext)})
					if e := sendPayload(1, 806, protocol.Refusal(4)); e != nil {
						return
					}
					continue
				}
				for _, packet := range plan {
					if e := sendPayload(packet.Kind, packet.ID, packet.Payload); e != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "attempt": "1/3", "character_id": selectedCharacterID, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
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
			if frame.ID == 19 && bootstrapped && verified && wearService != nil {
				plan, e := equipmentState.handle(wearService, worldState, plaintext, frame.Raw)
				if e != nil {
					event(map[string]any{"kind": "equipment_move_refused", "reason": e.Error()})
					r, _ := protocol.DecodeItemMove(plaintext)
					if e = sendPayload(1, 19, protocol.ItemMoveRefused(r)); e != nil {
						return
					}
					continue
				}
				r, decodeErr := protocol.DecodeItemMove(plaintext)
				// Ordinary worn-set moves already append AppearanceProbe and
				// WornSpaceUpdate in equipmentSession.handle. Re-sending an entry
				// user-info and the same worn-window refresh here rebuilds the actor
				// twice mid-dungeon and can strand the client before its next room
				// request. Keep the separate actor refresh only for slot-26 moves
				// that did not pass through the worn list.
				if decodeErr == nil && characters != nil && (r.SourceSlot == 26 || r.DestinationSlot == 26) && r.SourceList != 3 && r.DestinationList != 3 {
					var visual []byte
					visual, e = characters.EntryBasicProbe(worldState.role, [2]byte{})
					if e == nil {
						plan = append(plan, outboundPacket{"creature_actor_appearance_updated", 0, 2, visual})
					}
				}
				if decodeErr == nil && characters != nil && cloneAvatarRemoval(r, wearService.Catalog) {
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
				prepared, e := preparePackets(keys, plan)
				if e != nil {
					event(map[string]any{"kind": "equipment_encode_error", "error": e.Error()})
					return
				}
				if e = writePackets(c, prepared, func(p preparedPacket) {
					event(map[string]any{"kind": p.Name, "character_id": worldState.role.ID, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
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
				// subtype 0x13 is the skill lock, 0x05 ordinary settings, and
				// 0x01 the account level block restored by NOTI2826. Skill locks
				// and both setting blocks are persisted here; the character
				// settings block has no reversed NOTI2827 offset yet, so it is
				// stored for a later restore path.
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
				case protocol.UnifiedOptionAccount:
					if characters == nil {
						event(map[string]any{"kind": "account_settings_rejected", "reason": "storage unavailable"})
						continue
					}
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					e = characters.Store.SaveAccountUnifiedOptions(ctx, developmentAccount, unifiedEntries(opt.Entries))
					cancel()
					if e != nil {
						event(map[string]any{"kind": "account_settings_rejected", "reason": e.Error()})
						continue
					}
					event(map[string]any{"kind": "account_settings_saved", "entries": len(opt.Entries)})
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
				// CMD507 is the shared "use stackable" frame. Split it by action so
				// the fatigue potion (54) and `[add skin storage]` (169, damage font)
				// paths never collide; the fatigue path keeps its exact prior shape.
				_, action, actionErr := protocol.DecodeStackableAction(plaintext)
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
				if actionErr == nil && action == protocol.ActionOpenCreatureSkinSlot {
					// 宠物幻化栏扩展券（action 197）。同一个 CMD507 上复用三种动作，
					// 这里只接新增的这一路，54/169 保持各自原有的入口形状。
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
			if worldState != nil && bootstrapped && frame.ID == 80 {
				if !verified {
					event(map[string]any{"kind": "reinforcement_rejected", "reason": "强化请求校验失败"})
					continue
				}
				plan, err := equipmentState.reinforce(wearService, worldState, plaintext, frame.Raw, event)
				if err != nil {
					event(map[string]any{"kind": "reinforcement_refused", "character_id": worldState.role.ID, "reason": err.Error()})
					// 14529B2F0 的失败分支只使用分发器读取的错误码，并清除等待态。
					if err = sendPayload(1, 80, protocol.Refusal(22)); err != nil {
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
			if worldState != nil && bootstrapped && frame.ID == 26 && lootService != nil {
				if !verified {
					event(map[string]any{"kind": "disjoint_rejected", "id": frame.ID, "reason": "checksum failed"})
					continue
				}
				plan, e := worldState.disjointItem(plaintext)
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
				continue
			}
			if worldState != nil && bootstrapped && dungeonRequest(frame.ID) {
				if !verified {
					event(map[string]any{"kind": "dungeon_request_rejected", "id": frame.ID, "reason": "checksum failed"})
					continue
				}
				var plan []outboundPacket
				var pending *dungeon.Session
				var e error
				switch frame.ID {
				case 16:
					pending, plan, e = worldState.selectDungeon(plaintext)
				case 37:
					plan, e = worldState.finishDungeonLoading(plaintext)
				case 38:
					pending, plan, e = worldState.interactDoor(plaintext)
				case 39:
					worldState.completionErr = nil
					plan, e = worldState.monsterDeath(plaintext, event)
				case 2329:
					plan, e = worldState.scaleStatus(plaintext, event)
				case 40:
					plan, e = worldState.playerDeath(plaintext, frame.Raw)
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
				case 42:
					if len(plaintext) != 0 {
						e = fmt.Errorf("unexpected give-up body")
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
					if frame.ID == 39 || frame.ID == 46 || frame.ID == 117 || frame.ID == 132 || frame.ID == 2015 || frame.ID == 2062 {
						continue
					}
					if e = sendPayload(1, frame.ID, protocol.Refusal(4)); e != nil {
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
				if pending != nil {
					if frame.ID == 16 || frame.ID == 72 || frame.ID == 2062 {
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
						worldState.cardLayoutSent = true
					}
					if p.Name == "dungeon_return_users" {
						// Back in town: exchange actor info with everyone standing there.
						if e = worldState.announceSelf(event); e != nil {
							event(map[string]any{"kind": "area_presence_error", "error": e.Error()})
						}
					}
					if (p.Name == "settlement_exit_ack" || p.Name == "dungeon_leave_ack") && pending == nil {
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
					if p.Name == "dungeon_clear_enabled" {
						worldState.completionSent = true
					}
					if p.Name == "dungeon_clear_reward" {
						worldState.resultSent = true
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
				accountOptions := accountOptionsPayload
				if characters != nil {
					optCtx, optCancel := context.WithTimeout(context.Background(), 5*time.Second)
					overrides, optErr := characters.Store.AccountUnifiedOptions(optCtx, developmentAccount)
					optCancel()
					if optErr != nil {
						event(map[string]any{"kind": "account_options_restore_error", "error": optErr.Error()})
					} else if len(overrides) > 0 {
						if accountOptions, optErr = protocol.AccountOptions(overrides); optErr != nil {
							event(map[string]any{"kind": "account_options_restore_error", "error": optErr.Error()})
							accountOptions = accountOptionsPayload
						}
					}
				}
				plan := entryPayloads{Select: payload, Basic: basic, Addition: addition, Vault: vaultPayload, UserArea: userArea, Area: areaPayload, Fatigue: fatiguePayload, AccountOptions: accountOptions}
				plan.SecondaryVault = secondaryVaultPayload
				plan.AccountVault = accountVaultPayload
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
				// 幻化仓库（武器外观页签）容器只在复制时被推过一次，客户端重登即空；
				// 这里按存档重推 NOTI1545。读不出状态只记事件照常进场，仓库空一次
				// 比卡在角色选择界面好。
				plan.SkinCargo, e = skinCargoRestore(role.State)
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
						plan.Peers = append(plan.Peers, o.info)
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
					if p.ID == 13 || p.ID == 36 {
						entry["plain_hex"] = hex.EncodeToString(p.Payload)
					}
					event(entry)
				})
				if e != nil {
					event(map[string]any{"kind": "entry_write_error", "character_id": role.ID, "error": e.Error()})
					return
				}
				selectedCharacterID = role.ID
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
				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if _, err := io.Copy(c, bytes.NewReader(response)); err != nil {
					event(map[string]any{"kind": "write_error", "error": err.Error()})
					return
				}
				event(map[string]any{"kind": "server_response", "peer": peer, "id": frame.ID, "hex": hex.EncodeToString(response)})
				if frame.ID == 1 {
					bootstrapped = true
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
