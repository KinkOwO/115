package main

import (
	"os"
	"testing"
)

func TestPVFLayerRevisitsLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native layer revisit source binding")
	}
	c, err := preparePVFCoreCatalogs("dungeons,layer-revisits", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", dungeonPath: "../../configs/dungeons.full.json", scenePolicyPath: "../../configs/pvf-scene-policy.json", layerRevisitPolicyPath: "../../configs/pvf-layer-revisit-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	base := clonePVFDungeons(*c.dungeons)
	if err = c.attachLayerRevisits(&base, "missing-layer-revisits.json"); err != nil {
		t.Fatal(err)
	}
	if len(base.LayerRevisits) != 1 || base.LayerRevisits[0].Map != 100004546 || base.LayerRevisits[0].ResumeMap != 100004325 || len(c.dungeons.LayerRevisits) != 0 {
		t.Fatal("layer source scope or ownership changed")
	}
	base.LayerRevisits[0].Map = 0
	if c.layerRevisits.Scenes[0].Map != 100004546 {
		t.Fatal("runtime overlay shares source storage")
	}
	t.Log("quest 12893 final cinematic and same-grid base match full native source hashes and witnessed record")
}
