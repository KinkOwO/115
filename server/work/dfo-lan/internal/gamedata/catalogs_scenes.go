package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

func preparePVFClosingScenes(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["dungeon-terminal"] && !selected["dungeon-tournament"] {
		return nil
	}
	if c.Dungeons == nil {
		return fmt.Errorf("native closing scene overlays require native dungeons")
	}
	dir := filepath.Dir(fullDungeonAuditPath(i))
	if selected["dungeon-terminal"] {
		quests := c.Quests
		if quests == nil {
			q, err := s.Quests("")
			if err != nil {
				return err
			}
			quests = &q
		}
		direct, err := s.TerminalScenes(*c.Dungeons, *quests)
		if err != nil {
			return err
		}
		s.ReleaseReadCaches()
		base := clonePVFDungeons(*c.Dungeons)
		if err := catalog.ApplyTerminalScenes(&base, direct); err != nil {
			return err
		}
		if i.checksBaselines() {
			old := clonePVFDungeons(*c.Dungeons)
			if err := catalog.AttachTerminalScenes(&old, filepath.Join(dir, "dungeons.terminal-scenes.json")); err != nil {
				return err
			}
			if err := verifyPVFCatalog(old.TerminalScenes, direct.Scenes); err != nil {
				return fmt.Errorf("terminal scenes: %w", err)
			}
		}
		c.TerminalScenes = &direct
		log.Printf("PVF terminal scenes prepared: %d native quest/maze/ACT/CMT chains", len(direct.Scenes))
	}
	if selected["dungeon-tournament"] {
		direct, err := s.TournamentQuestMaps(*c.Dungeons)
		if err != nil {
			return err
		}
		s.ReleaseReadCaches()
		base := clonePVFDungeons(*c.Dungeons)
		if err := catalog.ApplyTournamentQuestMaps(&base, direct); err != nil {
			return err
		}
		c.TournamentMaps = &direct
		log.Printf("PVF tournament quest arenas prepared: %d source owner bindings", len(direct.Maps))
	}
	return nil
}

func (c *Catalogs) AttachTerminalScenes(d *catalog.DungeonCatalog, path string) error {
	if err := c.RequireSelected("dungeon-terminal", c.TerminalScenes != nil); err != nil {
		return err
	}
	if c.TerminalScenes != nil {
		return catalog.ApplyTerminalScenes(d, *c.TerminalScenes)
	}
	return catalog.AttachTerminalScenes(d, path)
}

func (c *Catalogs) AttachTournamentMaps(d *catalog.DungeonCatalog, _ string) error {
	if err := c.RequireSelected("dungeon-tournament", c.TournamentMaps != nil); err != nil {
		return err
	}
	if c.TournamentMaps == nil {
		return fmt.Errorf("tournament maps require the native PVF dungeon-tournament domain")
	}
	return catalog.ApplyTournamentQuestMaps(d, *c.TournamentMaps)
}

func preparePVFHellMaps(c *Catalogs, s *Source, selected map[string]bool, inputs CatalogInputs) error {
	if !selected["dungeon-hell"] {
		return nil
	}
	if c.Dungeons == nil {
		return fmt.Errorf("PVF dungeon-hell requires native dungeons")
	}
	direct, unavailable, err := s.HellPartyMaps(*c.Dungeons)
	if err != nil {
		return err
	}
	base := clonePVFDungeons(*c.Dungeons)
	if err := catalog.ApplyHellPartyMaps(&base, direct); err != nil {
		return err
	}
	c.HellMaps = &direct
	log.Printf("PVF Hell Party maps prepared: maps=%d unavailable source references=%d; missing maps remain refused", len(direct.Maps), len(unavailable))
	s.ReleaseReadCaches()
	return nil
}

func (c *Catalogs) AttachHellMaps(data *catalog.DungeonCatalog, _ string) error {
	if err := c.RequireSelected("dungeon-hell", c.HellMaps != nil); err != nil {
		return err
	}
	if c.HellMaps == nil {
		return fmt.Errorf("Hell Party maps require the native PVF dungeon-hell domain")
	}
	return catalog.ApplyHellPartyMaps(data, *c.HellMaps)
}

func preparePVFMazeRates(c *Catalogs, selected map[string]bool, inputs CatalogInputs) error {
	if !selected["dungeon-maze"] {
		return nil
	}
	if c.Dungeons == nil {
		return fmt.Errorf("PVF dungeon-maze requires native dungeons")
	}
	policy, err := readPVFScenePolicy(inputs.ScenePolicyPath)
	if err != nil {
		return err
	}
	direct, err := catalog.ImportMazeChanceOverlay(*c.Dungeons, policy.MazeRates)
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
	base := clonePVFDungeons(*c.Dungeons)
	if err := catalog.ApplyMazeChanceRates(&base, direct); err != nil {
		return err
	}
	c.MazeRates = &direct
	log.Printf("PVF maze rates prepared: %d selected dungeons; existing weight overrides retained", len(direct.Dungeons))
	return nil
}

func (c *Catalogs) AttachMazeRates(data *catalog.DungeonCatalog, path string) error {
	if err := c.RequireSelected("dungeon-maze", c.MazeRates != nil); err != nil {
		return err
	}
	if c.MazeRates != nil {
		return catalog.ApplyMazeChanceRates(data, *c.MazeRates)
	}
	return catalog.AttachMazeChanceRates(data, path)
}

func preparePVFLayerRevisits(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["layer-revisits"] {
		return nil
	}
	if c.Dungeons == nil {
		return fmt.Errorf("native layer revisits require native dungeons")
	}
	f, err := os.Open(i.LayerRevisitPolicyPath)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	var policy catalog.LayerRevisitPolicy
	if err = decoder.Decode(&policy); err != nil {
		return err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("layer revisit policy has trailing data")
	}
	direct, err := s.LayerRevisits(*c.Dungeons, policy)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old := clonePVFDungeons(*c.Dungeons)
		if err = catalog.AttachLayerRevisits(&old, filepath.Join(filepath.Dir(fullDungeonAuditPath(i)), "dungeons.layer-revisits.json")); err != nil {
			return err
		}
		if err = verifyPVFCatalog(old.LayerRevisits, direct.Scenes); err != nil {
			return fmt.Errorf("layer revisits: %w", err)
		}
	}
	c.LayerRevisits = &direct
	s.ReleaseReadCaches()
	log.Printf("PVF layer revisits prepared: %d native final-layer/map/ACT/CMT bindings; witnessed records and base cache restoration retained", len(direct.Scenes))
	return nil
}
func (c *Catalogs) AttachLayerRevisits(d *catalog.DungeonCatalog, path string) error {
	if err := c.RequireSelected("layer-revisits", c.LayerRevisits != nil); err != nil {
		return err
	}
	if c.LayerRevisits != nil {
		return catalog.ApplyLayerRevisits(d, *c.LayerRevisits)
	}
	return catalog.AttachLayerRevisits(d, path)
}

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

func preparePVFScenes(c *Catalogs, s *Source, selected map[string]bool, inputs CatalogInputs) error {
	if !selected["town"] && !selected["dungeons"] && !selected["training-dungeons"] && !selected["tutorial-dungeons"] {
		return nil
	}
	policy, err := readPVFScenePolicy(inputs.ScenePolicyPath)
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
			path := inputs.TownPath
			if path == "" {
				path = filepath.Join(filepath.Dir(inputs.IndexPath), "town.generated.json")
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
		c.Town = &direct
		log.Printf("PVF entry town prepared: town=%d area=%d rectangles=%d; separate spawn policy retained", direct.TownID, direct.AreaID, len(direct.Walkable))
		s.ReleaseReadCaches()
	}
	if selected["dungeons"] {
		world := c.World
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
		c.Dungeons = &direct
		log.Printf("PVF full dungeons prepared: dungeons=%d maps=%d skipped=%d", len(direct.Dungeons), len(direct.Maps), len(direct.Skipped))
		if declared := direct.DeclaredEnterFatigue(); declared != "" {
			log.Printf("PVF dungeons declaring [use fatigue only start dungeon]: %s", declared)
		}
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
		path := inputs.TrainingDungeonPath
		if override := os.Getenv("DFO_TRAINING_ROOM_CATALOG"); override != "" {
			path = override
		}
		if path == "" {
			path = filepath.Join(filepath.Dir(inputs.IndexPath), "dungeons.training-room.json")
		}
		if err := audit(path, direct); err != nil {
			return err
		}
		c.TrainingDungeons = &direct
		log.Printf("PVF training dungeons prepared: dungeons=%d maps=%d", len(direct.Dungeons), len(direct.Maps))
		s.ReleaseReadCaches()
	}
	if selected["tutorial-dungeons"] {
		routes := c.Tutorial
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
		path := inputs.TutorialDungeonPath
		if path == "" {
			path = filepath.Join(filepath.Dir(inputs.IndexPath), "tutorial-dungeons.current36.json")
		}
		if err := audit(path, direct); err != nil {
			return err
		}
		c.TutorialDungeons = &direct
		log.Printf("PVF tutorial dungeons prepared: dungeons=%d maps=%d", len(direct.Dungeons), len(direct.Maps))
		s.ReleaseReadCaches()
	}
	return nil
}

func fullDungeonAuditPath(inputs CatalogInputs) string {
	path := inputs.DungeonPath
	if override := os.Getenv("DFO_ODYSSEY_DUNGEON_CATALOG"); override != "" {
		path = override
	}
	if path == "" {
		path = filepath.Join(filepath.Dir(inputs.IndexPath), "dungeons.full.json")
	}
	return path
}

func (c *Catalogs) LoadTown(path string) (catalog.TownArea, error) {
	if err := c.RequireSelected("town", c.Town != nil); err != nil {
		return catalog.TownArea{}, err
	}
	if c.Town != nil {
		return *c.Town, nil
	}
	return catalog.LoadTownArea(path)
}

func (c *Catalogs) LoadDungeons(path string) (catalog.DungeonCatalog, error) {
	if err := c.RequireSelected("dungeons", c.Dungeons != nil); err != nil {
		return catalog.DungeonCatalog{}, err
	}
	if c.Dungeons != nil {
		return clonePVFDungeons(*c.Dungeons), nil
	}
	return catalog.DungeonCatalog{}, fmt.Errorf("dungeons require a prepared native PVF domain; JSON runtime catalogs are retired")
}

func (c *Catalogs) LoadTrainingDungeons(path string) (catalog.DungeonCatalog, error) {
	if err := c.RequireSelected("training-dungeons", c.TrainingDungeons != nil); err != nil {
		return catalog.DungeonCatalog{}, err
	}
	if c.TrainingDungeons != nil {
		return clonePVFDungeons(*c.TrainingDungeons), nil
	}
	return catalog.LoadDungeons(path)
}

func (c *Catalogs) LoadTutorialDungeons(path string) (catalog.DungeonCatalog, error) {
	if err := c.RequireSelected("tutorial-dungeons", c.TutorialDungeons != nil); err != nil {
		return catalog.DungeonCatalog{}, err
	}
	if c.TutorialDungeons != nil {
		return clonePVFDungeons(*c.TutorialDungeons), nil
	}
	return catalog.LoadDungeons(path)
}

func preparePVFScriptWarps(c *Catalogs, s *Source, selected map[string]bool, i CatalogInputs) error {
	if !selected["script-warps"] {
		return nil
	}
	if c.Dungeons == nil {
		return fmt.Errorf("native script warps require native dungeons")
	}
	f, err := os.Open(i.ScriptWarpPolicyPath)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	var policy catalog.ScriptWarpPolicy
	if err = decoder.Decode(&policy); err != nil {
		return err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("script warp policy has trailing data")
	}
	direct, err := s.ScriptWarpRoutes(*c.Dungeons, policy)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := dungeon.EmbeddedScriptWarpRoutes()
		if err != nil {
			return err
		}
		if len(old) != len(direct) {
			return fmt.Errorf("script warp scope changed")
		}
		for n := range old {
			if old[n].Source != direct[n].Source {
				return fmt.Errorf("script warp source identity changed")
			}
		}
		if err = verifyPVFCatalog(old, direct); err != nil {
			return fmt.Errorf("script warps: %w", err)
		}
	}
	c.ScriptWarps = direct
	s.ReleaseReadCaches()
	log.Printf("PVF script warps prepared: %d native map/CMT/object/custom-action/maze bindings; witnessed transition records and key-room admission retained", len(direct))
	return nil
}
func (c *Catalogs) InstallScriptWarps() (func(), error) {
	if err := c.RequireSelected("script-warps", c.Prepared("script-warps")); err != nil {
		return nil, err
	}
	if c.ScriptWarps == nil && !c.Selected("script-warps") {
		return func() {}, nil
	}
	return dungeon.InstallScriptWarpRoutes(c.ScriptWarps)
}

func preparePVFTowers(c *Catalogs, s *Source, selected map[string]bool, inputs CatalogInputs) error {
	if !selected["dungeon-towers"] {
		return nil
	}
	if c.Dungeons == nil {
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
	base := clonePVFDungeons(*c.Dungeons)
	if err := catalog.ApplyTowerGriefMaps(&base, grief); err != nil {
		return err
	}
	if err := catalog.ApplyDazzlementMaps(&base, dazzlement); err != nil {
		return err
	}
	c.Grief, c.Dazzlement = &grief, &dazzlement
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

func (c *Catalogs) AttachTowerGrief(data *catalog.DungeonCatalog, _ string) error {
	if err := c.RequireSelected("dungeon-towers", c.Grief != nil); err != nil {
		return err
	}
	if c.Grief == nil {
		return fmt.Errorf("Tower of Grief maps require the native PVF dungeon-towers domain")
	}
	return catalog.ApplyTowerGriefMaps(data, *c.Grief)
}

func (c *Catalogs) AttachTowerDazzlement(data *catalog.DungeonCatalog, _ string) error {
	if err := c.RequireSelected("dungeon-towers", c.Dazzlement != nil); err != nil {
		return err
	}
	if c.Dazzlement == nil {
		return fmt.Errorf("Tower of Dazzlement maps require the native PVF dungeon-towers domain")
	}
	return catalog.ApplyDazzlementMaps(data, *c.Dazzlement)
}
