package main

import (
	"dfolan/internal/character"
	"dfolan/internal/gamedata"
	"fmt"
	"log"
)

func preparePVFRosterBackgrounds(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["roster-backgrounds"] {
		return nil
	}
	if c.items == nil {
		return fmt.Errorf("native background tickets require native item index")
	}
	direct, err := s.RosterBackgrounds(*c.items)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := character.EmbeddedRosterBackgroundTickets()
		if err != nil {
			return err
		}
		if old.Source != direct.Source && (old.Source != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.Source != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80") {
			return fmt.Errorf("background ticket baseline has unknown provenance")
		}
		if err := verifyPVFCatalog(old.Items, direct.Items); err != nil {
			return fmt.Errorf("background tickets: %w", err)
		}
		resources := map[character.RosterBackground]bool{}
		for _, b := range direct.Backgrounds {
			resources[b] = true
		}
		for category := 0; category < 256; category++ {
			for id := 0; id <= 65535; id++ {
				b := character.RosterBackground{Category: uint8(category), ID: uint16(id)}
				if resources[b] != old.ValidRosterBackground(b) {
					return fmt.Errorf("native background boundary differs at %v", b)
				}
			}
		}
	}
	c.rosterBackgrounds = direct
	s.ReleaseReadCaches()
	log.Printf("PVF roster backgrounds prepared: tickets=%d resources=%d; independent item deletion and background authorization dates retained", len(direct.Items), len(direct.Backgrounds))
	return nil
}

func (c pvfCoreCatalogs) installRosterBackgrounds() (func(), error) {
	if c.rosterBackgrounds == nil {
		return func() {}, nil
	}
	return character.InstallRosterBackgroundTickets(c.rosterBackgrounds)
}
