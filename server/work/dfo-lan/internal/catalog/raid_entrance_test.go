package catalog

import (
	"os"
	"testing"
)

func TestRaidEntranceNativePVF(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF archive required")
	}
	a, err := OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	d, err := ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := ImportRaidEntrances(a, &d)
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range []uint32{82, 98, 107} {
		r, ok := rules[ch]
		if !ok {
			t.Fatalf("channel %d has no native entry", ch)
		}
		if r.MemberMax != 12 || r.StartMinimum != 1 || r.PartyMax != 3 || r.Waiting.AreaID != 2 {
			t.Fatalf("unexpected entry %d: %+v", ch, r)
		}
		x, y := r.Waiting.Spawn()
		if x == 0 && y == 0 {
			t.Fatalf("missing spawn %d", ch)
		}
		if ch == 82 {
			b := r.Bakal
			bid := b.Bidding
			if len(bid.Count) != 2 || bid.Count[0] != (BakalWeightedNumber{1, 30000}) || bid.Count[1] != (BakalWeightedNumber{2, 70000}) || len(bid.WeeklyCount) != 2 || bid.WeeklyCount[0] != (BakalWeightedNumber{0, 70000}) || bid.WeeklyCount[1] != (BakalWeightedNumber{1, 30000}) {
				t.Fatalf("normal auction weights merged with hard mode: %+v", bid)
			}
			if len(bid.WeeklyGroups) != 3 || len(bid.WeeklyItems[1]) != 13 || len(bid.WeeklyItems[2]) != 6 || len(bid.WeeklyItems[3]) != 4 || bid.StartDelay != 10 || bid.BreakTime != 3 || bid.Time != 8 || bid.LowestTime != 5 || bid.DecreaseAfterBids != 3 || bid.DecreasePerBid != 1 || len(bid.RewardTimes) != 13 || len(bid.GoldCards) != 9 {
				t.Fatalf("native auction source incomplete: %+v", bid)
			}
			if b == nil || b.PhaseMax != 1 || b.StartDelay != 3 || b.TimeLimit != 9999 || len(b.Dungeons) != 17 || len(b.Monsters) != 8 || len(b.ReservedMonsters) != 3 || len(b.Buffs) != 5 || len(b.Timers) != 4 {
				t.Fatalf("native opening phase incomplete: %+v", b)
			}
			if len(b.Slots) != 54 || b.Slots[52].TownArea != 2 || b.Slots[53].TownArea != 3 || b.Slots[54].TownArea != 4 || b.Slots[24].Dungeon != 100003149 || b.MonsterDefinitions["bakal"].ID != 109014482 {
				t.Fatalf("native battlefield mapping mismatch: %+v", b)
			}
			// Actual camp CMD2062 names this battlefield and entry grid0/3.
			var dungeonIDs []uint32
			for _, d := range b.Dungeons {
				dungeonIDs = append(dungeonIDs, d.ID)
			}
			battlefield, err := ImportDungeons(a, dungeonIDs)
			if err != nil {
				t.Fatal(err)
			}
			if len(battlefield.Dungeons) != len(dungeonIDs) {
				t.Fatalf("native raid dungeons skipped: %v", battlefield.Skipped)
			}
			definition, ok := battlefield.Dungeons[100003162]
			if !ok || len(definition.Mazes) == 0 || definition.Mazes[0].Start != [2]byte{3, 0} {
				t.Fatalf("captured portal grid/source mismatch: definition=%+v skipped=%+v", definition, battlefield.Skipped)
			}
			t.Logf("Bakal phase: %d dungeons, %d slots, %d monster definitions, %d initial monsters", len(b.Dungeons), len(b.Slots), len(b.MonsterDefinitions), len(b.Monsters))
		}
		t.Logf("channel %d source %s waiting %d/%d limits %d/%d/%d", ch, r.Path, r.Waiting.TownID, r.Waiting.AreaID, r.MemberMax, r.StartMinimum, r.PartyMax)
	}
}
