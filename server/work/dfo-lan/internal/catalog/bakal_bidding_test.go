package catalog

import "testing"

func TestBakalBiddingBindsNativeNormalHardAndWeeklyPools(t *testing.T) {
	r, e := ImportBakalRaid(OpenNativeArchive(t))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { r.DungeonCatalog.CloseMapSource() })
	if len(r.Bidding.Items) != 2 || r.Bidding.Items[0].Template != 10337823 || r.Bidding.Items[1].Template != 10342710 || r.Bidding.Items[1].Difficulty != 1 {
		t.Fatalf("native bidding pools: %+v", r.Bidding.Items)
	}
	if len(r.Bidding.WeeklyCounts) != 2 || len(r.Bidding.WeeklyRates) != 3 || len(r.Bidding.WeeklyGroups) != 3 {
		t.Fatal("weekly pool sections missing")
	}
	if got := r.Bidding.WeeklyGroups[3]; len(got) != 4 || got[0].Template != 10341753 || got[0].Weight != 2500 {
		t.Fatalf("weekly tuple not preserved: %+v", got)
	}
}
