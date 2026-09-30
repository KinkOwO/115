package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
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
	quests      *catalog.QuestCatalog
	progression *catalog.Progression
	world       *catalog.WorldCatalog
	items       *catalog.ItemIndex
	equipment   *inventory.FullEquipmentCatalog
	periods     []uint32
	skins       map[uint32]catalog.SkinStorageEntry
	journal     *catalog.EquipmentJournalRules
	createCost  *catalog.EquipmentCreateCost
	learning    *character.LearningCatalog
	prices      *catalog.ShopPrices
	materials   *catalog.ItemMaterials
	boosters    map[uint32]catalog.BoosterDefinition
	tutorial    *catalog.TutorialCatalog
}

type pvfItemInputs struct {
	indexPath, fullPrefix, journalPath, createCostPath, learningPath, pricesPath, materialsPath, boosterPath, tutorialPath string
	verifyBaselines                                                                                                        *bool
}

func (i pvfItemInputs) checksBaselines() bool { return i.verifyBaselines == nil || *i.verifyBaselines }

func parsePVFCatalogSelection(value string) (map[string]bool, error) {
	selected := map[string]bool{}
	if strings.TrimSpace(value) == "" {
		return selected, nil
	}
	for _, domain := range strings.Split(value, ",") {
		domain = strings.TrimSpace(domain)
		if domain != "quests" && domain != "progression" && domain != "world" && domain != "items" && domain != "equipment" && domain != "periods" && domain != "skins" && domain != "journal" && domain != "create-cost" && domain != "skills" && domain != "prices" && domain != "materials" && domain != "boosters" && domain != "tutorial" {
			return nil, fmt.Errorf("PVF candidate domain %q is not enabled; supported: quests,progression,world,items,equipment,periods,skins,journal,create-cost,skills,prices,materials,boosters,tutorial (character parity is pending)", domain)
		}
		if selected[domain] {
			return nil, fmt.Errorf("duplicate PVF candidate domain %q", domain)
		}
		selected[domain] = true
	}
	return selected, nil
}

func verifyPVFCatalog(legacy, direct any) error {
	comparison := gamedata.Compare(legacy, direct, 1)
	if comparison.Count != 0 {
		first := comparison.Differences[0]
		return fmt.Errorf("PVF candidate has %d effective field differences; first %s: JSON=%s PVF=%s", comparison.Count, first.Path, first.JSON, first.PVF)
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
	characters, err := catalog.LoadCharacters(characterPath)
	if err != nil {
		return result, fmt.Errorf("PVF character source anchor: %w", err)
	}
	started := time.Now()
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, ExpectedChecksum: checksum})
	if err != nil {
		return result, err
	}
	if source.Snapshot().Checksum != characters.Source.Checksum {
		return result, fmt.Errorf("PVF/character source mismatch: %s versus %s", source.Snapshot().Checksum, characters.Source.Checksum)
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
	if selected["skills"] {
		if e := preparePVFLearning(&result, source, characters, inputs); e != nil {
			return result, e
		}
	}
	if selected["items"] || selected["equipment"] || selected["prices"] || selected["materials"] || selected["boosters"] {

		direct, e := source.ItemIndex("")
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
		source.ReleaseReadCaches()
		if e := preparePVFCommerce(&result, source, selected, inputs); e != nil {
			return result, e
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
				if full.IndexSHA256 != candidate.IndexSHA256 || len(full.Records) != len(candidate.Records) || len(full.Errors) != 0 {
					return result, fmt.Errorf("full equipment baseline/PVF index differs")
				}
				for id := range full.Records {
					if _, ok := candidate.Records[id]; !ok {
						return result, fmt.Errorf("equipment %d absent from PVF index", id)
					}
				}
			}
			result.equipment = candidate
			log.Printf("PVF lazy equipment prepared: %d source bindings; expanded chunks bounded to 64 MiB", len(candidate.Records))
		}
	}
	log.Printf("PVF candidate catalogs prepared in %s; full directory can be collected before opening storage", time.Since(started))
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
	if c.quests != nil || c.progression != nil || c.world != nil || c.items != nil || c.periods != nil || c.skins != nil || c.journal != nil || c.createCost != nil || c.learning != nil || c.tutorial != nil {
		runtime.GC()
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
