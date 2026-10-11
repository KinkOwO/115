package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestLegionRewardBlocksRetainPartySourceAndFilter(t *testing.T) {
	text := "[legion data set]\n[contents] `Ispins`\n[reward data]\n[party type] `solo`\n[data]\n`normal` `basic` 100 123 4\n[/data]\n[/reward data]\n[reward data]\n[party type] `party`\n[data]\n`normal` `basic` 200 456 7\n[/data]\n[/reward data]\n[/legion data set]\n"
	contents, err := ParseLegionContents(text)
	if err != nil {
		t.Fatal(err)
	}
	c := contents["Ispins"]
	if len(c.Rewards) != 2 || c.Rewards[0].PartyType != "solo" || c.Rewards[1].PartyType != "party" {
		t.Fatal("lost repeated source block", c.Rewards)
	}
	c.PhaseReward = 2
	c.Dungeons = []LegionDungeon{{Dungeon: 789}}
	source := &LegionContents{Source: pvf.ArchiveSnapshot{Checksum: "fixture"}, Contents: map[string]LegionContent{"Ispins": c}}
	ops := map[uint16]IspinsOperationDefinition{8: {Type: 6, FixedValue: 4, Difficulty: "normal", SelectPool: 2, Rewards: []LegionOperationReward{{Difficulty: "normal", Template: 1000, Amount: 2, Weight: 300}}}}
	r, err := IspinsPhaseRewards(source, ops, 789, 8)
	if err != nil || r.WeeklyLimit != 2 || len(r.Basic) != 1 || r.Basic[0].Template != 123 || r.Basic[0].Amount != 4 || r.Additional[0].Amount != 2 {
		t.Fatal("lost or mixed source mode", r, err)
	}
	c.Rewards[0].Values[2] = 9
	source.Contents["Ispins"] = c
	r, err = IspinsPhaseRewards(source, ops, 789, 8)
	if err != nil || r.Basic[0].Amount != 9 {
		t.Fatal("source quantity did not drive result")
	}
	if _, err = IspinsPhaseRewards(source, ops, 790, 8); err == nil {
		t.Fatal("foreign dungeon accepted")
	}
	if _, err = IspinsPhaseRewards(source, ops, 789, 9); err == nil {
		t.Fatal("unbound operation accepted")
	}
}
