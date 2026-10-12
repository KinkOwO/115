package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestNativeCardSourceWeightsAndFloatPreservation(t *testing.T) {
	fixed := 5.0
	s := &NativeLegionOperations{Modes: map[string][]uint16{"normal": {1, 2}}, Definitions: map[uint16]NativeLegionOperation{
		1: {SelectPool: 0, MemberLimit: "party", Probability: 1, Ranges: []NativeLegionRange{{"party", "normal", 1.5, 1.5, 0}}},
		2: {SelectPool: 0, MemberLimit: "party", Probability: 3, FixedValue: &fixed, Ranges: []NativeLegionRange{{"party", "normal", 30, 40, 5}}},
	}}
	first := func(n int64) (int64, error) { return 0, nil }
	c, err := s.DrawCard("normal", "party", "normal", 0, first)
	if err != nil || c.Index != 1 || c.Parameter != 1.5 {
		t.Fatal(c, err)
	}
	last := func(n int64) (int64, error) { return n - 1, nil }
	c, err = s.DrawCard("normal", "party", "normal", 0, last)
	if err != nil || c.Index != 2 || c.Parameter != 40 || c.FixedValue == nil || *c.FixedValue != 5 {
		t.Fatal(c, err)
	}
	fixed = 9
	if *c.FixedValue != 5 {
		t.Fatal("frozen source operand changed")
	}
	d := s.Definitions[1]
	d.Probability = 0
	s.Definitions[1] = d
	c, err = s.DrawCard("normal", "party", "normal", 0, first)
	if err != nil || c.Index != 2 || c.Parameter != 30 {
		t.Fatal("source weight change ignored", c, err)
	}
	if _, err = s.DrawCard("missing", "party", "normal", 0, first); err == nil {
		t.Fatal("missing mode silently substituted")
	}
}
func TestNativeContinuousRangeNeverChangesOperationLottery(t *testing.T) {
	d := NativeLegionOperation{Ranges: []NativeLegionRange{{"party", "normal", 5, 10, 0}}}
	if _, err := d.DrawNativeParameter("party", "normal", func(n int64) (int64, error) { return 0, nil }); err == nil {
		t.Fatal("unbound continuous range coerced to integer step")
	}
}

func TestStagedLegionSourceRotation(t *testing.T) {
	c := LegionContent{LastPhase: 2, NativeOperations: &NativeLegionOperations{WeeklyDungeons: [][]uint32{{3, 2, 1}, {4, 3, 2}}}}
	for i := 0; i < 5; i++ {
		c.Dungeons = append(c.Dungeons, LegionDungeon{Dungeon: uint32(100 + i), Boss: uint32(200 + i)})
	}
	d, err := StagedLegionDungeons(c, 1)
	if err != nil || d[0].Dungeon != 104 || d[2].Dungeon != 102 {
		t.Fatal(d, err)
	}
	c.NativeOperations.WeeklyDungeons[1][0] = 0
	d, err = StagedLegionDungeons(c, 1)
	if err != nil || d[0].Dungeon != 100 {
		t.Fatal("source rotation ignored", d, err)
	}
	if _, err = StagedLegionDungeons(c, -1); err == nil {
		t.Fatal("missing weekly row silently defaulted")
	}
	c.NativeOperations.WeeklyDungeons = nil
	d, err = StagedLegionDungeons(c, -1)
	if err != nil || d[2].Dungeon != 102 {
		t.Fatal(d, err)
	}
}

func TestNativePhaseRewardsFollowSourceAndMode(t *testing.T) {
	s := &LegionContents{Source: pvf.ArchiveSnapshot{Checksum: "fixture"}, Contents: map[string]LegionContent{"DuskyIsland": {
		ProbabilityRate: 100, PhaseReward: 1, Dungeons: []LegionDungeon{{Dungeon: 10}}, Rewards: []LegionRewardRow{{PartyType: "solo", Difficulty: "normal", Kind: "basic", Values: []int64{100, 20, 2}}}, NativeOperations: &NativeLegionOperations{Definitions: map[uint16]NativeLegionOperation{9: {Rewards: []NativeLegionReward{{Difficulty: "normal", Template: 30, Amount: 3, Weight: 50, Tier: 2, Account: 1}}}}},
	}}}
	r, err := NativePhaseRewards(s, "DuskyIsland", 10, 9, "normal", "normal")
	if err != nil || r.Basic[0].Amount != 2 || r.Additional[0].Amount != 3 || r.Additional[0].Grade != 2 || r.Additional[0].Extra != 1 {
		t.Fatal(r, err)
	}
	c := s.Contents["DuskyIsland"]
	c.Rewards[0].Values[2] = 7
	s.Contents["DuskyIsland"] = c
	r, err = NativePhaseRewards(s, "DuskyIsland", 10, 9, "normal", "normal")
	if err != nil || r.Basic[0].Amount != 7 {
		t.Fatal("source quantity ignored", r, err)
	}
	for _, mode := range []string{"expert", "pcroom", ""} {
		if _, err = NativePhaseRewards(s, "DuskyIsland", 10, 9, mode, "normal"); err == nil {
			t.Fatal("unearned source mode substituted", mode)
		}
	}
	if _, err = NativePhaseRewards(s, "DuskyIsland", 11, 9, "normal", "normal"); err == nil {
		t.Fatal("foreign dungeon rewarded")
	}
}
