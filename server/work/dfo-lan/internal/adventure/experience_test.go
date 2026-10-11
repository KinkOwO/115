package adventure

import (
	"math"
	"testing"
)

func TestAdventureExperienceSourceRateAndLegacyRemainder(t *testing.T) {
	r := &Rules{ExpRate: 0.3}
	var earned, remainder uint64
	for i := 0; i < 10; i++ {
		gain, next, scale, err := r.ExperienceGain(1, remainder, 0)
		if err != nil || scale != 10 {
			t.Fatal(gain, next, scale, err)
		}
		earned += gain
		remainder = next
	}
	if earned != 3 || remainder != 0 {
		t.Fatal("small kills lost experience", earned, remainder)
	}
	// An old save holds seven tenths; a new source rate of 0.15 must keep it.
	r.ExpRate = 0.15
	gain, rem, scale, err := r.ExperienceGain(2, 7, 0)
	if err != nil || gain != 1 || rem != 0 || scale != 20 {
		t.Fatal("source rate/old remainder ignored", gain, rem, scale, err)
	}
	r.ExpRate = 0.3
	gain, rem, scale, err = r.ExperienceGain(1, 95, 100)
	if err != nil || gain != 1 || rem != 25 || scale != 100 {
		t.Fatal("changed source discarded fractional experience", gain, rem, scale, err)
	}
	for _, rate := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))} {
		if _, _, _, err := (&Rules{ExpRate: rate}).ExperienceGain(1, 0, 0); err == nil {
			t.Fatal("invalid rate accepted", rate)
		}
	}
	if _, _, _, err := r.ExperienceGain(1, 10, 0); err == nil {
		t.Fatal("invalid legacy remainder accepted")
	}
	if _, _, _, err := (&Rules{ExpRate: 2}).ExperienceGain(math.MaxUint64, 0, 0); err == nil {
		t.Fatal("experience overflow accepted")
	}
}

func TestAdventureLevelLimitComesFromSource(t *testing.T) {
	r, err := EmbeddedRules()
	if err != nil {
		t.Fatal(err)
	}
	copy := *r
	copy.MaxLevel = 3
	copy.ExpRate = 0.5
	if _, err := NewRules(copy); err != nil {
		t.Fatal("source maximum/rate replaced by Go constants", err)
	}
	copy.MaxLevel = uint32(len(copy.Experience) + 1)
	if _, err := NewRules(copy); err == nil {
		t.Fatal("missing source experience accepted")
	}
}
