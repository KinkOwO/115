package gamedata

import (
	"os"
	"testing"
)

func TestRetiredProgressionJSONCannotSupplyRuntimeContent(t *testing.T) {
	for _, c := range []*Catalogs{{}, {selected: map[string]bool{"progression": true}}} {
		if _, err := c.LoadProgression("../../configs/progression.next25.json"); err == nil {
			t.Fatal("retired progression JSON must not supply runtime growth")
		}
	}
}

func TestNativeProgressionPreparationNeedsNoJSONBaseline(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native progression preparation")
	}
	c, err := PrepareCatalogs(CatalogInputs{
		Selection: "progression", ArchivePath: path,
		ArchiveChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"),
		ProgressionPath: "missing-historical-progression.json", VerifyBaselines: true,
	}, CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Progression.Thresholds) == 0 || len(c.Progression.MonsterExperience) == 0 {
		t.Fatalf("incomplete native progression: thresholds=%d monster_exp=%d", len(c.Progression.Thresholds), len(c.Progression.MonsterExperience))
	}
	if _, err := c.LoadProgression("missing-retired-progression.json"); err != nil {
		t.Fatal(err)
	}
}
