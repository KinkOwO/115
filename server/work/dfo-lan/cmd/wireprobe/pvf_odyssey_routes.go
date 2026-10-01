package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/gamedata"
	"fmt"
	"log"
)

func preparePVFOdysseyRoutes(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["odyssey-routes"] {
		return nil
	}
	direct, err := s.OdysseyJournalRoutes()
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := character.EmbeddedOdysseyJournalRoutes()
		if err != nil {
			return err
		}
		if err := verifyPVFCatalog(old.Nodes, direct.Nodes); err != nil {
			return fmt.Errorf("Odyssey journal routes: %w", err)
		}
	}
	c.odysseyRoutes = direct
	s.ReleaseReadCaches()
	log.Printf("PVF Odyssey journal routes prepared: nodes=%d dungeon references=50; prior-node membership and native destinations retained", len(direct.Nodes))
	return nil
}

func (c pvfCoreCatalogs) bindOdysseyRoutes(s *character.ProgressionService) error {
	if c.odysseyRoutes == nil {
		return nil
	}
	if s == nil {
		return fmt.Errorf("missing progression service for Odyssey routes")
	}
	routes, err := catalog.NewOdysseyJournalRoutes(*c.odysseyRoutes)
	if err != nil {
		return err
	}
	s.JournalRoutes = routes
	return nil
}
