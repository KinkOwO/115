package main

import (
	"encoding/binary"
	"os"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
)

// TestAttunementBossPaysTheExclusiveBoxes pins the payoff of the boundary-of-
// attunement reward line end to end: a confirmed source-boss death inside the
// real dungeon 100005068 must hand the run its exclusive boxes, and no other
// monster in the room may hand out an exclusive box at all.
//
// The reward comes from the source rewardboostinfo CTP, keyed by the dungeon the
// file declares, and is bound to the [hunt boss] template the dungeon script
// names. Both halves are asserted here: the CTP set must cover every dungeon our
// catalog identifies as boundary-of-attunement (or a run would be silently
// unpayable), and the payout must be bound to the declared boss rather than to
// "some rank 3 in the room".
//
// Opt in with ATTUNEMENT_REWARD_INTEGRATION=1: it loads the 295 MB dungeon
// catalog.
func TestAttunementBossPaysTheExclusiveBoxes(t *testing.T) {
	if os.Getenv("ATTUNEMENT_REWARD_INTEGRATION") != "1" {
		t.Skip("set ATTUNEMENT_REWARD_INTEGRATION=1 to load the 295 MB dungeon catalog")
	}
	dc, err := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if err != nil {
		t.Fatal(err)
	}
	lc, err := catalog.LoadLoot("../../configs/loot.level150.json")
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
	a, err := loot.LoadAttunementRewards("../../configs/attunement-rewards.generated.json")
	if err != nil {
		t.Fatal(err)
	}

	// Every dungeon the script marks as boundary-of-attunement must be payable.
	covered := map[uint32]bool{}
	for _, id := range a.Dungeons() {
		covered[id] = true
	}
	marked := 0
	for id, d := range dc.Dungeons {
		if d.AttunementBoss == 0 {
			continue
		}
		marked++
		if !covered[id] {
			t.Errorf("dungeon %d declares [dungeon type] boundary of attunement with boss %d but the reward table does not name it", id, d.AttunementBoss)
		}
	}
	if marked == 0 {
		t.Fatal("no dungeon in the catalog is identified as boundary of attunement")
	}
	if !covered[100005068] {
		t.Fatal("dungeon 100005068 is not in the reward table")
	}

	exclusive := map[uint32]bool{}
	for _, id := range a.Templates() {
		exclusive[id] = true
	}

	const runs = 20
	paidRuns, totalBoxes := 0, 0
	distinct := map[uint32]int{}
	for i := 0; i < runs; i++ {
		s, err := dungeon.Select(dc, protocol.DungeonSelection{ID: 100005068, Difficulty: 1, Party: 65535}, 115, nil)
		if err != nil {
			t.Fatal(err)
		}
		s.Loaded = true
		l := loot.NewSession(lc, tables, rules, gear, s.RunID, 1, 11, 11)
		l.Attunement = a

		boss := s.Definition.AttunementBoss
		if boss == 0 {
			t.Fatal("dungeon 100005068 lost its source boss")
		}
		bossBoxes := 0
		for _, m := range s.Monsters {
			if _, e := s.ConfirmDeath(uint32(m.Entity), 11, 11); e != nil {
				continue
			}
			rows, e := l.Death(s, m.Entity)
			if e != nil {
				continue
			}
			boxes := 0
			for _, r := range rows {
				// Scene rows carry the template at +2 on both the ordinary and
				// the equipment layout (see OrdinaryItem / EquipmentRow).
				template := binary.LittleEndian.Uint32(r.Item[2:6])
				if exclusive[template] {
					boxes++
					distinct[template]++
				}
			}
			if m.Template == boss {
				bossBoxes += boxes
			} else if boxes > 0 {
				t.Fatalf("run %d: non-boss template %d paid %d exclusive boxes", i, m.Template, boxes)
			}
		}
		// One fixed box plus one additional branch of 3, 5, 10 or 1.
		if bossBoxes < 2 {
			t.Fatalf("run %d: the source boss paid %d exclusive boxes, want >= 2", i, bossBoxes)
		}
		paidRuns++
		totalBoxes += bossBoxes
	}
	if paidRuns != runs {
		t.Fatalf("%d of %d runs paid nothing", runs-paidRuns, runs)
	}
	if len(distinct) < 5 {
		t.Fatalf("only %d distinct boxes over %d runs: the additional branches may have collapsed", len(distinct), runs)
	}
	t.Logf("%d runs: %d exclusive boxes from the source boss, %d distinct; marked dungeons %d",
		runs, totalBoxes, len(distinct), marked)
}
