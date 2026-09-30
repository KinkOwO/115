package main

import (
	"dfolan/internal/adventure"
	"dfolan/internal/gamedata"
	"fmt"
	"log"
)

func preparePVFRecommended(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["adventure-recommended"] {
		return nil
	}
	direct, err := s.RecommendedDungeons()
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := adventure.EmbeddedRecommendedRules()
		if err != nil {
			return err
		}
		baseline := *old
		if baseline.SourceChecksum != direct.SourceChecksum {
			if baseline.SourceChecksum != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.SourceChecksum != "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80" {
				return fmt.Errorf("recommended baseline has unknown source provenance")
			}
			baseline.SourceChecksum = direct.SourceChecksum
		}
		if err := verifyPVFCatalog(&baseline, direct); err != nil {
			return fmt.Errorf("recommended: %w", err)
		}
	}
	c.recommendedRules = direct
	s.ReleaseReadCaches()
	log.Printf("PVF recommended dungeons prepared: ranges=%d excluded=%d ambiguous=%d unavailable worldmaps=%d; source eligibility retained", len(direct.Ranges), len(direct.Excluded), len(direct.AmbiguousDungeons), len(direct.UnavailableWorldmaps))
	return nil
}

func (c pvfCoreCatalogs) installRecommendedRules() (func(), error) {
	if c.recommendedRules == nil {
		return func() {}, nil
	}
	return adventure.InstallRecommendedRules(c.recommendedRules)
}
