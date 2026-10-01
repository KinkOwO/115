package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

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

func preparePVFRules(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, inputs pvfItemInputs) error {
	checksum := s.Snapshot().Checksum
	dir := filepath.Dir(inputs.indexPath)
	if selected["tutorial"] {
		direct, err := s.Tutorials()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			legacy, err := catalog.LoadTutorialRoutes(inputs.tutorialPath, checksum)
			if err != nil {
				return err
			}
			if err = verifyPVFCatalog(*legacy, direct); err != nil {
				return fmt.Errorf("tutorial: %w", err)
			}
		}
		c.tutorial = &direct
		log.Printf("PVF tutorial prepared: %d source flows", len(direct.Flows))
		s.ReleaseReadCaches()
	}
	if selected["periods"] {
		path := filepath.Join(dir, "item-period-tags.json")
		var direct catalog.ItemPeriodCatalog
		var err error
		if c.itemBasics != nil {
			direct = *c.itemBasics.Periods
		} else {
			direct, err = s.ItemPeriods()
		}
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			var legacy catalog.ItemPeriodCatalog
			if err := readPVFRuleBaseline(path, checksum, &legacy); err != nil {
				return err
			}
			if _, err := catalog.LoadItemPeriods(path, checksum); err != nil {
				return err
			}
			if err = verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("periods: %w", err)
			}
		}
		c.periods = direct.Templates
		log.Printf("PVF item periods prepared: %d templates", len(c.periods))
		s.ReleaseReadCaches()
	}
	if selected["skins"] {
		path := filepath.Join(dir, "skin-storage-items.json")
		direct, err := s.SkinStorage()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			var legacy catalog.SkinStorageCatalog
			if err := readPVFRuleBaseline(path, checksum, &legacy); err != nil {
				return err
			}
			if _, err := catalog.LoadSkinStorage(path, checksum); err != nil {
				return err
			}
			if err = verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("skins: %w", err)
			}
		}
		c.skins = make(map[uint32]catalog.SkinStorageEntry, len(direct.Entries))
		for _, e := range direct.Entries {
			c.skins[e.Template] = e
		}
		log.Printf("PVF skin storage prepared: %d templates, %d unresolved source skins", len(c.skins), len(direct.MissingSkins))
		s.ReleaseReadCaches()
	}
	if selected["journal"] {
		direct, err := s.EquipmentJournal()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			legacy, err := catalog.LoadEquipmentJournalRules(inputs.journalPath, checksum)
			if err != nil {
				return err
			}
			if err = verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("journal: %w", err)
			}
		}
		c.journal = &direct
		log.Printf("PVF equipment journal prepared: %d categories", len(direct.Categories))
		s.ReleaseReadCaches()
	}
	if selected["create-cost"] {
		direct, err := s.EquipmentCreateCost()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			legacy, err := catalog.LoadEquipmentCreateCost(inputs.createCostPath, checksum)
			if err != nil {
				return err
			}
			if err = verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("create-cost: %w", err)
			}
		}
		c.createCost = &direct
		log.Printf("PVF equipment creation costs prepared: %d groups", len(direct.Groups))
		s.ReleaseReadCaches()
	}
	return nil
}

func (c pvfCoreCatalogs) loadItemPeriods(path, checksum string) ([]uint32, error) {
	if c.periods != nil {
		return c.periods, nil
	}
	return catalog.LoadItemPeriods(path, checksum)
}
func (c pvfCoreCatalogs) loadSkinStorage(path, checksum string) (map[uint32]catalog.SkinStorageEntry, error) {
	if c.skins != nil {
		return c.skins, nil
	}
	return catalog.LoadSkinStorage(path, checksum)
}
func (c pvfCoreCatalogs) loadEquipmentJournal(path, checksum string) (catalog.EquipmentJournalRules, error) {
	if c.journal != nil {
		return *c.journal, nil
	}
	return catalog.LoadEquipmentJournalRules(path, checksum)
}
func (c pvfCoreCatalogs) loadEquipmentCreateCost(path, checksum string) (catalog.EquipmentCreateCost, error) {
	if c.createCost != nil {
		return *c.createCost, nil
	}
	return catalog.LoadEquipmentCreateCost(path, checksum)
}

func (c pvfCoreCatalogs) loadTutorialRoutes(path, checksum string) (*catalog.TutorialCatalog, error) {
	if c.tutorial != nil {
		if c.tutorial.Source.Checksum != checksum {
			return nil, fmt.Errorf("prepared tutorial source mismatch")
		}
		return c.tutorial, nil
	}
	return catalog.LoadTutorialRoutes(path, checksum)
}
