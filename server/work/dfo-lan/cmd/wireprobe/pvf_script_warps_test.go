package main

import (
	"os"
	"testing"
)

func TestPVFScriptWarpsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for full native script warp bindings")
	}
	c, err := preparePVFCoreCatalogs("dungeons,script-warps", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", dungeonPath: "../../configs/dungeons.full.json", scenePolicyPath: "../../configs/pvf-scene-policy.json", scriptWarpPolicyPath: "../../configs/pvf-script-warp-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.scriptWarps) != 12 {
		t.Fatal("script warp scope changed")
	}
	restore, err := c.installScriptWarps()
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	t.Log("all 11 cinematic and 1 forced monster routes match full source hashes, action ownership, grid and landing fields")
}
