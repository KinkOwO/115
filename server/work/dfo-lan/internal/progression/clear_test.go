package progression

import (
	"dfolan/internal/catalog"
	"testing"
)

func TestSourceClearBaseAndReferenceRankFormula(t *testing.T) {
	c, e := catalog.LoadProgression("../../configs/progression.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	d, e := catalog.LoadDungeons("../../configs/dungeons.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	got, e := DungeonClear(c, d.Dungeons[3], 0, 50)
	// Current level3 base126, decimal rate1.3, rank50 crosses four thresholds.
	// The explicit reference formula floors base to163 and its15% score to24.
	if e != nil || got.Base != 163 || got.Score != 24 || got.Grade != 50 {
		t.Fatal(got, e)
	}
	c.DifficultyRates[0] = 2
	got, e = DungeonClear(c, d.Dungeons[3], 0, 50)
	if e != nil || got.Base != 252 || got.Score != 37 {
		t.Fatal("rate did not follow source", got, e)
	}
	if _, e = DungeonClear(c, d.Dungeons[3], 255, 50); e == nil {
		t.Fatal("unknown difficulty accepted")
	}
}
