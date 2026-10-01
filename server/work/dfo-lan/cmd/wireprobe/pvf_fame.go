package main

import (
	"dfolan/internal/character"
	"dfolan/internal/gamedata"
	"fmt"
	"log"
)

func preparePVFFame(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["fame"] {
		return nil
	}
	if c.items == nil {
		return fmt.Errorf("native fame requires native item index")
	}
	direct, err := s.Fame(*c.items)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := character.EmbeddedFameRules()
		if err != nil {
			return err
		}
		if old.Source != direct.Source && (old.Source != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.Source != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80") {
			return fmt.Errorf("fame baseline has unknown provenance")
		}
		if err := verifyPVFCatalog(old, direct); err != nil {
			return fmt.Errorf("fame: %w", err)
		}
	}
	c.fameRules = direct
	s.ReleaseReadCaches()
	log.Printf("PVF fame rules prepared: tables=%d items=%d sets=%d item points=%d awakening templates=%d sources=%d; existing client formulas retained", len(direct.Tables), len(direct.Items), len(direct.Sets), len(direct.ItemPoints), len(direct.Awakening), len(direct.Sources))
	return nil
}

func (c pvfCoreCatalogs) installFameRules() (func(), error) {
	if c.fameRules == nil {
		return func() {}, nil
	}
	return character.InstallFameRules(c.fameRules)
}
