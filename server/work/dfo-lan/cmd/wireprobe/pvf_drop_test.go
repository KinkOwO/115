package main

import (
	"os"
	"testing"
)

func TestPVFDropLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for full loot and equipment selection parity")
	}
	c, err := preparePVFCoreCatalogs("loot,equipment-selection", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", dropPolicyPath: "../../configs/pvf-drop-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.loot.Items) != 1022 || len(c.selection.Rows) != 3174 {
		t.Fatal("existing runtime pool changed", len(c.loot.Items), len(c.selection.Rows))
	}
	if _, err := c.loadLoot("missing.json"); err != nil {
		t.Fatal(err)
	}
	a, err := c.loadEquipmentSelection("missing.json", c.items.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.loadEquipmentSelection("missing.json", c.items.Source.Checksum)
	if err != nil || a == b {
		t.Fatal("shared mutable wrapper", err)
	}
	t.Logf("loot and equipment full effective parity; drop pool=%d", len(a.DropPool()))
}

func TestPVFEquipmentSelectionLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip()
	}
	c, err := preparePVFCoreCatalogs("equipment-selection", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", dropPolicyPath: "../../configs/pvf-drop-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(len(c.selection.Rows), len(c.selection.DropPool()))
}
