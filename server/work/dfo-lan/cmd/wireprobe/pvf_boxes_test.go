package main

import (
	"os"
	"testing"
)

func TestPVFBoxesSourceOnlyImportsItsOwnItemDependency(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for independent box source preparation")
	}
	verify := false
	c, err := preparePVFCoreCatalogs("characters,boxes", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "missing-characters.json", "", "", "", pvfItemInputs{verifyBaselines: &verify, characterPolicyPath: "../../configs/pvf-character-policy.json", boxPolicyPath: "../../configs/pvf-box-policy.json", indexPath: "missing-items.json", boxesPath: "missing-boxes.json"})
	if err != nil {
		t.Fatal(err)
	}
	if c.items == nil || c.boxes == nil || c.boxes.TableCount() != 2 {
		t.Fatal("implicit native item dependency missing")
	}
	if _, err := c.loadBoxes("missing-boxes.json", c.items.Source.Checksum); err != nil {
		t.Fatal(err)
	}
}

func TestPVFBoxesLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native COS material binding parity")
	}
	c, err := preparePVFCoreCatalogs("items,boxes", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{indexPath: "../../configs/items.index.json", boxesPath: "../../configs/boxes.json", boxPolicyPath: "../../configs/pvf-box-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.loadBoxes("missing-boxes.json", c.items.Source.Checksum)
	if err != nil || b.TableCount() != 2 || b.RewardCount() != 54 {
		t.Fatal("native boxes unavailable", err)
	}
	if b.Tables["590712474"].PointStacks[1].SectionReward[0].Template != 590722560 || b.Tables["590719043"].PointStacks[1].SectionReward[0].Template != 590719045 {
		t.Fatal("same-name COS material owners confused")
	}
	if _, err := c.loadBoxes("missing", "foreign"); err == nil {
		t.Fatal("foreign save identity accepted")
	}
	t.Logf("2 exact native material owners, 54 rewards, %d raw hashes; COS hashes %s / %s", len(b.Sources), b.Sources["live/else/univ/2024/0514_radianttreasurebox/radianttreasurebox.cos"], b.Sources["live/else/univ/2025/0318_newrandombox/radianttreasurebox.cos"])
}
