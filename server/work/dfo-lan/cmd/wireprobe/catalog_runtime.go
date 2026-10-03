package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/quest"
	"fmt"
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

func validatePreparedLottery(tables catalog.LotteryTables, index catalog.ItemIndex, _ string, _ bool) error {
	bound, err := buildLotteryItemCatalog(lotteryItemsFromSource(tables.Items), index.Items, false)
	if err != nil {
		return err
	}
	_, err = bindLotteryEquipmentPools(tables.Equipment, index.Items, bound, false)
	return err
}

func loadRuntimeLotteryItems(c *gamedata.Catalogs, path string, index map[uint32]ItemIndexInfo) (*lotteryItemCatalog, error) {
	source, err := c.LoadLotteryItemPools(path)
	if err != nil {
		return nil, err
	}
	return buildLotteryItemCatalog(lotteryItemsFromSource(source), index, false)
}

func loadRuntimeLotteryEquipment(c *gamedata.Catalogs, path string, index map[uint32]ItemIndexInfo, base *lotteryItemCatalog) (int, error) {
	if base == nil || !lotterySourceAccepts(base.SourcePVFSHA256) {
		return 0, fmt.Errorf("lottery base catalog unavailable")
	}
	source, err := c.LoadLotteryEquipmentPools(path)
	if err != nil {
		return 0, err
	}
	return bindLotteryEquipmentPools(source, index, base, false)
}
