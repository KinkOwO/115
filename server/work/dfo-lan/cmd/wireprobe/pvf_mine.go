package main

import (
	"dfolan/internal/gamedata"
	"dfolan/internal/loot"
	"fmt"
	"log"
	"path/filepath"
)

func preparePVFMine(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["bleeding-mine"] {
		return nil
	}
	policy, err := readPVFContentPolicy(i.contentPolicyPath)
	if err != nil {
		return err
	}
	direct, err := s.BleedingMine(*c.items, policy.BleedingMine)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		path := i.minePath
		if path == "" && i.lootPath != "" {
			path = filepath.Join(filepath.Dir(i.lootPath), "bleeding-mine-rewards.json")
		}
		legacy, err := loot.LoadBleedingMineRewards(pvfOdysseyBaseline(i, path, "", "bleeding-mine-rewards.json"))
		if err != nil {
			return err
		}
		if legacy.Source != direct.Source {
			return fmt.Errorf("mine baseline/PVF source mismatch")
		}
		if err := verifyPVFCatalog(legacy, direct); err != nil {
			return fmt.Errorf("mine rewards: %w", err)
		}
	}
	c.mine = direct
	log.Printf("PVF mine rewards prepared: stages=%d boss entries=%d difficulty rewards=%d containers=%d items=%d; signed empty faces and source composition retained", len(direct.StageBoxes), len(direct.BossBoxes), len(direct.GroupBoxes), len(direct.Boxes), len(direct.Items))
	s.ReleaseReadCaches()
	return nil
}

func (c pvfCoreCatalogs) loadMine(path string) (*loot.BleedingMineRewards, error) {
	if c.mine != nil {
		return c.mine, nil
	}
	return loot.LoadBleedingMineRewards(path)
}
