package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"fmt"
	"log"
	"path/filepath"
)

func preparePVFClosingScenes(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["dungeon-terminal"] && !selected["dungeon-tournament"] {
		return nil
	}
	if c.dungeons == nil {
		return fmt.Errorf("native closing scene overlays require native dungeons")
	}
	dir := filepath.Dir(fullDungeonAuditPath(i))
	if selected["dungeon-terminal"] {
		quests := c.quests
		if quests == nil {
			q, err := s.Quests("")
			if err != nil {
				return err
			}
			quests = &q
		}
		direct, err := s.TerminalScenes(*c.dungeons, *quests)
		if err != nil {
			return err
		}
		s.ReleaseReadCaches()
		base := clonePVFDungeons(*c.dungeons)
		if err := catalog.ApplyTerminalScenes(&base, direct); err != nil {
			return err
		}
		if i.checksBaselines() {
			old := clonePVFDungeons(*c.dungeons)
			if err := catalog.AttachTerminalScenes(&old, filepath.Join(dir, "dungeons.terminal-scenes.json")); err != nil {
				return err
			}
			if err := verifyPVFCatalog(old.TerminalScenes, direct.Scenes); err != nil {
				return fmt.Errorf("terminal scenes: %w", err)
			}
		}
		c.terminalScenes = &direct
		log.Printf("PVF terminal scenes prepared: %d native quest/maze/ACT/CMT chains", len(direct.Scenes))
	}
	if selected["dungeon-tournament"] {
		direct, err := s.TournamentQuestMaps(*c.dungeons)
		if err != nil {
			return err
		}
		s.ReleaseReadCaches()
		base := clonePVFDungeons(*c.dungeons)
		if err := catalog.ApplyTournamentQuestMaps(&base, direct); err != nil {
			return err
		}
		if i.checksBaselines() {
			old := clonePVFDungeons(*c.dungeons)
			if err := catalog.AttachTournamentQuestMaps(&old, filepath.Join(dir, "dungeons.tournament-quest-maps.json")); err != nil {
				return err
			}
			if err := verifyPVFCatalog(old, base); err != nil {
				return fmt.Errorf("tournament maps: %w", err)
			}
		}
		c.tournamentMaps = &direct
		log.Printf("PVF tournament quest arenas prepared: %d source owner bindings", len(direct.Maps))
	}
	return nil
}

func (c pvfCoreCatalogs) attachTerminalScenes(d *catalog.DungeonCatalog, path string) error {
	if c.terminalScenes != nil {
		return catalog.ApplyTerminalScenes(d, *c.terminalScenes)
	}
	return catalog.AttachTerminalScenes(d, path)
}

func (c pvfCoreCatalogs) attachTournamentMaps(d *catalog.DungeonCatalog, path string) error {
	if c.tournamentMaps != nil {
		return catalog.ApplyTournamentQuestMaps(d, *c.tournamentMaps)
	}
	return catalog.AttachTournamentQuestMaps(d, path)
}
