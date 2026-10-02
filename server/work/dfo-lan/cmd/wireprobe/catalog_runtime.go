package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/quest"
	"fmt"
	"log"
	"path/filepath"
)

// Runtime adapters bind prepared data to gateway-only reward and lottery rules.
func runtimeCatalogAdapters() gamedata.CatalogAdapters {
	return gamedata.CatalogAdapters{
		ValidatedSource: func(checksum string) {
			catalog.SetOdysseySource(checksum)
			SetLotterySource(checksum)
			inventory.SetClearCubeSource(checksum)
			quest.SetImageCommunicationSource(checksum)
		},
		ValidateLottery: validatePreparedLottery,
		RewardBoxes: func(definitions map[uint32]catalog.BoosterDefinition, items map[uint32]catalog.ItemIndexEntry) loot.RewardBoxSource {
			return boosterBoxSource{catalog: &BoosterCatalog{Definitions: definitions, Items: items}}
		},
	}
}

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

func validatePreparedLottery(tables catalog.LotteryTables, index catalog.ItemIndex, baselineDir string, verify bool) error {
	bound, err := buildLotteryItemCatalog(lotteryItemsFromSource(tables.Items), index.Items, false)
	if err != nil {
		return err
	}
	if _, err = bindLotteryEquipmentPools(tables.Equipment, index.Items, bound, false); err != nil {
		return err
	}
	if !verify {
		return nil
	}
	old, err := loadLotteryItemCatalog(filepath.Join(baselineDir, "lottery-item-pools.json"), index.Items)
	if err != nil {
		return err
	}
	if _, err = loadLotteryEquipmentPools(filepath.Join(baselineDir, "lottery-equipment-pools.json"), index.Items, old); err != nil {
		return err
	}
	for _, pair := range [][2]any{{old.Pools, bound.Pools}, {old.byTemplate, bound.byTemplate}} {
		comparison := gamedata.Compare(pair[0], pair[1], 1)
		if comparison.Count != 0 {
			first := comparison.Differences[0]
			log.Printf("baseline vs PVF direct: %d effective field difference(s); first %s: JSON=%s PVF=%s (historical snapshot; not fatal)", comparison.Count, first.Path, first.JSON, first.PVF)
		}
	}
	for id, pool := range old.byTemplate {
		current := bound.byTemplate[id]
		if current == nil || pool.total != current.total {
			return fmt.Errorf("lottery total weight changed for %d", id)
		}
	}
	return nil
}

func loadRuntimeLotteryItems(c *gamedata.Catalogs, path string, index map[uint32]ItemIndexInfo) (*lotteryItemCatalog, error) {
	source, err := c.LoadLotteryItemPools(path)
	if err != nil {
		return nil, err
	}
	return buildLotteryItemCatalog(lotteryItemsFromSource(source), index, c.LotteryTables == nil)
}

func loadRuntimeLotteryEquipment(c *gamedata.Catalogs, path string, index map[uint32]ItemIndexInfo, base *lotteryItemCatalog) (int, error) {
	if c.LotteryTables == nil && (base == nil || !lotterySourceAccepts(base.SourcePVFSHA256)) {
		return 0, fmt.Errorf("lottery base catalog unavailable")
	}
	source, err := c.LoadLotteryEquipmentPools(path)
	if err != nil {
		return 0, err
	}
	return bindLotteryEquipmentPools(source, index, base, c.LotteryTables == nil)
}
