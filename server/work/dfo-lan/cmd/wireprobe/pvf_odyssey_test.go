package main

import (
	"dfolan/internal/game/protocol"
	"os"
	"testing"
)

func TestPVFOdysseyLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for Odyssey native parity")
	}
	c, err := preparePVFCoreCatalogs("odyssey-growth,odyssey-chapters,odyssey-weapons,odyssey-drop,odyssey-currency", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", contentPolicyPath: "../../configs/pvf-odyssey-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	g, err := c.loadOdysseyGrowth("missing-growth.json")
	if err != nil {
		t.Fatal(err)
	}
	ch, err := c.loadOdysseyChapters("missing-chapters.json")
	if err != nil {
		t.Fatal(err)
	}
	drop, err := c.loadOdysseyDrop("missing-drop.json")
	if err != nil {
		t.Fatal(err)
	}
	coins, err := c.loadOdysseyCurrency("missing-coins.json")
	if err != nil {
		t.Fatal(err)
	}
	w, err := c.loadOdysseyWeapons("missing-weapons.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(g.ClearLevels) != 50 || g.GraduateReward != 10420561 || ch.DungeonCount != 50 || len(w.Categories) != 85 {
		t.Fatal("Odyssey source scope changed")
	}
	awards, seed, err := drop.Roll(12345, 100004953)
	if err != nil || len(awards) != 0 || seed != 12345 {
		t.Fatal("chapter 2 policy changed", err)
	}
	if coins.Rates != [4]uint32{1000, 10000, 10000, 10000} || coins.Items[10418036].Weight != 0 {
		t.Fatal("coin policy/pool changed")
	}
	if !w.allows(protocol.WeaponBoxSelection{Category: [2]byte{0, 0}, Template: 101001229}) || w.allows(protocol.WeaponBoxSelection{Category: [2]byte{0, 0}, Template: 10418036}) {
		t.Fatal("weapon choice membership changed")
	}
	t.Log(len(g.Items), ch.ChapterCount, len(drop.Drops), len(coins.Items), len(w.Categories))
}
