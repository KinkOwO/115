package main

import (
	"os"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/testfixture"
)

// TestBorderOfAttunementPaysWithAGearPool pins the 2026-09-26 incident.
//
// In that session the player cleared twice and confirmed 23 deaths, and not one
// item dropped. Two independent causes were stacked:
//
//   - the gateway built its loot service without an Equipment catalog, so
//     Roll's equipment branch found an empty pool and skipped every gear award;
//   - dungeon 100005068 declares [exclude gold drop] as its first script cell,
//     so filterDungeonAwards discards every gold award.
//
// What survived was roughly one drop per thousand deaths, which is why a full
// clear looked like "drops are not implemented". The gold half is source
// intent; the gear half was a wiring gap. This test fails if the pool is ever
// dropped again, because a run without it pays essentially nothing.
//
// Opt in with BORDER_DROP_INTEGRATION=1: it loads the 295 MB dungeon catalog.
func TestBorderOfAttunementPaysWithAGearPool(t *testing.T) {
	if os.Getenv("BORDER_DROP_INTEGRATION") != "1" {
		t.Skip("set BORDER_DROP_INTEGRATION=1 to load the 295 MB dungeon catalog")
	}
	dc := catalog.LoadNativeFullDungeons(t)
	lc, err := catalog.LoadLoot(testfixture.LootLevel150Path(t))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := loot.LoadRules("../../configs/drop.compat90.json")
	if err != nil {
		t.Fatal(err)
	}
	tables, err := loot.Parse(lc)
	if err != nil {
		t.Fatal(err)
	}
	gear, err := inventory.LoadEquipmentCatalog("../../configs/equipment.current37.json", lc.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	pool := gear.DropPool()
	if len(pool) == 0 {
		t.Fatal("equipment catalog projected an empty drop pool")
	}

	// The room declares [basis level] 145 and its grade row is [145 45 3], so
	// the selectable window is [100, 148). A pool that cannot reach that window
	// would make the sample below pass only by luck on other levels.
	const level = 145
	window := 0
	for _, d := range pool {
		if d.Grade >= 100 && d.Grade < 148 {
			window++
		}
	}
	if window == 0 {
		t.Fatalf("no gear in the level-%d window [100,148); pool has %d drops", level, len(pool))
	}

	// Sample whole runs. With the pool wired in the room pays roughly 0.3 drops
	// per run; without it, only the sub-percent stackable branches survive, so
	// a total below the threshold is the incident coming back.
	const runs, threshold = 60, 5
	total, payingRuns := 0, 0
	for i := 0; i < runs; i++ {
		s, err := dungeon.Select(dc, protocol.DungeonSelection{ID: 100005068, Difficulty: 1, Party: 65535}, 115, nil)
		if err != nil {
			t.Fatal(err)
		}
		s.Loaded = true
		l := loot.NewSession(lc, tables, rules, gear, s.RunID, 1, 11, 11)
		paid := 0
		for _, m := range s.Monsters {
			if _, e := s.ConfirmDeath(uint32(m.Entity), 11, 11); e != nil {
				continue
			}
			if rows, e := l.Death(s, m.Entity); e == nil {
				paid += len(rows)
			}
		}
		total += paid
		if paid > 0 {
			payingRuns++
		}
	}
	if total < threshold {
		t.Fatalf("%d runs of dungeon 100005068 paid only %d drops (want >= %d): "+
			"the gear pool is probably not wired into the loot session again",
			runs, total, threshold)
	}
	t.Logf("%d runs: %d drops, %d paying runs; pool %d (grade %s window %d) on level %d",
		runs, total, payingRuns, len(pool), "1..107", window, level)
}
