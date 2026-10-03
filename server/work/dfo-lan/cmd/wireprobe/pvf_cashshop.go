package main

import (
	"dfolan/internal/cashshop"
	"dfolan/internal/gamedata"
	"fmt"
	"log"
)

func preparePVFCashShop(c *pvfCoreCatalogs, s *gamedata.Source, i pvfItemInputs) error {
	raw, err := s.CashShop()
	if err != nil {
		return err
	}
	raw.Release = i.cashshopRelease
	direct, err := cashshop.NewPilot(raw, s.Snapshot().Checksum)
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		old, err := cashshop.LoadPilot(i.cashshopPath, s.Snapshot().Checksum, i.cashshopRelease)
		if err != nil {
			return err
		}
		if err = verifyPVFCatalog(old.Config, direct.Config); err != nil {
			return fmt.Errorf("cashshop source projection: %w", err)
		}
		oldProducts, err := old.ProductSnapshot()
		if err != nil {
			return err
		}
		directProducts, err := direct.ProductSnapshot()
		if err != nil {
			return err
		}
		if err = verifyPVFCatalog(oldProducts, directProducts); err != nil {
			return fmt.Errorf("cashshop effective products: %w", err)
		}
	}
	c.cashshop = direct
	s.ReleaseReadCaches()
	log.Printf("PVF cashshop prepared: rows=%d enabled=%d release=%t; native prices, item cells and purchase policies retained", len(direct.Config.Entries), direct.EnabledCount(), direct.Config.Release)
	return nil
}

func (c pvfCoreCatalogs) loadCashShop(path, source string, release bool) (*cashshop.Pilot, error) {
	if c.cashshop != nil {
		if c.cashshop.Config.Source.Checksum != source || c.cashshop.Config.Release != release {
			return nil, fmt.Errorf("prepared cashshop source/release settings changed")
		}
		return c.cashshop, nil
	}
	return cashshop.LoadPilot(path, source, release)
}
