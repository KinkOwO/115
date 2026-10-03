package gamedata

import (
	"os"
	"testing"
)

func TestRetiredLearningJSONCannotSupplyRuntimeContent(t *testing.T) {
	for _, c := range []*Catalogs{{}, {selected: map[string]bool{"skills": true}}} {
		if _, err := c.LoadLearning("../../configs/skills.release.json", "historical-source"); err == nil {
			t.Fatal("retired skills JSON must never supply runtime learning")
		}
	}
}

func TestNativeLearningPreparationNeedsNoJSONBaseline(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native learning preparation")
	}
	c, err := PrepareCatalogs(CatalogInputs{
		Selection: "skills", ArchivePath: path,
		ArchiveChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"),
		VerifyBaselines: true,
	}, CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Learning.Close()
	if len(c.Learning.Rows) != 3224 {
		t.Fatalf("incomplete native learning: %d", len(c.Learning.Rows))
	}
	l, err := c.LoadLearning("missing-retired-skills.json", c.SourceChecksum)
	if err != nil || l != c.Learning {
		t.Fatalf("prepared source was not reused: %v", err)
	}
	if _, err := c.LoadLearning("missing-retired-skills.json", "different-source"); err == nil {
		t.Fatal("learning source identity must stay strict")
	}
}
