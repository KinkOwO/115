package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/loot"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

func pvfOdysseyBaseline(i pvfItemInputs, explicit, env, fallback string) string {
	if explicit != "" {
		return explicit
	}
	if path := os.Getenv(env); path != "" {
		return path
	}
	return filepath.Join(filepath.Dir(i.indexPath), fallback)
}

func preparePVFOdyssey(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["odyssey-growth"] && !selected["odyssey-chapters"] && !selected["odyssey-weapons"] && !selected["odyssey-drop"] && !selected["odyssey-currency"] {
		return nil
	}
	var policy pvfContentPolicy
	if selected["odyssey-growth"] || selected["odyssey-drop"] || selected["odyssey-currency"] {
		var err error
		policy, err = readPVFContentPolicy(i.contentPolicyPath)
		if err != nil {
			return err
		}
	}
	if selected["odyssey-growth"] {
		direct, err := s.OdysseyGrowth(*c.items, policy.OdysseySupplemental)
		if err != nil {
			return err
		}
		if i.checksBaselines() {
			legacy, err := catalog.LoadOdysseyGrowth(pvfOdysseyBaseline(i, i.odysseyGrowthPath, "DFO_ODYSSEY_GROWTH", "odyssey-growth-release.json"))
			if err != nil {
				return err
			}
			if legacy.Source != direct.Source {
				return fmt.Errorf("Odyssey growth baseline source mismatch")
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("Odyssey growth: %w", err)
			}
		}
		c.odysseyGrowth = direct
		log.Printf("PVF Odyssey growth prepared: clear=%d entry=%d gifts=%d graduate=%d; graduation quests retained", len(direct.ClearLevels), len(direct.EntryLevels), len(direct.Gifts), direct.GraduateReward)
	}
	var chapters *catalog.OdysseyChapters
	if selected["odyssey-chapters"] || selected["odyssey-drop"] {
		var err error
		chapters, err = s.OdysseyChapters()
		if err != nil {
			return err
		}
		if selected["odyssey-chapters"] {
			if i.checksBaselines() {
				legacy, err := catalog.LoadOdysseyChapters(pvfOdysseyBaseline(i, i.odysseyChapterPath, "DFO_ODYSSEY_CHAPTERS", "odyssey-chapters-release.json"))
				if err != nil {
					return err
				}
				if legacy.Source.Checksum != chapters.Source.Checksum {
					return fmt.Errorf("Odyssey chapters baseline source mismatch")
				}
				if err := verifyPVFCatalog(legacy, chapters); err != nil {
					return fmt.Errorf("Odyssey chapters: %w", err)
				}
			}
			c.odysseyChapters = chapters
			log.Printf("PVF Odyssey chapters prepared: chapters=%d dungeons=%d reward templates=%d", chapters.ChapterCount, chapters.DungeonCount, chapters.RewardTemplate)
		}
	}
	if selected["odyssey-drop"] {
		direct, err := loot.ImportOdysseyChapterDrop(chapters, *c.items, policy.OdysseyDrops)
		if err != nil {
			return err
		}
		if i.checksBaselines() {
			legacy, err := loot.LoadOdysseyChapterDrop(pvfOdysseyBaseline(i, i.odysseyDropPath, "DFO_ODYSSEY_CHAPTER_DROP", "odyssey-chapter-drop-release.json"))
			if err != nil {
				return err
			}
			if legacy.Source.Checksum != direct.Source.Checksum {
				return fmt.Errorf("Odyssey drop baseline source mismatch")
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("Odyssey chapter drop: %w", err)
			}
		}
		c.odysseyDrop = direct
		log.Printf("PVF Odyssey chapter drop prepared: %d rows; chapter activation and rates retained", len(direct.Drops))
	}
	if selected["odyssey-currency"] {
		direct, err := s.OdysseyCurrency(*c.items, policy.OdysseyCurrency)
		if err != nil {
			return err
		}
		if i.checksBaselines() {
			legacy, err := loot.LoadOdysseyCurrency(pvfOdysseyBaseline(i, i.odysseyCurrencyPath, "DFO_ODYSSEY_COIN_RULES", "odyssey-currency.json"))
			if err != nil {
				return err
			}
			if legacy.Source != direct.Source {
				return fmt.Errorf("Odyssey currency baseline source mismatch")
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("Odyssey currency: %w", err)
			}
		}
		c.odysseyCurrency = direct
		log.Printf("PVF Odyssey currency prepared: %d templates; rates=%v operator policy retained", len(direct.Items), direct.Rates)
	}
	if selected["odyssey-weapons"] {
		direct, err := s.OdysseyWeapons(*c.items)
		if err != nil {
			return err
		}
		if i.checksBaselines() {
			legacy, err := catalog.LoadOdysseyWeaponChoices(pvfOdysseyBaseline(i, i.odysseyWeaponPath, "DFO_ODYSSEY_WEAPON_BOX", "odyssey-weapon-box-release.json"))
			if err != nil {
				return err
			}
			if legacy.Source != direct.Source {
				return fmt.Errorf("Odyssey weapon baseline source mismatch")
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("Odyssey weapons: %w", err)
			}
		}
		c.odysseyWeapons = &direct
		log.Printf("PVF Odyssey creation weapons prepared: categories=%d; existing rewards activation retained", len(direct.Categories))
	}
	s.ReleaseReadCaches()
	return nil
}

func (c pvfCoreCatalogs) loadOdysseyGrowth(path string) (*catalog.OdysseyGrowth, error) {
	if c.odysseyGrowth != nil {
		return c.odysseyGrowth, nil
	}
	return catalog.LoadOdysseyGrowth(path)
}
func (c pvfCoreCatalogs) loadOdysseyChapters(path string) (*catalog.OdysseyChapters, error) {
	if c.odysseyChapters != nil {
		return c.odysseyChapters, nil
	}
	return catalog.LoadOdysseyChapters(path)
}
func (c pvfCoreCatalogs) loadOdysseyDrop(path string) (*loot.OdysseyChapterDrop, error) {
	if c.odysseyDrop != nil {
		return c.odysseyDrop, nil
	}
	return loot.LoadOdysseyChapterDrop(path)
}
func (c pvfCoreCatalogs) loadOdysseyCurrency(path string) (*loot.OdysseyCurrency, error) {
	if c.odysseyCurrency != nil {
		return c.odysseyCurrency, nil
	}
	return loot.LoadOdysseyCurrency(path)
}
func (c pvfCoreCatalogs) loadOdysseyWeapons(path string) (odysseyWeaponChoices, error) {
	if c.odysseyWeapons != nil {
		return odysseyWeaponChoices(*c.odysseyWeapons), nil
	}
	return loadOdysseyWeaponChoices(path)
}
