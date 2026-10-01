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

func preparePVFHellMaps(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, inputs pvfItemInputs) error {
	if !selected["dungeon-hell"] {
		return nil
	}
	if c.dungeons == nil {
		return fmt.Errorf("PVF dungeon-hell requires native dungeons")
	}
	direct, unavailable, err := s.HellPartyMaps(*c.dungeons)
	if err != nil {
		return err
	}
	if inputs.checksBaselines() {
		path := filepath.Join(filepath.Dir(fullDungeonAuditPath(inputs)), "dungeons.hell-party-maps.json")
		var legacy catalog.SourceMapOverlay
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &legacy); err != nil {
			return err
		}
		if err := verifyPVFCatalog(legacy, direct); err != nil {
			return fmt.Errorf("Hell Party maps: %w", err)
		}
	}
	base := clonePVFDungeons(*c.dungeons)
	if err := catalog.ApplyHellPartyMaps(&base, direct); err != nil {
		return err
	}
	c.hellMaps = &direct
	log.Printf("PVF Hell Party maps prepared: maps=%d unavailable source references=%d; missing maps remain refused", len(direct.Maps), len(unavailable))
	s.ReleaseReadCaches()
	return nil
}

func (c pvfCoreCatalogs) attachHellMaps(data *catalog.DungeonCatalog, path string) error {
	if c.hellMaps != nil {
		return catalog.ApplyHellPartyMaps(data, *c.hellMaps)
	}
	return catalog.AttachHellPartyMaps(data, path)
}

func preparePVFMazeRates(c *pvfCoreCatalogs, selected map[string]bool, inputs pvfItemInputs) error {
	if !selected["dungeon-maze"] {
		return nil
	}
	if c.dungeons == nil {
		return fmt.Errorf("PVF dungeon-maze requires native dungeons")
	}
	policy, err := readPVFScenePolicy(inputs.scenePolicyPath)
	if err != nil {
		return err
	}
	direct, err := catalog.ImportMazeChanceOverlay(*c.dungeons, policy.MazeRates)
	if err != nil {
		return err
	}
	if inputs.checksBaselines() {
		path := filepath.Join(filepath.Dir(fullDungeonAuditPath(inputs)), "dungeons.maze-chance-rates.json")
		var legacy catalog.MazeChanceOverlay
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(b, &legacy); err != nil {
			return err
		}
		if err := verifyPVFCatalog(legacy, direct); err != nil {
			return fmt.Errorf("maze rates: %w", err)
		}
	}
	base := clonePVFDungeons(*c.dungeons)
	if err := catalog.ApplyMazeChanceRates(&base, direct); err != nil {
		return err
	}
	c.mazeRates = &direct
	log.Printf("PVF maze rates prepared: %d selected dungeons; existing weight overrides retained", len(direct.Dungeons))
	return nil
}

func (c pvfCoreCatalogs) attachMazeRates(data *catalog.DungeonCatalog, path string) error {
	if c.mazeRates != nil {
		return catalog.ApplyMazeChanceRates(data, *c.mazeRates)
	}
	return catalog.AttachMazeChanceRates(data, path)
}
