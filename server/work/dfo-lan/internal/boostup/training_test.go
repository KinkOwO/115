package boostup

import "testing"

func TestTrainingClaimsDoNotSkipProofOrDuplicateReward(t *testing.T) {
	c := &Catalog{Steps: []Step{
		{Number: 1, Guide: "none", Mission: "equip item", Rewards: []Reward{{1, 1}}},
		{Number: 2, Guide: "dungeon", Dungeon: 10, Mission: "none", Rewards: []Reward{{2, 1}}},
		{Number: 3, Guide: "normal", Mission: "none", Rewards: []Reward{{3, 1}}},
	}}
	s, e := c.Begin()
	if e != nil || s.Phase != 1 {
		t.Fatal(s, e)
	}
	if _, _, e = c.Claim(s, 2, false); e == nil {
		t.Fatal("future reward")
	}
	next, items, e := c.Claim(s, 1, false)
	if e != nil || len(items) != 1 || next.Phase != 2 || s.Claimed[1] {
		t.Fatal("non-atomic candidate mutated input", e)
	}
	again, items, e := c.Claim(next, 1, false)
	if e != nil || len(items) != 0 || again.Step != 1 {
		t.Fatal("duplicate reward")
	}
	if _, e = c.MissionCompleted(next, "disjoint"); e == nil {
		t.Fatal("unrelated inventory action advanced event")
	}
	s, e = c.MissionCompleted(next, "equip item")
	if e != nil || s.Step != 2 || s.Phase != 0 {
		t.Fatal(s, e)
	}
	if _, e = c.GuideViewed(s, 2); e == nil {
		t.Fatal("query skipped training dungeon")
	}
	if _, e = c.GuideDungeonCleared(s, 10, 11); e == nil {
		t.Fatal("wrong dungeon clear")
	}
	s, e = c.GuideDungeonCleared(s, 10, 10)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e = c.Claim(s, 2, false)
	if e != nil || s.Step != 3 {
		t.Fatal(s, e)
	}
	s, e = c.GuideViewed(s, 3)
	if e != nil {
		t.Fatal(e)
	}
	s, _, e = c.Claim(s, 3, false)
	if e != nil || !s.Finished || s.Step != 4 {
		t.Fatal(s, e)
	}
	if _, items, e = c.Claim(s, 3, false); e != nil || len(items) != 0 {
		t.Fatal("graduation duplicated reward")
	}
}
