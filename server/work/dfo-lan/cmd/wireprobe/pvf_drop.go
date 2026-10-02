package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func preparePVFLoot(c *pvfCoreCatalogs, s *gamedata.Source, inputs pvfItemInputs) error {
	policy, err := inventory.ReadDropPolicy(inputs.dropPolicyPath)
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
	if inputs.checksBaselines() {
		path := inputs.lootPath
		if override := os.Getenv("DFO_LOOT_CATALOG"); override != "" {
			path = override
		}
		if path == "" {
			path = filepath.Join(filepath.Dir(inputs.indexPath), "loot.next25.json")
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
	c.loot = &direct
	log.Printf("PVF loot prepared: maximum grade=%d stackable candidates=%d drop groups=%d dungeon indexes=%d; ordinary difficulty/creation weights from PVF", direct.MaximumGrade, len(direct.Items), len(direct.DropGroups), len(direct.DungeonDropInfo))
	s.ReleaseReadCaches()
	return nil
}

func preparePVFEquipmentSelection(c *pvfCoreCatalogs, s *gamedata.Source, inputs pvfItemInputs) error {
	policy, err := inventory.ReadDropPolicy(inputs.dropPolicyPath)
	if err != nil {
		return err
	}
	quests := c.quests
	if quests == nil {
		native, err := s.Quests("")
		if err != nil {
			return err
		}
		quests = &native
	}
	direct, err := s.EquipmentSelection(*c.items, *quests, policy)
	if err != nil {
		return err
	}
	if inputs.checksBaselines() {
		paths := []string{inputs.equipmentPath, inputs.questEquipmentPath}
		if paths[0] == "" && paths[1] == "" {
			paths = []string{filepath.Join(filepath.Dir(inputs.indexPath), "equipment.current37.json")}
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
	c.selection = direct
	log.Printf("PVF equipment selection prepared: basic whitelist=%d source quest additions=%d total=%d legacy drop pool=%d ordinary source pool=%d", len(policy.BasicEquipmentIDs), len(direct.Rows)-len(policy.BasicEquipmentIDs), len(direct.Rows), len(direct.DropPool()), len(direct.OrdinaryPool))
	s.ReleaseReadCaches()
	return nil
}

func (c pvfCoreCatalogs) loadLoot(path string) (catalog.LootCatalog, error) {
	if c.loot != nil {
		return *c.loot, nil
	}
	return catalog.LoadLoot(path)
}

func (c pvfCoreCatalogs) loadEquipmentSelection(path, source string) (*inventory.EquipmentCatalog, error) {
	if c.selection != nil {
		if source != c.selection.Source.Checksum {
			return nil, fmt.Errorf("prepared equipment selection source mismatch")
		}
		copy := *c.selection
		return &copy, nil
	}
	return inventory.LoadEquipmentCatalog(path, source)
}
