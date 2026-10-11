package catalog

import "testing"

func TestIspinsStageIDsFollowSourceAndRejectUnboundGeometry(t *testing.T) {
	c := &LegionContents{Contents: map[string]LegionContent{"Ispins": {LastPhase: 3, Dungeons: []LegionDungeon{{Dungeon: 100}, {Dungeon: 200}, {Dungeon: 300}, {Dungeon: 400}}}}}
	id, err := IspinsDefaultStageDungeon(c, 2)
	if err != nil || id != 300 {
		t.Fatal(id, err)
	}
	row := c.Contents["Ispins"]
	row.Dungeons[2].Dungeon = 999
	c.Contents["Ispins"] = row
	id, err = IspinsDefaultStageDungeon(c, 2)
	if err != nil || id != 999 {
		t.Fatal("stage ID still comes from capture", id, err)
	}
	for _, index := range []int{-1, 4} {
		if _, err = IspinsDefaultStageDungeon(c, index); err == nil {
			t.Fatal("invalid native ordinal accepted")
		}
	}
	if _, err = IspinsDefaultStageDungeon(nil, 0); err == nil {
		t.Fatal("missing source accepted")
	}
}
