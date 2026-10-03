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

func preparePVFTowers(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, inputs pvfItemInputs) error {
	if !selected["dungeon-towers"] {
		return nil
	}
	if c.dungeons == nil {
		return fmt.Errorf("PVF dungeon-towers requires native dungeons")
	}
	grief, err := s.TowerGrief()
	if err != nil {
		return err
	}
	s.ReleaseReadCaches()
	dazzlement, err := s.TowerDazzlement()
	if err != nil {
		return err
	}
	s.ReleaseReadCaches()
	if inputs.checksBaselines() {
		dir := filepath.Dir(fullDungeonAuditPath(inputs))
		var oldGrief catalog.TowerGriefOverlay
		var oldDazzlement catalog.DazzlementOverlay
		for _, x := range []struct {
			name        string
			old, direct any
		}{
			{"dungeons.tower-of-grief-maps.json", &oldGrief, &grief},
			{"dungeons.tower-of-dazzlement-maps.json", &oldDazzlement, &dazzlement},
		} {
			b, err := os.ReadFile(filepath.Join(dir, x.name))
			if err != nil {
				return err
			}
			if err := json.Unmarshal(b, x.old); err != nil {
				return err
			}
			if err := verifyPVFCatalog(x.old, x.direct); err != nil {
				return fmt.Errorf("tower %s: %w", x.name, err)
			}
		}
	}
	base := clonePVFDungeons(*c.dungeons)
	if err := catalog.ApplyTowerGriefMaps(&base, grief); err != nil {
		return err
	}
	if err := catalog.ApplyDazzlementMaps(&base, dazzlement); err != nil {
		return err
	}
	c.grief, c.dazzlement = &grief, &dazzlement
	log.Printf("PVF towers prepared: grief floors=%d maps=%d dazzlement dungeons=%d maps=%d; progression and settlement unchanged", len(grief.Layers), len(grief.Maps), len(dazzlement.Dungeons), len(dazzlement.Maps))
	return nil
}

// Each runtime consumer gets its own mutable maze/map wrapper. Script cells
// remain shared read-only source facts; overlay application never edits them.
func clonePVFDungeons(c catalog.DungeonCatalog) catalog.DungeonCatalog {
	dungeons := make(map[uint32]catalog.DungeonDefinition, len(c.Dungeons))
	for id, d := range c.Dungeons {
		d.Mazes = append([]catalog.DungeonMaze(nil), d.Mazes...)
		for i := range d.Mazes {
			d.Mazes[i].Rooms = append([]catalog.DungeonRoom(nil), d.Mazes[i].Rooms...)
			d.Mazes[i].Pending = append([]string(nil), d.Mazes[i].Pending...)
		}
		dungeons[id] = d
	}
	maps := make(map[uint32]catalog.ScriptRecord, len(c.Maps))
	for id, script := range c.Maps {
		maps[id] = script
	}
	c.Dungeons, c.Maps = dungeons, maps
	return c
}

func (c pvfCoreCatalogs) attachTowerGrief(data *catalog.DungeonCatalog, path string) error {
	if c.grief != nil {
		return catalog.ApplyTowerGriefMaps(data, *c.grief)
	}
	return catalog.AttachTowerGriefMaps(data, path)
}

func (c pvfCoreCatalogs) attachTowerDazzlement(data *catalog.DungeonCatalog, path string) error {
	if c.dazzlement != nil {
		return catalog.ApplyDazzlementMaps(data, *c.dazzlement)
	}
	return catalog.AttachDazzlementMaps(data, path)
}
