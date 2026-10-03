package main

import (
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"fmt"
	"log"
	"maps"
	"path/filepath"
	"slices"
)

func preparePVFEnhancements(c *pvfCoreCatalogs, s *gamedata.Source, inputs pvfItemInputs) error {
	direct := c.enhancements
	var err error
	if direct == nil {
		direct, err = s.Enhancements(*c.items, inputs.enhancementPolicyPath)
	}
	if err != nil {
		return err
	}
	if inputs.checksBaselines() {
		legacy, err := inventory.ReadEnhancementBaseline(filepath.Dir(inputs.indexPath))
		if err != nil {
			return err
		}
		if err = auditPVFEnhancements(legacy, direct); err != nil {
			return fmt.Errorf("enhancements: %w", err)
		}
	}
	c.enhancements = direct
	log.Printf("PVF enhancements prepared: reinforcement tickets=%d amplify tickets=%d grimoires=%d enchant beads=%d reinforcement levels=%d amplify levels=%d", len(direct.ReinforcementTickets), len(direct.AmplifyTickets), len(direct.Grimoires.Grimoires), len(direct.Enchant.Beads), len(direct.Gold.Levels), len(direct.Amplify.Levels))
	s.ReleaseReadCaches()
	return nil
}

func auditPVFEnhancements(legacy, direct *inventory.EnhancementCatalog) error {
	// These exports identify different outer snapshots. This is a field audit,
	// not an archive alias: direct source/save checks remain strict and unchanged.
	legacy.Grimoires.Source = direct.Grimoires.Source
	legacy.Enchant.Source = direct.Enchant.Source
	legacy.Enchant.Rule = direct.Enchant.Rule // descriptive text, never consumed
	legacy.Gold.Source = direct.Gold.Source
	legacy.Amplify.Source = direct.Amplify.Source
	// The old ordinary-ticket export has no expiration headers. Native scripts
	// contain 926. Keep those headers in the direct catalog; the ticket consumer
	// checks the saved instance ExpireTime, not this script date. The independently
	// audited periods catalog supplies template period classification. Only absent
	// headers may be supplemented: any changed existing date or other tag fails.
	supplemented := 0
	for id, row := range legacy.ReinforcementTickets {
		native, ok := direct.ReinforcementTickets[id]
		if !ok {
			continue
		}
		if _, exists := row.Fields["[expiration date]"]; exists {
			continue
		}
		if date, exists := native.Fields["[expiration date]"]; exists {
			row.Fields = maps.Clone(row.Fields)
			row.Fields["[expiration date]"] = slices.Clone(date)
			legacy.ReinforcementTickets[id] = row
			supplemented++
		}
	}
	if err := verifyPVFCatalog(legacy, direct); err != nil {
		return err
	}
	log.Printf("PVF enhancement audit passed: ordinary-ticket native expiration headers supplemented=%d; all other typed fields equal", supplemented)
	return nil
}

func (c pvfCoreCatalogs) loadEnhancements(dir string) error {
	if c.enhancements != nil {
		return c.enhancements.Activate()
	}
	for _, r := range []struct {
		name string
		load func(string) error
	}{
		{"reinforcement-tickets.json", inventory.LoadReinforcementTickets},
		{"reinforcement-gold.json", inventory.LoadGoldRules},
		{"amplify-grimoire.json", inventory.LoadAmplifyGrimoires},
		{"amplify-upgrade.json", inventory.LoadAmplifyUpgradeRules},
		{"amplify-tickets.json", inventory.LoadAmplifyTickets},
		{"enchant-beads.json", inventory.LoadEnchantBeads},
	} {
		if err := r.load(filepath.Join(dir, r.name)); err != nil {
			return err
		}
	}
	return nil
}
