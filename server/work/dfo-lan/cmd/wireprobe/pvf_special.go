package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
)

type pvfContentPolicy struct {
	BleedingMine        loot.BleedingMinePolicy         `json:"bleeding_mine"`
	BlackPurgatory      loot.BlackPurgatoryPolicy       `json:"black_purgatory"`
	OdysseySupplemental []uint32                        `json:"odyssey_supplemental_items"`
	OdysseyDrops        []loot.OdysseyChapterDropPolicy `json:"odyssey_chapter_drops"`
	OdysseyCurrency     loot.OdysseyCurrencyPolicy      `json:"odyssey_currency"`
	Version             int                             `json:"version"`
}

func readPVFContentPolicy(path string) (pvfContentPolicy, error) {
	var p pvfContentPolicy
	f, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("content policy has trailing data")
	}
	if p.Version != 1 {
		return p, fmt.Errorf("invalid content selection policy")
	}
	return p, nil
}

func preparePVFSpecial(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, inputs pvfItemInputs) error {
	if selected["apocalypse"] {
		direct, err := s.Apocalypse()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			path := inputs.apocalypsePath
			if path == "" {
				path = filepath.Join(filepath.Dir(inputs.indexPath), "apocalypse.generated.json")
			}
			legacy, err := catalog.LoadApocalypseCatalog(path)
			if err != nil {
				return err
			}
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("apocalypse: %w", err)
			}
		}
		c.apocalypse = direct
		log.Printf("PVF apocalypse prepared: records=%d operations=%d phases=%d duty records=%d; positional rewards and operation behavior retained", direct.RecordCount, len(direct.Operations), len(direct.PhaseClock), len(direct.Duties.Records))
		s.ReleaseReadCaches()
	}
	if selected["attunement"] {
		// 副本范围来自源：etc/rewardboostinfo/**.ctp 各自声明 [dungeon index]，
		// 不再读 configs 的 attunement_dungeons（单一内容真源铁律，server/AGENTS.md §0）。
		direct, err := s.Attunement()
		if err != nil {
			return err
		}
		if inputs.checksBaselines() {
			path := inputs.attunementPath
			if override := os.Getenv("DFO_ATTUNEMENT_REWARDS"); override != "" {
				path = override
			}
			if path == "" {
				path = filepath.Join(filepath.Dir(inputs.indexPath), "attunement-rewards.generated.json")
			}
			legacy, err := loot.LoadAttunementRewards(path)
			if err != nil {
				return err
			}
			if legacy.Archive.Checksum != direct.Archive.Checksum {
				return fmt.Errorf("attunement baseline source mismatch")
			}
			legacy.Archive = direct.Archive
			if err := verifyPVFCatalog(legacy, direct); err != nil {
				return fmt.Errorf("attunement: %w", err)
			}
		}
		c.attunement = direct
		log.Printf("PVF attunement prepared: dungeons=%v templates=%d coupon rows=%d; rebalance and omen policy unchanged", direct.Dungeons(), len(direct.Templates()), direct.Coupons())
		s.ReleaseReadCaches()
	}
	return nil
}

func (c pvfCoreCatalogs) loadAttunement(path string) (*loot.AttunementRewards, error) {
	if c.attunement != nil {
		return c.attunement.Clone()
	}
	return loot.LoadAttunementRewards(path)
}

func (c pvfCoreCatalogs) loadApocalypse(path string) (*catalog.ApocalypseCatalog, error) {
	if c.apocalypse != nil {
		return c.apocalypse, nil
	}
	return catalog.LoadApocalypseCatalog(path)
}
