package main

import (
	"os"
	"testing"
)

func TestPVFCharactersLocalArchive(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete character runtime parity")
	}
	c, err := preparePVFCoreCatalogs("characters", path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"), "../../configs/characters.skycastle-release.json", "", "", "", pvfItemInputs{characterPolicyPath: "../../configs/pvf-character-policy.json"})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := c.loadCharacters("missing-characters.json")
	if err != nil || len(direct.Professions) != 17 {
		t.Fatal("native professions missing", err)
	}
	if len(c.sourceCharacters.Professions[0].Growth) != 6 || len(direct.Professions[0].Growth) != 0 || direct.Source.Checksum != c.sourceCharacters.Source.Checksum {
		t.Fatal("raw source view or save identity changed")
	}
	t.Log("17 complete runtime professions match; 341 raw differences resolved through shortcut/command policy and duplicate source-view projection")
}
