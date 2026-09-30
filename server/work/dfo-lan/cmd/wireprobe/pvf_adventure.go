package main

import (
	"dfolan/internal/adventure"
	"dfolan/internal/gamedata"
	"fmt"
	"log"
)

func preparePVFAdventure(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["adventure"] {
		return nil
	}
	if c.items == nil {
		return fmt.Errorf("native adventure requires native item index")
	}
	direct, err := s.Adventure(*c.items)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := adventure.EmbeddedRules()
		if err != nil {
			return err
		}
		if err := auditAdventureRules(old, direct); err != nil {
			return fmt.Errorf("adventure: %w", err)
		}
	}
	c.adventureRules = direct
	s.ReleaseReadCaches()
	log.Printf("PVF adventure prepared: levels=%d shops=%d items=%d; limits, prices, experience and reset rules retained", len(direct.Experience), len(direct.Shops), len(direct.Items))
	return nil
}

// The embedded export named its old outer file. Normalize only that known
// provenance for a complete audit; every rule and raw hash still must match.
// Native runtime metadata and character/save identities are never rewritten.
func auditAdventureRules(old, direct *adventure.Rules) error {
	if old == nil || direct == nil || len(direct.SourceChecksum) != 64 {
		return fmt.Errorf("missing adventure source identity")
	}
	baseline := *old
	if baseline.SourceChecksum != direct.SourceChecksum {
		if baseline.SourceChecksum != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.SourceChecksum != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80" {
			return fmt.Errorf("adventure baseline has unknown source provenance")
		}
		baseline.SourceChecksum = direct.SourceChecksum
	}
	return verifyPVFCatalog(&baseline, direct)
}

func (c pvfCoreCatalogs) installAdventureRules() (func(), error) {
	if c.adventureRules == nil {
		return func() {}, nil
	}
	return adventure.InstallRules(c.adventureRules)
}
