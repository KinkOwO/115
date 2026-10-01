package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/gamedata"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

func loadBoosterBaseline(path string) (map[uint32]catalog.BoosterDefinition, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var raw map[string]catalog.BoosterDefinition
	if err = json.NewDecoder(f).Decode(&raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty booster baseline")
	}
	out := make(map[uint32]catalog.BoosterDefinition, len(raw))
	for key, def := range raw {
		id, err := strconv.ParseUint(key, 10, 32)
		if err != nil || id == 0 || uint32(id) != def.Template {
			return nil, fmt.Errorf("invalid booster key %s", key)
		}
		out[def.Template] = def
	}
	return out, nil
}

// Row order in the historical skill exporter came from map iteration. The
// runtime identity is (profession, skill), so compare that exact projection.
func learningRows(c *character.LearningCatalog) map[byte]map[uint16]character.LearningDefinition {
	out := map[byte]map[uint16]character.LearningDefinition{}
	for _, row := range c.Rows {
		if out[row.Job] == nil {
			out[row.Job] = map[uint16]character.LearningDefinition{}
		}
		out[row.Job][row.ID] = row
	}
	return out
}

func preparePVFLearning(c *pvfCoreCatalogs, s *gamedata.Source, chars catalog.Characters, inputs pvfItemInputs) error {
	direct, err := s.Learning(chars)
	if err != nil {
		return err
	}
	if inputs.checksBaselines() {
		path := inputs.learningPath
		if path == "" {
			path = os.Getenv("DFO_SKILL_CATALOG")
		}
		if path == "" {
			return fmt.Errorf("PVF skills requires the active skill catalog baseline")
		}
		legacy, err := character.LoadLearningCatalog(path, chars.Source.Checksum)
		if err != nil {
			return err
		}
		if err = verifyPVFCatalog(learningRows(legacy), learningRows(direct)); err != nil {
			return fmt.Errorf("skills: %w", err)
		}
	}
	c.learning = direct
	log.Printf("PVF learning prepared: %d definitions", len(direct.Rows))
	s.ReleaseReadCaches()
	return nil
}

func preparePVFCommerce(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, inputs pvfItemInputs) error {
	checksum := s.Snapshot().Checksum
	dir := filepath.Dir(inputs.indexPath)
	if selected["prices"] {
		path := inputs.pricesPath
		if path == "" {
			path = filepath.Join(dir, "shop-prices.json")
		}
		var direct *catalog.ShopPrices
		var err error
		if c.itemBasics != nil {
			direct = c.itemBasics.Prices
		} else {
			direct, err = s.ShopPrices(*c.items)
		}
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			legacy, err := catalog.LoadShopPrices(path, checksum)
			if err != nil {
				return err
			}
			if err = verifyPVFCatalog(legacy.Items, direct.Items); err != nil {
				return fmt.Errorf("prices: %w", err)
			}
		}
		c.prices = direct
		log.Printf("PVF prices prepared: %d definitions", len(direct.Items))
		s.ReleaseReadCaches()
	}
	if selected["materials"] {
		path := inputs.materialsPath
		if path == "" {
			path = filepath.Join(dir, "item-materials.json")
		}
		var direct *catalog.ItemMaterials
		var err error
		if c.itemBasics != nil && c.itemBasics.Materials != nil {
			direct = c.itemBasics.Materials
		} else {
			direct, err = s.ItemMaterials(*c.items)
		}
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			legacy, err := catalog.LoadItemMaterials(path)
			if err != nil {
				return err
			}
			if legacy == nil || len(legacy.Source) != 64 {
				return fmt.Errorf("materials baseline lacks source provenance")
			}
			if err = verifyPVFCatalog(legacy.Items, direct.Items); err != nil {
				return fmt.Errorf("materials: %w", err)
			}
			// The existing material loader never binds this metadata to player
			// saves. Exact cost/path parity permits replacement of this projection,
			// while the new catalog keeps the verified PVF checksum. This is not
			// an archive-version alias and never rewrites a save's source version.
			if legacy.Source != checksum {
				log.Printf("PVF materials provenance replaced after complete cost parity: %s -> %s", legacy.Source, checksum)
			}
		}
		c.materials = direct
		log.Printf("PVF materials prepared: %d definitions", len(direct.Items))
		s.ReleaseReadCaches()
	}
	if selected["boosters"] {
		path := inputs.boosterPath
		if path == "" {
			path = filepath.Join(dir, "booster-catalog.json")
		}
		direct, err := s.Boosters(*c.items)
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			legacy, err := loadBoosterBaseline(path)
			if err != nil {
				return err
			}
			if err = verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("boosters: %w", err)
			}
		}
		c.boosters = direct
		log.Printf("PVF boosters prepared: %d definitions", len(direct))
		s.ReleaseReadCaches()
	}
	return nil
}

func (c pvfCoreCatalogs) loadLearning(path, checksum string) (*character.LearningCatalog, error) {
	if c.learning != nil {
		if c.learning.Source.Checksum != checksum {
			return nil, fmt.Errorf("prepared learning source mismatch")
		}
		return c.learning, nil
	}
	return character.LoadLearningCatalog(path, checksum)
}
func (c pvfCoreCatalogs) loadShopPrices(path, checksum string) (*catalog.ShopPrices, error) {
	if c.prices != nil {
		if c.prices.Source != checksum {
			return nil, fmt.Errorf("prepared price source mismatch")
		}
		return c.prices, nil
	}
	return catalog.LoadShopPrices(path, checksum)
}
func (c pvfCoreCatalogs) loadItemMaterials(path string) (*catalog.ItemMaterials, error) {
	if c.materials != nil {
		return c.materials, nil
	}
	return catalog.LoadItemMaterials(path)
}
