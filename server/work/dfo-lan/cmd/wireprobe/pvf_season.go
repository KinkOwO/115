package main

import (
	"dfolan/internal/adventure"
	"dfolan/internal/gamedata"
	"fmt"
	"log"
)

func preparePVFSeason(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["season"] {
		return nil
	}
	if c.items == nil {
		return fmt.Errorf("native season requires native item index")
	}
	direct, err := s.Season(*c.items)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := adventure.EmbeddedSeasonRules()
		if err != nil {
			return err
		}
		baseline := *old
		if baseline.SourcePath != direct.SourcePath {
			return fmt.Errorf("season source path mismatch")
		}
		if baseline.SourceChecksum != direct.SourceChecksum {
			if baseline.SourceChecksum != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.SourceChecksum != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80" {
				return fmt.Errorf("season baseline has unknown provenance")
			}
			baseline.SourceChecksum = direct.SourceChecksum
		}
		if err := verifyPVFCatalog(&baseline, direct); err != nil {
			return fmt.Errorf("season: %w", err)
		}
	}
	c.seasonRules = direct
	s.ReleaseReadCaches()
	log.Printf("PVF season rules prepared: levels=%d contents=%d penalties=%d capsules=%d reward items=%d oath equipment=%d; source experience and cost key retained", len(direct.Levels), len(direct.Contents), len(direct.Penalties), len(direct.Capsules), len(direct.Items), len(direct.OathEquipment))
	return nil
}

func (c pvfCoreCatalogs) installSeasonRules() (func(), error) {
	if c.seasonRules == nil {
		return func() {}, nil
	}
	return adventure.InstallSeasonRules(c.seasonRules)
}
