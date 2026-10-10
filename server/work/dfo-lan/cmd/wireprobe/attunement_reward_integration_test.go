package main

import (
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/testfixture"
)

// attunementMaterial is the face the epic fixed reward pays unconditionally: pool
// 0 of 10419728 is a single candidate of 70 units of this material. That makes it
// a fingerprint of the attunement line - the generic roll pays stackables one at
// a time (see Roll in internal/loot/rules.go), never seventy - so a non-boss
// monster carrying it would prove the payout leaked beyond the source boss.
//
// It is not unique to pool 0: the fx=5 branch wraps the same 70-material entry, so
// a run can legitimately carry it twice. The assertion is therefore "at least one
// on the boss, none anywhere else", not "exactly one".
const attunementMaterial = 10362432

// TestAttunementBossPaysTheUnwrappedRewards pins the payoff of the boundary-of-
// attunement reward line end to end: a confirmed source-boss death inside the
// real dungeon 100005068 must hand the run its exclusive rewards, and what
// reaches the ground must be the *contents* of the reward wrappers, not the
// wrappers themselves.
//
// The reward comes from the source rewardboostinfo CTP, keyed by the dungeon the
// file declares, and is bound to the [hunt boss] template the dungeon script
// names. Three halves are asserted here: the CTP set must cover every dungeon our
// catalog identifies as boundary-of-attunement (or a run would be silently
// unpayable), every wrapper it pays must be one the box catalog can open, and the
// ground rows must be the unwrapped set - the equipment as equipment, the oath
// prize as the random book that picks it, the star-stone inside its jar.
//
// Opt in with ATTUNEMENT_REWARD_INTEGRATION=1: it loads the 295 MB dungeon
// catalog.
// dungeonType 走 catalog 的同一个读取器（生产路径用的也是它，避免两份实现）。
func dungeonType(d catalog.DungeonDefinition) string { return catalog.DungeonType(d) }

func TestAttunementBossPaysTheUnwrappedRewards(t *testing.T) {
	if os.Getenv("ATTUNEMENT_REWARD_INTEGRATION") != "1" {
		t.Skip("set ATTUNEMENT_REWARD_INTEGRATION=1 to load the 295 MB dungeon catalog")
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

	// Every dungeon the script marks as boundary-of-attunement must be payable.
	covered := map[uint32]bool{}
	for _, id := range a.Dungeons() {
		covered[id] = true
	}
	marked := 0
	for id, d := range dc.Dungeons {
		// SourceBoss is declared by every hunt-boss dungeon now, so the mode has
		// to be read from the script's own [dungeon type] rather than used as a
		// proxy for it.
		if dungeonType(d) != "boundary of attunement" {
			continue
		}
		marked++
		if !covered[id] {
			t.Errorf("dungeon %d declares [dungeon type] boundary of attunement with source boss %d but the reward table does not name it", id, d.SourceBoss)
		}
	}
	if marked == 0 {
		t.Fatal("no dungeon in the catalog is identified as boundary of attunement")
	}
	if !covered[100005068] {
		t.Fatal("dungeon 100005068 is not in the reward table")
	}

	// Every wrapper a clear can pay must be openable at drop time.
	empties, unopenable, err := a.ValidateBoxes(boxes)
	if err != nil {
		t.Fatalf("reward table names a wrapper the box catalog cannot open: %v", err)
	}
	if len(empties) != 1 || empties[0] != 12 {
		t.Fatalf("empty-face templates = %v, want [12] - the reserved id the source spends on \"no prize\"", empties)
	}
	// The selection boxes need the player to choose, which this build cannot do
	// for them, so they are reported and deliberately not paid.
	if len(unopenable) == 0 {
		t.Fatal("the tree should hold a selection box this build cannot open")
	}
	for _, id := range unopenable {
		if !boxes.Container(id) {
			t.Fatalf("unopenable template %d is not a container", id)
		}
	}

	// Anything the source marks as a box must stay off the ground, whether it
	// is one of the reward entries or a wrapper nested inside one.
	boxesSet := map[uint32]bool{}
	for _, id := range a.Templates() {
		boxesSet[id] = true
	}
	// The oath prize reaches the ground as gear (the book that picks it is a
	// wrapper), so "random" is counted on the gear family. The star-stone is the
	// leaf of the primestella jar, which is why its id, not the jar's, is what
	// must show up.
	oathFamily := func(id uint32) bool {
		it, ok := bc.Items[id]
		return ok && strings.Contains(it.Path, "common/oath/")
	}
	const (
		primerEquipment = 100401592 // the gear the fixed wrapper pays directly
		starStone       = 10403609  // the primestella jar's contents
	)

	const runs = 60
	paidRuns, wrapperRows := 0, 0
	distinctOath, sawEquipment, sawStarStone := map[uint32]bool{}, false, false
	minRows := 1 << 30
	for i := 0; i < runs; i++ {
		s, err := dungeon.Select(dc, protocol.DungeonSelection{ID: 100005068, Difficulty: 1, Party: 65535}, 115, nil)
		if err != nil {
			t.Fatal(err)
		}
		s.Loaded = true
		l := loot.NewSession(lc, tables, rules, gear, s.RunID, 1, 11, 11)
		l.Attunement = a
		l.RewardBoxes = boxes

		boss := s.Definition.SourceBoss
		if boss == 0 {
			t.Fatal("dungeon 100005068 lost its source boss")
		}
		bossRows, bossMaterial := 0, 0
		for _, m := range s.Monsters {
			if _, e := s.ConfirmDeath(uint32(m.Entity), 11, 11); e != nil {
				continue
			}
			rows, e := l.Death(s, m.Entity)
			if e != nil {
				continue
			}
			own := 0
			for _, r := range rows {
				// Scene rows carry the template at +2 and, for a stackable, the
				// amount at +6 (see protocol.OrdinaryItem).
				template := binary.LittleEndian.Uint32(r.Item[2:6])
				amount := binary.LittleEndian.Uint32(r.Item[6:10])
				if boxesSet[template] || boxes.Container(template) {
					wrapperRows++
				}
				if template == attunementMaterial && amount == 70 {
					own++
				}
				if template == primerEquipment {
					sawEquipment = true
				}
				if template == starStone {
					sawStarStone = true
				}
				if oathFamily(template) {
					distinctOath[template] = true
				}
			}
			if m.Template == boss {
				bossRows += len(rows)
				bossMaterial += own
			} else if own > 0 {
				t.Fatalf("run %d: non-boss template %d paid the attunement material row", i, m.Template)
			}
		}
		// The fixed wrapper pays four pools and the additional branch at least
		// one draw; two of the fixed pools are partly empty faces, so the floor
		// is three rows.
		if bossRows < 3 {
			t.Fatalf("run %d: the source boss paid %d rows, want >= 3", i, bossRows)
		}
		if bossMaterial < 1 {
			t.Fatalf("run %d: the source boss paid no material row; the fixed wrapper always pays pool 0", i)
		}
		if bossRows < minRows {
			minRows = bossRows
		}
		paidRuns++
	}
	if paidRuns != runs {
		t.Fatalf("%d of %d runs paid nothing", runs-paidRuns, runs)
	}
	if wrapperRows != 0 {
		t.Fatalf("%d reward boxes reached the ground over %d runs: the unwrap did not finish", wrapperRows, runs)
	}
	if len(unopenable) > 0 {
		t.Logf("%d box(es) this build cannot open are reported rather than paid: %v", len(unopenable), unopenable)
	}
	if !sawEquipment {
		t.Fatalf("no equipment dropped over %d runs: the fixed wrapper's gear face is %d", runs, primerEquipment)
	}
	if len(distinctOath) < 2 {
		t.Fatalf("only %d distinct oath pieces over %d runs: the oath prize never looked random", len(distinctOath), runs)
	}
	if !sawStarStone {
		t.Fatalf("the star-stone %d never dropped over %d runs", starStone, runs)
	}
	t.Logf("%d runs: >=%d rows from the boss, %d distinct oath pieces, equipment=%v star-stone=%v; marked dungeons %d",
		runs, minRows, len(distinctOath), sawEquipment, sawStarStone, marked)
}
