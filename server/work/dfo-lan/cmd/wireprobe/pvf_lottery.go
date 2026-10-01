package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"fmt"
	"log"
	"path/filepath"
)

func lotteryItemsFromSource(source catalog.LotteryPoolCatalog) lotteryItemCatalog {
	c := lotteryItemCatalog{SourcePVFSHA256: source.SourcePVFSHA256}
	for _, row := range source.Pools {
		p := &lotteryItemPool{SourceItem: row.SourceItem, SourceScript: row.SourceScript, SourceScriptSHA256: row.SourceScriptSHA256}
		for _, triple := range row.Candidates {
			p.Candidates = append(p.Candidates, BoosterRewardCandidate{Template: triple[0], Weight: triple[1], Count: triple[2]})
		}
		c.Pools = append(c.Pools, p)
	}
	return c
}

func preparePVFLottery(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["lottery"] {
		return nil
	}
	if c.items == nil {
		return fmt.Errorf("native lotteries require native item index")
	}
	p, err := catalog.ReadLotteryPolicy(i.lotteryPolicyPath)
	if err != nil {
		return err
	}
	direct, err := s.Lottery(*c.items, p)
	if err != nil {
		return err
	}
	bound, err := newLotteryItemCatalog(lotteryItemsFromSource(direct.Items), c.items.Items)
	if err != nil {
		return err
	}
	if _, err := applyLotteryEquipmentPools(direct.Equipment, c.items.Items, bound); err != nil {
		return err
	}
	if i.checksBaselines() {
		dir := filepath.Dir(i.indexPath)
		old, err := loadLotteryItemCatalog(filepath.Join(dir, "lottery-item-pools.json"), c.items.Items)
		if err != nil {
			return err
		}
		if _, err := loadLotteryEquipmentPools(filepath.Join(dir, "lottery-equipment-pools.json"), c.items.Items, old); err != nil {
			return err
		}
		if err := verifyPVFCatalog(old.Pools, bound.Pools); err != nil {
			return fmt.Errorf("lottery item pools: %w", err)
		}
		if err := verifyPVFCatalog(old.byTemplate, bound.byTemplate); err != nil {
			return fmt.Errorf("lottery complete pools: %w", err)
		}
		for id, pool := range old.byTemplate {
			if pool.total != bound.byTemplate[id].total {
				return fmt.Errorf("lottery total weight changed for %d", id)
			}
		}
	}
	c.lotteryTables = &direct
	s.ReleaseReadCaches()
	log.Printf("PVF lotteries prepared: item pools=%d equipment pools=%d; source odds and grantable server scope retained", len(direct.Items.Pools), len(direct.Equipment.Pools))
	return nil
}

func (c pvfCoreCatalogs) loadLotteryItems(path string, index map[uint32]ItemIndexInfo) (*lotteryItemCatalog, error) {
	if c.lotteryTables != nil {
		return newLotteryItemCatalog(lotteryItemsFromSource(c.lotteryTables.Items), index)
	}
	return loadLotteryItemCatalog(path, index)
}

func (c pvfCoreCatalogs) loadLotteryEquipment(path string, index map[uint32]ItemIndexInfo, base *lotteryItemCatalog) (int, error) {
	if c.lotteryTables != nil {
		return applyLotteryEquipmentPools(c.lotteryTables.Equipment, index, base)
	}
	return loadLotteryEquipmentPools(path, index, base)
}
