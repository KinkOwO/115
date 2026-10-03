package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
)

// endkeeperDungeon is the small abyss "最终调律者" (endkeeper of order).
const endkeeperDungeon = 100005014

// countByTemplate renders a template histogram in a stable order, so a failure
// reports what was actually paid instead of only that nothing was.
func countByTemplate(counts map[uint32]int) []string {
	out := make([]string, 0, len(counts))
	for id, n := range counts {
		out = append(out, fmt.Sprintf("%d x%d", id, n))
	}
	sort.Strings(out)
	return out
}

// boxContents collects every template the given wrappers can reach, so the
// assertion below can be stated as "this row could only have come from the
// endkeeper table" instead of hard-coding a fingerprint the source may re-cut.
func boxContents(boxes boosterBoxSource, ids []uint32) map[uint32]bool {
	out := map[uint32]bool{}
	var walk func(id uint32, depth int)
	walk = func(id uint32, depth int) {
		if depth > 4 {
			return
		}
		box, ok := boxes.RewardBox(id)
		if !ok {
			out[id] = true
			return
		}
		for _, pool := range box.Pools {
			for _, c := range pool.Candidates {
				walk(c.Template, depth+1)
			}
		}
	}
	for _, id := range ids {
		walk(id, 0)
	}
	return out
}

// TestEndkeeperBossPaysTheUnwrappedRewards is the small abyss counterpart of
// TestAttunementBossPaysTheUnwrappedRewards, pinning the wiring added on
// 2026-09-27: dungeon 100005014 has its own rewardboostinfo table (imported from
// etc/rewardboostinfo/endkeeperoforder/normal.ctp through the generator's
// -extra), keyed by the dungeon the file declares and bound to the [hunt boss]
// template the dungeon script names - which here is the scale (109019266), the
// very fixture the server now has to declare dead itself.
//
// Three things are asserted that a unit test on the decoded table cannot show:
//
//  1. the payout is reachable from a real run - the roll is gated on
//     monster.Rank == 3 && monster.Template == SourceBoss, and the scale has to
//     satisfy both;
//  2. it really is the table that pays - the fingerprint is gear the generic
//     drop pool provably refuses (EquipmentCatalog.Basic), so a row carrying one
//     could not have come from anywhere else;
//  3. it is exclusive and unwrapped - no other monster may carry a fingerprint,
//     and no wrapper may reach the ground.
//
// Opt in with ATTUNEMENT_REWARD_INTEGRATION=1: it loads the 295 MB dungeon
// catalog.
func TestEndkeeperBossPaysTheUnwrappedRewards(t *testing.T) {
	if os.Getenv("ATTUNEMENT_REWARD_INTEGRATION") != "1" {
		t.Skip("set ATTUNEMENT_REWARD_INTEGRATION=1 to load the 295 MB dungeon catalog")
	}
	dc := catalog.LoadNativeFullDungeons(t)
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
	bc, err := catalog.LoadBoosterCatalog("../../internal/catalog/testdata/booster-flow.json", "../../internal/catalog/testdata/booster-items.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(bc.Definitions) == 0 || len(bc.Items) == 0 {
		t.Fatalf("booster catalog came back empty (%d definitions, %d items): the paths are wrong", len(bc.Definitions), len(bc.Items))
	}
	boxes := boosterBoxSource{catalog: bc}

	covered := map[uint32]bool{}
	for _, id := range a.Dungeons() {
		covered[id] = true
	}
	if !covered[endkeeperDungeon] {
		t.Fatalf("dungeon %d is not in the reward table: the generator ran without -extra?", endkeeperDungeon)
	}
	_, unopenable, err := a.ValidateBoxes(boxes)
	if err != nil {
		t.Fatalf("reward table names a wrapper the box catalog cannot open: %v", err)
	}

	// The wrappers this dungeon's own lists name, and everything they can turn
	// into.
	var ownWrappers []uint32
	for i := range a.Tables {
		tb := &a.Tables[i]
		if tb.Dungeon != endkeeperDungeon {
			continue
		}
		for _, f := range tb.Fixed {
			for _, e := range f.Entries {
				ownWrappers = append(ownWrappers, e.Item)
			}
		}
		for _, at := range tb.Additional {
			for _, e := range at.Entries {
				ownWrappers = append(ownWrappers, e.Item)
			}
		}
	}
	if len(ownWrappers) == 0 {
		t.Fatalf("dungeon %d carries no payable wrapper", endkeeperDungeon)
	}
	contents := boxContents(boxes, ownWrappers)

	// A fingerprint is a template the generic roll can never pay: equipment the
	// drop pool refuses (row 20xx of the endkeeper wrappers is primer gear with
	// [attach type] [trade], so the pool's [free]-only rule excludes it). It is
	// derived, not hard-coded, so re-cutting the source cannot silently turn the
	// assertion into a tautology.
	fingerprints := map[uint32]bool{}
	for id := range contents {
		it, ok := bc.Items[id]
		if !ok || it.Kind != "equipment" {
			continue
		}
		if _, e := gear.Basic(id); e != nil {
			fingerprints[id] = true
		}
	}
	if len(fingerprints) == 0 {
		t.Fatal("no wrapper content is unreachable from the generic pool: this test would prove nothing")
	}
	wrappers := map[uint32]bool{}
	for _, id := range a.Templates() {
		wrappers[id] = true
	}

	const runs = 40
	fingerprintRows, minRows := 0, 1<<30
	distinct := map[uint32]int{}
	for i := 0; i < runs; i++ {
		s, err := dungeon.Select(dc, protocol.DungeonSelection{ID: endkeeperDungeon, Difficulty: 1, Party: 65535}, 115, nil)
		if err != nil {
			t.Fatal(err)
		}
		s.Loaded = true
		boss := s.Definition.SourceBoss
		if boss == 0 {
			t.Fatalf("dungeon %d lost its source boss (the [clear condition] [hunt boss] reading)", endkeeperDungeon)
		}
		l := loot.NewSession(lc, tables, rules, gear, s.RunID, 1, 11, 11)
		l.Attunement = a
		l.RewardBoxes = boxes

		bossRows := 0
		for _, m := range s.Monsters {
			if _, e := s.ConfirmDeath(uint32(m.Entity), 11, 11); e != nil {
				continue
			}
			rows, e := l.Death(s, m.Entity)
			if e != nil {
				if m.Template == boss {
					t.Fatalf("run %d: the source boss paid nothing but an error: %v", i, e)
				}
				continue
			}
			onBoss := m.Template == boss
			for _, r := range rows {
				template := binary.LittleEndian.Uint32(r.Item[2:6])
				if wrappers[template] || boxes.Container(template) {
					t.Fatalf("run %d: monster %d dropped wrapper %d: the unwrap did not finish", i, m.Template, template)
				}
				if !fingerprints[template] {
					continue
				}
				if !onBoss {
					t.Fatalf("run %d: non-boss template %d paid %d, which only the endkeeper table can produce",
						i, m.Template, template)
				}
				fingerprintRows++
				distinct[template]++
			}
			if onBoss {
				bossRows += len(rows)
			}
		}
		if bossRows == 0 {
			t.Fatalf("run %d: the source boss paid no row at all", i)
		}
		if bossRows < minRows {
			minRows = bossRows
		}
	}
	if fingerprintRows == 0 {
		t.Fatalf("no table-only reward dropped in %d runs: the endkeeper table never paid", runs)
	}
	t.Logf("%d runs: >=%d rows per clear from the boss, %d table-only row(s) over %d distinct template(s) %v; %d wrappers this build cannot open %v",
		runs, minRows, fingerprintRows, len(distinct), countByTemplate(distinct), len(unopenable), unopenable)
}
