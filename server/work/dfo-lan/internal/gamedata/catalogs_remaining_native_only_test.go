package gamedata

import (
	"dfolan/internal/catalog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A legacy artifact must never become a content source, including when the
// caller did not select a PVF domain. Check the source error, not a file error.
func TestRemainingRuntimeContentRefusesHistoricalJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "historical.json")
	if err := os.WriteFile(path, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, state := range []string{"unselected", "selected-unprepared"} {
		t.Run(state, func(t *testing.T) {
			c := &Catalogs{}
			if state == "selected-unprepared" {
				c.selected = map[string]bool{}
				for _, domain := range strings.Split(SupportedDomains, ",") {
					c.selected[domain] = true
				}
			}
			checks := map[string]func() error{
				"characters":        func() error { _, e := c.LoadCharacters(path); return e },
				"random-options":    func() error { _, e := c.LoadRandomOptions(path, "old"); return e },
				"shields":           func() error { _, e := c.LoadShields(path, "old"); return e },
				"oath-grades":       func() error { _, e := c.LoadOathGrades(path); return e },
				"vault":             func() error { _, e := c.LoadVaultRules(path); return e },
				"journal":           func() error { _, e := c.LoadEquipmentJournal(path, "old"); return e },
				"create-cost":       func() error { _, e := c.LoadEquipmentCreateCost(path, "old"); return e },
				"tutorial":          func() error { _, e := c.LoadTutorialRoutes(path, "old"); return e },
				"boxes":             func() error { _, e := c.LoadBoxes(path, "old"); return e },
				"item-shops":        func() error { _, e := c.LoadItemShops(path, "old"); return e },
				"town":              func() error { _, e := c.LoadTown(path); return e },
				"training-dungeons": func() error { _, e := c.LoadTrainingDungeons(path); return e },
				"tutorial-dungeons": func() error { _, e := c.LoadTutorialDungeons(path); return e },
				"black-purgatory":   func() error { _, e := c.LoadBlackPurgatory(path, nil, nil); return e },
				"clear-cube":        func() error { _, e := c.WithClearCube(catalog.LootCatalog{}, path); return e },
				"bleeding-mine":     func() error { _, e := c.LoadMine(path); return e },
				"odyssey-growth":    func() error { _, e := c.LoadOdysseyGrowth(path); return e },
				"odyssey-chapters":  func() error { _, e := c.LoadOdysseyChapters(path); return e },
				"odyssey-weapons":   func() error { _, e := c.LoadOdysseyWeapons(path); return e },
				"odyssey-drop":      func() error { _, e := c.LoadOdysseyDrop(path); return e },
				"odyssey-currency":  func() error { _, e := c.LoadOdysseyCurrency(path); return e },
				"attunement":        func() error { _, e := c.LoadAttunement(path); return e },
				"apocalypse":        func() error { _, e := c.LoadApocalypse(path); return e },
				"dungeon-terminal":  func() error { return c.AttachTerminalScenes(&catalog.DungeonCatalog{}, path) },
				"dungeon-maze":      func() error { return c.AttachMazeRates(&catalog.DungeonCatalog{}, path) },
				"layer-revisits":    func() error { return c.AttachLayerRevisits(&catalog.DungeonCatalog{}, path) },
			}
			for domain, check := range checks {
				t.Run(domain, func(t *testing.T) {
					if err := check(); err == nil || !strings.Contains(err.Error(), "PVF") {
						t.Fatalf("historical source accepted or read: %v", err)
					}
				})
			}
		})
	}
	for _, check := range []func() error{
		func() error { _, e := (&Source{mode: JSON}).Characters(path); return e },
		func() error { _, e := (&Source{mode: JSON}).Progression(path); return e },
	} {
		if err := check(); err == nil || !strings.Contains(err.Error(), "native PVF") {
			t.Fatal(err)
		}
	}
}

func TestNativeRulesIgnoreMissingCharacterBaseline(t *testing.T) {
	archive := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for real PVF rules")
	}
	c, err := PrepareCatalogs(CatalogInputs{
		Selection: "random-options", ArchivePath: archive,
		ArchiveChecksum:  os.Getenv("DFO_PVF_CORE_TEST_SHA256"),
		CharacterPath:    filepath.Join(t.TempDir(), "missing-characters.json"),
		RandomOptionPath: "missing-options.json", DerivedCacheDir: testPVFCacheDir(),
	}, CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.LoadRandomOptions("missing-options.json", c.SourceChecksum); err != nil {
		t.Fatal(err)
	}
	if _, err := c.LoadRandomOptions("missing-options.json", "foreign-source"); err == nil {
		t.Fatal("foreign source accepted")
	}
}
