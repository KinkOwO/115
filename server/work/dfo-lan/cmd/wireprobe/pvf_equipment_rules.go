package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func preparePVFEquipmentRules(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, inputs pvfItemInputs) error {
	dir := filepath.Dir(inputs.indexPath)
	if selected["random-options"] {
		direct, err := s.RandomOptions()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			path := inputs.randomOptionPath
			if path == "" {
				path = filepath.Join(dir, "randomoption.current37.json")
			}
			legacy, err := inventory.ReadRandomOptionData(path)
			if err != nil {
				return err
			}
			if legacy.Source.Checksum != direct.Source.Checksum {
				return fmt.Errorf("random options source mismatch")
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("random options: %w", err)
			}
		}
		runtime, err := inventory.NewRandomOptionCatalog(direct, s.Snapshot().Checksum)
		if err != nil {
			return err
		}
		c.randomOptions = runtime
		log.Printf("PVF random options prepared: ratios=%d quantity rows=%d groups=%d choices=%d costs=%d", len(direct.ValueRatios), len(direct.OptionQuantities), len(direct.OptionGroups), len(direct.GroupChoices), len(direct.BreakSealCosts))
		s.ReleaseReadCaches()
	}
	if selected["oath-grades"] {
		direct, err := s.OathGrades()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			path := inputs.oathPath
			if path == "" {
				path = filepath.Join(dir, "oath-grades.json")
			}
			legacy, err := inventory.LoadOathGradeTable(path)
			if err != nil {
				return err
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("oath grades: %w", err)
			}
		}
		c.oath = direct
		log.Printf("PVF oath grades prepared: %d entries; diagnostic enable policy unchanged", direct.Len())
		s.ReleaseReadCaches()
	}
	if selected["vault"] {
		direct, err := s.VaultRules(inputs.vaultPolicyPath)
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			path := inputs.vaultPath
			if path == "" {
				path = filepath.Join(dir, "vault.generated.json")
			}
			legacy, err := inventory.LoadVaultRules(path)
			if err != nil {
				return err
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("vault: %w", err)
			}
		}
		c.vault = &direct
		log.Printf("PVF account vault prepared: level=%d upgrades=%d; client capacity save source retained=%s", direct.Account.RequiredLevel, len(direct.Account.Upgrades), direct.SourceSHA256)
		s.ReleaseReadCaches()
	}
	return nil
}

func preparePVFShields(c *pvfCoreCatalogs, s *gamedata.Source, jobs catalog.Characters, inputs pvfItemInputs) error {
	wear := inputs.wearRulesPath
	if override := os.Getenv("DFO_EQUIPMENT_WEAR_RULES"); override != "" {
		wear = override
	}
	rules, err := inventory.LoadWearRules(wear, s.Snapshot().Checksum)
	if err != nil {
		return err
	}
	direct, err := s.KnightShields(*c.items, jobs, rules)
	if err != nil {
		return err
	}
	if inputs.checksBaselines() {
		path := inputs.shieldPath
		if path == "" {
			path = "equipment-knight-shield.full-candidate.json"
		}
		path = knightShieldCatalogPath(path, wear)
		legacy, err := inventory.LoadKnightShields(path, s.Snapshot().Checksum)
		if err != nil {
			return err
		}
		if err := verifyPVFCatalog(legacy, direct); err != nil {
			return fmt.Errorf("shields: %w", err)
		}
	}
	c.shields = direct
	log.Printf("PVF knight shields prepared: %d source window rows, profession=%d; quest refusal retained", len(direct.Rows), direct.Profession)
	s.ReleaseReadCaches()
	return nil
}

func (c pvfCoreCatalogs) loadRandomOptions(path, source string) (*inventory.RandomOptionCatalog, error) {
	if c.randomOptions != nil {
		return c.randomOptions, c.randomOptions.ValidateSource(source)
	}
	return inventory.LoadRandomOptionCatalog(path, source)
}
func (c pvfCoreCatalogs) loadShields(path, source string) (*inventory.KnightShields, error) {
	if c.shields != nil {
		return c.shields, c.shields.Validate(source)
	}
	return inventory.LoadKnightShields(path, source)
}
func (c pvfCoreCatalogs) loadOathGrades(path string) (*inventory.OathGradeTable, error) {
	if c.oath != nil {
		return c.oath, nil
	}
	return loadOathGradeTable(path)
}
func (c pvfCoreCatalogs) loadVaultRules(path string) (inventory.VaultRules, error) {
	if c.vault != nil {
		return *c.vault, nil
	}
	return inventory.LoadVaultRules(path)
}
