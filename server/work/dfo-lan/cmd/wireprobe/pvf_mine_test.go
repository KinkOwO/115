package main

import (
	"os"
	"testing"
)

func TestPVFMineLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for mine reward graph parity")
	}
	c, err := preparePVFCoreCatalogs("bleeding-mine", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", contentPolicyPath: "../../configs/pvf-mine-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := c.loadMine("missing-mine.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(r.StageBoxes) != 12 || len(r.BossBoxes) != 12 || len(r.GroupBoxes) != 3 || len(r.Boxes) != 117 || len(r.Items) != 1782 || r.Combine.Maximum != 5 || len(r.NoDrop) != 1 || r.NoDrop[0] != 10330673 {
		t.Fatal("mine source scope changed")
	}
	empty := 0
	for _, pools := range r.Boxes {
		for _, pool := range pools {
			for _, v := range pool.Candidates {
				if v.Template < 0 {
					empty++
				}
			}
		}
	}
	if empty == 0 {
		t.Fatal("signed empty faces disappeared")
	}
	t.Log("signed empty faces", empty, "combine chances", r.Combine.Chances)
}
