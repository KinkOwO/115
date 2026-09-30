package main

import (
	"os"
	"testing"
)

func TestPVFLotteryLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete lottery source and weighted boundary parity")
	}
	c, err := preparePVFCoreCatalogs("lottery", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", lotteryPolicyPath: "../../configs/pvf-lottery-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := c.loadLotteryItems("missing-items.json", c.items.Items)
	if err != nil {
		t.Fatal(err)
	}
	if count, err := c.loadLotteryEquipment("missing-equipment.json", c.items.Items, direct); err != nil || count != 2477 {
		t.Fatal(count, err)
	}
	old, err := loadLotteryItemCatalog("../../configs/lottery-item-pools.json", c.items.Items)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := loadLotteryEquipmentPools("../../configs/lottery-equipment-pools.json", c.items.Items, old); err != nil {
		t.Fatal(err)
	}
	boundaries := 0
	for id, p := range direct.byTemplate {
		before := old.byTemplate[id]
		if p.total != before.total {
			t.Fatal("total changed", id)
		}
		var offset int64
		for _, row := range p.Candidates {
			for _, draw := range []int64{offset, offset + int64(row.Weight) - 1} {
				a, ea := p.pick(draw)
				b, eb := before.pick(draw)
				if ea != nil || eb != nil || a != b {
					t.Fatal("weighted boundary changed", id, draw, ea, eb)
				}
				boundaries++
			}
			offset += int64(row.Weight)
		}
	}
	direct.byTemplate[7772].Candidates[0].Count++
	again, err := c.loadLotteryItems("missing-items.json", c.items.Items)
	if err != nil || again.byTemplate[7772].Candidates[0] != old.byTemplate[7772].Candidates[0] {
		t.Fatal("prepared source was mutated", err)
	}
	t.Log("complete 276/2477 source pool parity; weighted boundaries checked", boundaries)
}
