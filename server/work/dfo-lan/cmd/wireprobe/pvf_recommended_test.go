package main

import (
	"dfolan/internal/adventure"
	"os"
	"testing"
)

func TestPVFRecommendedLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete recommended eligibility parity")
	}
	c, err := preparePVFCoreCatalogs("adventure-recommended", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{})
	if err != nil {
		t.Fatal(err)
	}
	old, err := adventure.EmbeddedRecommendedRules()
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.installRecommendedRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := adventure.CurrentRecommendedRules()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPVFCatalog(c.recommendedRules, current); err != nil {
		t.Fatal(err)
	}
	excluded := map[uint32]bool{}
	ids := map[uint32]bool{0: true, 1: true, 999999999: true}
	for id := range old.Ranges {
		ids[id] = true
	}
	for _, id := range old.Excluded {
		ids[id] = true
		excluded[id] = true
	}
	for _, id := range old.AmbiguousDungeons {
		ids[id] = true
	}
	checks := 0
	for id := range ids {
		for level := 0; level <= 255; level++ {
			bounds, found := old.Ranges[id]
			want := uint32(level) >= old.MinimumLevel && !excluded[id] && found && uint32(level) >= bounds[0] && uint32(level) <= bounds[1]
			got, err := adventure.RecommendedDungeonClear(id, byte(level))
			if err != nil || got != want {
				t.Fatalf("dungeon=%d level=%d got=%v want=%v error=%v", id, level, got, want, err)
			}
			checks++
		}
	}
	t.Log("ranges", len(current.Ranges), "excluded", len(current.Excluded), "ambiguous", len(current.AmbiguousDungeons), "unavailable worldmaps", len(current.UnavailableWorldmaps), "sources", len(current.Sources), "eligibility queries", checks)
}
