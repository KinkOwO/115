package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

// Only the server's choice of entry scene and separately enabled training
// dungeons is policy. Map paths, hashes, rectangles and dungeon rules are PVF.
type pvfScenePolicy struct {
	Version   int                        `json:"version"`
	Town      uint32                     `json:"town"`
	Area      uint32                     `json:"area"`
	Training  []uint32                   `json:"training_dungeons"`
	Disabled  []uint32                   `json:"disabled_full_dungeons"`
	MazeRates []catalog.MazeChancePolicy `json:"maze_rates"`
}

func readPVFScenePolicy(path string) (pvfScenePolicy, error) {
	var p pvfScenePolicy
	f, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("scene policy has trailing data")
	}
	if p.Version != 1 || p.Town == 0 || len(p.Training) == 0 {
		return p, fmt.Errorf("invalid scene selection policy")
	}
	seen := map[uint32]bool{}
	for _, id := range append(append([]uint32(nil), p.Training...), p.Disabled...) {
		if id == 0 || seen[id] {
			return p, fmt.Errorf("invalid or duplicate training selection")
		}
		seen[id] = true
	}
	return p, nil
}

func preparePVFScenes(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, inputs pvfItemInputs) error {
	if !selected["town"] && !selected["dungeons"] && !selected["training-dungeons"] && !selected["tutorial-dungeons"] {
		return nil
	}
	policy, err := readPVFScenePolicy(inputs.scenePolicyPath)
	if err != nil {
		return err
	}
	audit := func(path string, direct catalog.DungeonCatalog) error {
		if !inputs.checksBaselines() {
			return nil
		}
		legacy, err := catalog.LoadDungeons(path)
		if err != nil {
			return err
		}
		if legacy.Source.Checksum != direct.Source.Checksum {
			return fmt.Errorf("dungeon baseline source mismatch")
		}
		if path == fullDungeonAuditPath(inputs) {
			legacy = normalizeOldDungeonBasisDiagnostics(legacy, policy)
		}
		expanded, err := direct.ExpandedMaps()
		if err != nil {
			return err
		}
		if err := verifyPVFCatalog(legacy, expanded); err != nil {
			return fmt.Errorf("dungeons %s: %w", path, err)
		}
		return nil
	}
	if selected["town"] {
		direct, err := s.Town(policy.Town, policy.Area)
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			path := inputs.townPath
			if path == "" {
				path = filepath.Join(filepath.Dir(inputs.indexPath), "town.generated.json")
			}
			legacy, err := catalog.LoadTownArea(path)
			if err != nil {
				return err
			}
			if legacy.Source.Checksum != direct.Source.Checksum {
				return fmt.Errorf("town baseline source mismatch")
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("town: %w", err)
			}
		}
		c.town = &direct
		log.Printf("PVF entry town prepared: town=%d area=%d rectangles=%d; separate spawn policy retained", direct.TownID, direct.AreaID, len(direct.Walkable))
		s.ReleaseReadCaches()
	}
	if selected["dungeons"] {
		world := c.world
		if world == nil {
			x, err := s.World("")
			if err != nil {
				return err
			}
			world = &x
		}
		excluded := append(append([]uint32(nil), policy.Training...), policy.Disabled...)
		direct, err := s.RuntimeFullDungeons(*world, excluded)
		if err != nil {
			return err
		}
		path := fullDungeonAuditPath(inputs)
		if err := audit(path, direct); err != nil {
			return err
		}
		c.dungeons = &direct
		log.Printf("PVF full dungeons prepared: dungeons=%d maps=%d skipped=%d", len(direct.Dungeons), len(direct.Maps), len(direct.Skipped))
		s.ReleaseReadCaches()
	}
	if selected["training-dungeons"] {
		direct, err := s.Dungeons(policy.Training)
		if err != nil {
			return err
		}
		if len(direct.Dungeons) != len(policy.Training) || len(direct.Skipped) != 0 {
			return fmt.Errorf("incomplete training dungeon import")
		}
		path := inputs.trainingDungeonPath
		if override := os.Getenv("DFO_TRAINING_ROOM_CATALOG"); override != "" {
			path = override
		}
		if path == "" {
			path = filepath.Join(filepath.Dir(inputs.indexPath), "dungeons.training-room.json")
		}
		if err := audit(path, direct); err != nil {
			return err
		}
		c.trainingDungeons = &direct
		log.Printf("PVF training dungeons prepared: dungeons=%d maps=%d", len(direct.Dungeons), len(direct.Maps))
		s.ReleaseReadCaches()
	}
	if selected["tutorial-dungeons"] {
		routes := c.tutorial
		if routes == nil {
			x, err := s.Tutorials()
			if err != nil {
				return err
			}
			routes = &x
		}
		var ids []uint32
		seen := map[uint32]bool{}
		for _, flow := range routes.Flows {
			if !flow.EventOnly && !seen[flow.Dungeon] {
				seen[flow.Dungeon] = true
				ids = append(ids, flow.Dungeon)
			}
		}
		direct, err := s.Dungeons(ids)
		if err != nil {
			return err
		}
		if len(direct.Dungeons) != len(ids) || len(direct.Skipped) != 0 {
			return fmt.Errorf("incomplete tutorial dungeon import")
		}
		path := inputs.tutorialDungeonPath
		if path == "" {
			path = filepath.Join(filepath.Dir(inputs.indexPath), "tutorial-dungeons.current36.json")
		}
		if err := audit(path, direct); err != nil {
			return err
		}
		c.tutorialDungeons = &direct
		log.Printf("PVF tutorial dungeons prepared: dungeons=%d maps=%d", len(direct.Dungeons), len(direct.Maps))
		s.ReleaseReadCaches()
	}
	return nil
}

func fullDungeonAuditPath(inputs pvfItemInputs) string {
	path := inputs.dungeonPath
	if override := os.Getenv("DFO_ODYSSEY_DUNGEON_CATALOG"); override != "" {
		path = override
	}
	if path == "" {
		path = filepath.Join(filepath.Dir(inputs.indexPath), "dungeons.full.json")
	}
	return path
}

// The old exporter rejected these fourteen missing-basis scripts. The current
// parser already accepts them, but training is separate and ten are disabled.
// Normalize only their obsolete diagnostic text; every other diagnostic and
// every gameplay field still participates in the complete comparison.
func normalizeOldDungeonBasisDiagnostics(c catalog.DungeonCatalog, p pvfScenePolicy) catalog.DungeonCatalog {
	obsolete := map[string]bool{}
	for _, id := range append(append([]uint32(nil), p.Training...), p.Disabled...) {
		obsolete[fmt.Sprintf("dungeon %d: invalid [basis level]", id)] = true
	}
	kept := make([]string, 0, len(c.Skipped))
	for _, diagnostic := range c.Skipped {
		if !obsolete[diagnostic] {
			kept = append(kept, diagnostic)
		}
	}
	c.Skipped = kept
	return c
}

func (c pvfCoreCatalogs) loadTown(path string) (catalog.TownArea, error) {
	if c.town != nil {
		return *c.town, nil
	}
	return catalog.LoadTownArea(path)
}

func (c pvfCoreCatalogs) loadDungeons(path string) (catalog.DungeonCatalog, error) {
	if c.dungeons != nil {
		return clonePVFDungeons(*c.dungeons), nil
	}
	return catalog.LoadDungeons(path)
}

func (c pvfCoreCatalogs) loadTrainingDungeons(path string) (catalog.DungeonCatalog, error) {
	if c.trainingDungeons != nil {
		return clonePVFDungeons(*c.trainingDungeons), nil
	}
	return catalog.LoadDungeons(path)
}

func (c pvfCoreCatalogs) loadTutorialDungeons(path string) (catalog.DungeonCatalog, error) {
	if c.tutorialDungeons != nil {
		return clonePVFDungeons(*c.tutorialDungeons), nil
	}
	return catalog.LoadDungeons(path)
}
