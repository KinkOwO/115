package main

import (
	"dfolan/internal/adventure"
	"os"
	"testing"
	"time"
)

func TestPVFSeasonLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete season source parity")
	}
	c, err := preparePVFCoreCatalogs("season", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json"})
	if err != nil {
		t.Fatal(err)
	}
	old, err := adventure.EmbeddedSeasonRules()
	if err != nil {
		t.Fatal(err)
	}
	restore, err := c.installSeasonRules()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	current, err := adventure.CurrentSeason()
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyPVFCatalog(c.seasonRules, current); err != nil {
		t.Fatal(err)
	}
	if len(current.Levels) != 120 || len(current.Contents) != 59 || len(current.Capsules) != 40 || current.OathCost.Gold != 0 || current.OathCostKey != 16 {
		t.Fatal("season scope changed")
	}
	checks := 0
	for _, level := range old.Levels {
		for _, experience := range []uint32{level.Upper - 1, level.Upper, level.Upper + 1} {
			state := adventure.SeasonState{Experience: experience}
			if current.Level(state) != old.Level(state) || current.DisplayLevel(state) != old.DisplayLevel(state) {
				t.Fatal("season level threshold changed", experience)
			}
			checks++
		}
	}
	for _, s := range old.SpecialReward[:2] {
		at, err := time.Parse("2006-01-02 15:04:05", s)
		if err != nil {
			t.Fatal(err)
		}
		for _, when := range []time.Time{at.Add(-time.Nanosecond), at, at.Add(time.Nanosecond)} {
			state := adventure.SeasonState{Experience: old.Levels[29].Upper}
			id, ok := current.SpecialRewardAt(state, when)
			wantID, wantOK := old.SpecialRewardAt(state, when)
			if id != wantID || ok != wantOK {
				t.Fatal("season event boundary changed", when)
			}
		}
	}
	t.Log("native levels", len(current.Levels), "contents", len(current.Contents), "capsules", len(current.Capsules), "level boundaries", checks, "COS hash", current.SHA256, "CTP hash", current.CostSHA256)
}
