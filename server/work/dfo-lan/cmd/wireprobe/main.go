// wireprobe is a protocol experiment gateway. It binds the game port on the
// host named by -game-listen, which accepts a wildcard (0.0.0.0:PORT) so
// clients on other machines can reach it, and publishes the dialable address
// through the channel directory via -advertise-host.
package main

import (
	"bytes"
	"context"
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/progression"
	"dfolan/internal/quest"
	"dfolan/internal/storage"
	"dfolan/internal/world"
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
	bagRulesFile := flag.String("bag-rules", "configs/inventory.compat90.json", "separate bag slot and missing stack limit policy")
	cardRulesFile := flag.String("card-rules", "configs/cards.compat90.json", "separate compatible free-card policy")
	learningFile := flag.String("skill-catalog", "", "current PVF learning metadata; enables manual learning and persisted skill slots")
	channelRefreshFile := flag.String("channel-refresh-config", "", "separate local channel directory service for native refresh")
	equipmentRewardFile := flag.String("quest-equipment-catalog", "", "source basic-equipment metadata for atomic quest rewards")
	wearRulesFile := flag.String("equipment-wear-rules", "", "current-client equipment slots and persistent wear handling")
	fullEquipmentFile := flag.String("equipment-full-catalog", os.Getenv("DFO_EQUIPMENT_FULL_CATALOG"), "separate indexed wear catalog prefix; does not widen drops")
	itemIndexFile := flag.String("item-index", os.Getenv("DFO_ITEM_INDEX"), "full stackable item index JSON (e.g. configs/items.index.json)")
	boosterCatalogFile := flag.String("booster-catalog", os.Getenv("DFO_BOOSTER_CATALOG"), "booster definitions JSON")
	soloPartyBootstrap := flag.Bool("solo-party-bootstrap", false, "initialize the owned actor in the current solo party roster")
	accountOptionsFile := flag.String("account-options", "", "sparse current-client account option overrides; other defaults remain client-owned")
	tutorialRoutesFile := flag.String("tutorial-routes", "", "source per-job starting route table")
	tutorialDungeonsFile := flag.String("tutorial-dungeons", "", "source starting-route dungeon catalog")
	shopPilotFile := flag.String("shop-purchase-pilot", os.Getenv("DFO_SHOP_PURCHASE_PILOT"), "isolated single-item cash purchase pilot catalog")
	shopRelease := flag.Bool("shop-release", os.Getenv("DFO_SHOP_RELEASE") == "1", "enable accepted ordinary shop in release profile")
	vaultPurchase := flag.Bool("vault-purchase-candidate", os.Getenv("DFO_VAULT_PURCHASE_CANDIDATE") == "1", "enable isolated vault purchase candidate")
	vaultRelease := flag.Bool("vault-purchase-release", os.Getenv("DFO_VAULT_PURCHASE_RELEASE") == "1", "enable accepted personal vault purchases in release profile")
	flag.Parse()
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
		if e = s.Migrate(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigrateTutorial(ctx); e != nil {
			log.Fatal(e)
		}
		// The account cera ledger backs the balance sent in SELECT.
		if e = s.MigrateGrants(ctx); e != nil {
			log.Fatal(e)
		}
		if e = s.MigratePremiums(ctx); e != nil {
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
		if *shopPilotFile != "" {
			var database string
			if e = s.DB.QueryRow(ctx, "SELECT current_database()").Scan(&database); e == nil {
				if database != "dfo_swordmaster_pilot_20260916" && !*shopRelease {
					log.Printf("shop purchase pilot running on database: %s", database)
				}
			}
			shopPilot, e = cashshop.LoadPilot(*shopPilotFile, data.Source.Checksum)
			if e != nil {
				log.Fatal(e)
			}
			if e = s.MigrateCashShop(ctx); e != nil {
				log.Fatal(e)
			}
			log.Printf("PVF shop enabled: %d ordinary products", shopPilot.EnabledCount())
		}
		if *learningFile != "" {
			characters.Learning, e = character.LoadLearningCatalog(*learningFile, data.Source.Checksum)
			if e != nil {
				log.Fatal(e)
			}
			if e = s.MigrateCharacterEvents(ctx); e != nil {
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
		if data.Source.Checksum != worldService.Catalog.Source.Checksum {
			log.Fatal("dungeon/world source versions differ")
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
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = characters.Store.MigrateCharacterEvents(ctx)
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
		lootService = &loot.Service{Store: characters.Store, Catalog: c, DropCatalog: dropCatalog, Rules: r, BagRules: bag, Tables: tables}
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
		questService = &quest.Service{Store: characters.Store, Catalog: data, Professions: characters.Catalog, Progression: progressionService}
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
					log.Printf("separate wear catalog: %d records; original reward/drop catalog: %d", len(full.Records), len(equipment.Rows))
				}
			}
			// The same source equipment catalog backs quest rewards and
			// monster gear drops; a drop only offers what a bag accepts.
			lootService.Equipment = equipment
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
		if *vaultPurchase || *vaultRelease {
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
			e = characters.Store.MigrateAccountMaterials(ctx)
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
	if *channelRefreshFile != "" {
		channelCfg, err = channelrefresh.Load(*channelRefreshFile)
		if err != nil {
			log.Fatal(err)
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
		defer c.Close()
		peer := c.RemoteAddr().String()
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
		if (*vaultPurchase || *vaultRelease) && vaultService != nil {
			purchaseSession.vaultRules = &vaultService.Rules
		}
		var selectedBasic []byte
		var selectedAddition []byte
		var worldState *worldSession
		var skillState skillSession
		var equipmentState equipmentSession
		if worldService != nil {
			worldState = &worldSession{characters: characters, service: worldService, account: developmentAccount, flags: townPolicy.Flags, dungeons: dungeonCatalog, tutorials: tutorialRoutes, tutorialDungeons: tutorialDungeons, professions: characters.Catalog, fatigue: fatigueService, quests: questService, progression: progressionService, loot: lootService, vault: vaultService, soloPartyBootstrap: *soloPartyBootstrap, hub: hub}
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
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			var incoming clientRead
			select {
			case incoming = <-frames:
			case now := <-ticker.C:
				if bootstrapped && selectedCharacterID != 0 && worldState != nil {
					p, e := worldState.refreshDailyFatigue(now)
					if e != nil {
						event(map[string]any{"kind": "fatigue_daily_error", "error": e.Error()})
						continue
					}
					if p != nil {
						if e = sendPayload(0, 36, p); e != nil {
							return
						}
						event(map[string]any{"kind": "fatigue_daily_refresh", "character_id": selectedCharacterID})
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
						packets, encodeErr := shopPilotPackets(receipt, balance, applied)
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
					// Keep the explicitly approved Odyssey pilot credits isolated from
					// ordinary life-token consumption.
					if odysseyTemporaryCreditsEnabled() && isOdysseyRewardRole(worldState.role) && worldState.activeDungeon != nil && worldState.activeDungeon.Definition.Odyssey {
						plan, e = worldState.pilotRevive(ctx, characters.Store, plaintext, frame.Raw)
					} else {
						plan, e = worldState.lifeTokenRevive(ctx, characters.Store, plaintext, frame.Raw)
					}
				} else if worldState.activeDungeon != nil || worldState.role.ID == 0 {
					e = fmt.Errorf("booster box use requires selected character in town")
				} else {
					plan, e = worldState.openBoosterItem(ctx, characters.Store, wearService, lootService, boosterCatalog, odysseyChoices, plaintext, frame.Raw)
				}
				cancel()
				if e != nil {
					event(map[string]any{"kind": "booster_action_refused", "id": frame.ID, "reason": e.Error()})
					plan = []outboundPacket{{"booster_action_refused_ack", 1, frame.ID, protocol.Refusal(4)}}
				}
				for _, packet := range plan {
					if sendPayload(packet.Kind, packet.ID, packet.Payload) != nil {
						return
					}
					event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": selectedCharacterID, "plain_hex": hex.EncodeToString(packet.Payload)})
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
				if decodeErr == nil && characters != nil && (r.SourceSlot == 26 || r.DestinationSlot == 26 || r.SourceList == 3 || r.DestinationList == 3) {
					var visual []byte
					visual, e = characters.EntryBasicProbe(worldState.role, [2]byte{})
					if e == nil {
						plan = append(plan, outboundPacket{"creature_actor_appearance_updated", 0, 2, visual})
					}
					wornUpdate, e := inventory.WornSpaceUpdate(worldState.role.State)
					if e == nil && len(wornUpdate) > 0 {
						plan = append(plan, outboundPacket{"worn_equipment_visuals_updated", 0, 14, wornUpdate})
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
			if frame.Type != 1 {
				event(map[string]any{"kind": "unsupported_client_type", "type": frame.Type})
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
				event(map[string]any{"kind": "unified_option_accepted", "character_id": selectedCharacterID})
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
			if characters != nil && bootstrapped && (frame.ID == 28 || frame.ID == 29) {
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
			if worldState != nil && bootstrapped && frame.ID == 18 {
				if !verified {
					continue
				}
				plan, e := worldState.deleteSkillMaterial(plaintext, frame.Raw)
				if e != nil {
					event(map[string]any{"kind": "skill_material_refused", "reason": e.Error(), "character_id": worldState.role.ID})
					if e = sendPayload(1, 18, protocol.MaterialDeleteReply(nil, false)); e != nil {
						return
					}
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
			if worldState != nil && bootstrapped && frame.ID == 507 && fatigueService != nil {
				if !verified {
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
			if worldState != nil && bootstrapped && frame.ID == 44 && lootService != nil {
				if !verified {
					event(map[string]any{"kind": "item_use_rejected", "reason": "checksum failed"})
					continue
				}
				plan, e := worldState.useStackable(plaintext)
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
			if worldState != nil && bootstrapped && frame.ID == 21 && lootService != nil {
				if !verified {
					event(map[string]any{"kind": "shop_buy_rejected", "reason": "checksum failed"})
					continue
				}
				plan, e := worldState.buyItem(plaintext)
				if e != nil {
					event(map[string]any{"kind": "shop_buy_refused", "character_id": worldState.role.ID, "reason": e.Error()})
					if e = sendPayload(1, 21, protocol.Refusal(4)); e != nil {
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
			if worldState != nil && bootstrapped && frame.ID == 22 && lootService != nil {
				if !verified {
					event(map[string]any{"kind": "shop_sell_rejected", "reason": "checksum failed"})
					continue
				}
				plan, e := worldState.sellItem(plaintext)
				if e != nil {
					event(map[string]any{"kind": "shop_sell_refused", "character_id": worldState.role.ID, "reason": e.Error()})
					if e = sendPayload(1, 22, protocol.Refusal(4)); e != nil {
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
					plan, e = worldState.monsterDeath(plaintext)
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
				case 132:
					plan, e = worldState.returnFromDungeonSelection(plaintext)
				case 42:
					if len(plaintext) != 0 {
						e = fmt.Errorf("unexpected give-up body")
					} else {
						plan, e = worldState.leaveDungeon()
					}
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
					if frame.ID == 39 || frame.ID == 46 || frame.ID == 117 || frame.ID == 132 {
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
				if pending != nil {
					if frame.ID == 16 || frame.ID == 72 {
						worldState.deathSent = map[uint16]bool{}
						worldState.drops = nil
						worldState.resetCards()
					}
					worldState.activeDungeon = pending
					// A dungeon is a private instance: this actor leaves the shared town.
					worldState.leaveScene()
					worldState.completionSent = false
					worldState.resultSent = false
					worldState.selectingDungeon = false
					event(map[string]any{"kind": "dungeon_session_started", "dungeon": pending.Definition.ID, "maze": pending.Maze.Index, "map": pending.Room.Map, "monsters": len(pending.Monsters), "quests_changed": false})
				}
				if frame.ID == 37 {
					worldState.activeDungeon.Loaded = true
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
					if p.Name == "settlement_exit_ack" && pending == nil {
						worldState.activeDungeon = nil
						worldState.drops = nil
						worldState.deathSent = nil
						worldState.completionSent = false
						worldState.resultSent = false
						worldState.resetCards()
						worldState.selectingDungeon = p.Payload[2] == 1
					}
					if p.Name == "monster_death_confirmed" {
						if worldState.deathSent == nil {
							worldState.deathSent = map[uint16]bool{}
						}
						entity := uint16(p.Payload[0]) | uint16(p.Payload[1])<<8
						worldState.deathSent[entity] = true
						if worldState.drops != nil && len(worldState.drops.Skipped[entity]) > 0 {
							event(map[string]any{"kind": "drop_rules_pending", "entity": entity, "rules": worldState.drops.Skipped[entity]})
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
				}
				if frame.ID == 42 || frame.ID == 132 {
					worldState.selectingDungeon = false
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
			if worldState != nil && bootstrapped && (frame.ID == 35 || frame.ID == 36) {
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
				}
				profile := *selectProbe
				ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
				profile.TutorialCompleted, e = characters.Store.TutorialFlags(ctx, developmentAccount, role.ID)
				cancel()
				if e != nil {
					event(map[string]any{"kind": "tutorial_restore_error", "error": e.Error()})
					continue
				}
				event(map[string]any{"kind": "tutorial_flags_restored", "character_id": role.ID, "completed": profile.TutorialCompleted})
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
				if vaultService != nil {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					vaultPayload, e = vaultService.Bootstrap(ctx, role)
					cancel()
					if e != nil {
						event(map[string]any{"kind": "vault_entry_rejected", "error": e.Error()})
						continue
					}
				}
				var areaPayload []byte
				if *townProbeFile != "" {
					var state character.State
					if e = json.Unmarshal(role.State, &state); e != nil || !townCatalog.Allows(state.Level, townPolicy.X, townPolicy.Y) {
						event(map[string]any{"kind": "town_entry_rejected", "error": "character level or spawn policy incompatible with source area"})
						continue
					}
					premiumCtx, premiumCancel := context.WithTimeout(context.Background(), 5*time.Second)
					premiums, pe := characters.Store.ActivePremiums(premiumCtx, developmentAccount, time.Now())
					premiumCancel()
					if pe != nil {
						event(map[string]any{"kind": "premium_restore_error", "error": pe.Error()})
						continue
					}
					for _, premium := range premiums {
						profile.Premiums = append(profile.Premiums, protocol.PremiumEntry{Type: premium.Type, EndTime: premium.EndTime})
					}
					event(map[string]any{"kind": "premiums_restored", "account": developmentAccount, "count": len(profile.Premiums)})
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
						worldState.peer = &lanPeer{roleID: role.ID, actorID: role.WireID, channel: channel, info: basic, addition: addition, send: sendPayload}
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
				plan := entryPayloads{Select: payload, Basic: basic, Addition: addition, Vault: vaultPayload, UserArea: userArea, Area: areaPayload, Fatigue: fatiguePayload, AccountOptions: accountOptionsPayload}
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
				if e == nil && characters != nil {
					plan.SkillVariations, e = characters.VariationRestore(role)
				}
				if e != nil {
					event(map[string]any{"kind": "cinematic_restore_error", "error": e.Error()})
					continue
				}
				if wearService != nil {
					plan.Worn, e = inventory.WornPayload(role.State)
					if e == nil {
						plan.WornUpdate, e = inventory.WornSpaceUpdate(role.State)
					}
					if e == nil {
						plan.AvatarReady, e = inventory.SpecialEquipmentRestorePayload(role.State, 1)
					}
					if e == nil {
						plan.Avatars, e = inventory.EquipmentPayload(1, nil, true)
					}
					if e == nil {
						plan.Creatures, e = inventory.SpecialEquipmentRestorePayload(role.State, 7)
					}
					if e == nil {
						plan.CreatureList, _ = inventory.CreatureListPayload(role.State)
						if inventory.HasEquippedCreature(role.State) {
							plan.CreatureGrowth = []byte{1, 0, 0, 0, 0, 0}
						}
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
						continue
					}
					plan.AccountMaterials, e = accountMaterialSnapshot(materials)
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
				}
				if len(addition) > 0 && len(areaPayload) > 0 {
					plan.Complete = protocol.EnterGameworldComplete()
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
				if worldState != nil {
					if e = worldState.announceSelf(event); e != nil {
						event(map[string]any{"kind": "area_presence_error", "error": e.Error()})
					}
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
				c.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if _, err := io.Copy(c, bytes.NewReader(response)); err != nil {
					event(map[string]any{"kind": "write_error", "error": err.Error()})
					return
				}
				event(map[string]any{"kind": "server_response", "peer": peer, "id": frame.ID, "hex": hex.EncodeToString(response)})
				if frame.ID == 1 {
					bootstrapped = true
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
