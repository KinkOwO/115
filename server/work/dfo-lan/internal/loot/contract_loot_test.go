package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"testing"
)

func TestGrowthContractQuestDropBonus(t *testing.T) {
	c, e := catalog.LoadLoot("../../configs/loot.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	tables, e := Parse(c)
	if e != nil {
		t.Fatal(e)
	}
	rules, e := LoadRules("../../configs/drop.current36.json")
	if e != nil {
		t.Fatal(e)
	}
	gear, e := inventory.LoadEquipmentCatalog("../../configs/equipment.current35.json", c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	pool := gear.DropPool()

	normalDrops := 0
	boostedDrops := 0
	const samples = 10000

	for seed := uint32(1); seed <= samples; seed++ {
		outNormal, err := Roll(c, tables, rules, pool, seed, 25, 1, 0)
		if err != nil {
			t.Fatal(err)
		}
		normalDrops += len(outNormal.Awards)

		outBoosted, err := RollWithBonus(c, tables, rules, pool, seed, 25, 1, 0, 20)
		if err != nil {
			t.Fatal(err)
		}
		boostedDrops += len(outBoosted.Awards)
	}

	if boostedDrops < normalDrops {
		t.Fatalf("expected boosted drops (%d) >= normal drops (%d)", boostedDrops, normalDrops)
	}
	t.Logf("PASS growth contract drop rate: normal=%d, boosted=%d across %d seeds", normalDrops, boostedDrops, samples)
}
