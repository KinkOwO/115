package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

func preparePVFClearCube(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["clear-cube"] {
		return nil
	}
	direct, err := s.ClearCube(*c.items)
	if err != nil {
		return err
	}
	base := catalog.LootCatalog{Source: s.Snapshot()}
	if _, err := inventory.WithClearCubeItem(base, direct); err != nil {
		return err
	}
	if i.checksBaselines() {
		path := pvfOdysseyBaseline(i, i.clearCubePath, "DFO_CLEAR_CUBE_SOURCE", "clear-cube-source.json")
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
	c.clearCube = &direct
	log.Printf("PVF clear cube prepared: template=3037; storage overlay and random-pool exclusion retained")
	s.ReleaseReadCaches()
	return nil
}

func (c pvfCoreCatalogs) withClearCube(base catalog.LootCatalog, path string) (catalog.LootCatalog, error) {
	if c.clearCube != nil {
		return inventory.WithClearCubeItem(base, *c.clearCube)
	}
	return inventory.WithClearCube(base, path)
}
