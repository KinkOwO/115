package managementdata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestSourceFlagsFailClosed(t *testing.T) {
	for _, args := range [][]string{{"-catalog-source", "unknown"}, {"-catalog-source", "pvf"}, {"-catalog-source", "pvf", "-pvf-archive", "missing.pvf", "-pvf-source-checksum", "abcd"}} {
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		f := Register(fs)
		if err := fs.Parse(args); err != nil {
			t.Fatal(err)
		}
		if _, err := f.Open(); err == nil {
			t.Fatal("invalid source accepted", args)
		}
	}
	f := Register(flag.NewFlagSet("legacy", flag.ContinueOnError))
	if s, err := f.Open(); err != nil || s != nil {
		t.Fatal("legacy source changed", err)
	}
}

func TestPolicyRejectsUnknownAndTrailingFields(t *testing.T) {
	for _, body := range []string{`{"version":1,"source_data":42}`, `{"version":1} {}`, `{"version":1} garbage`} {
		p := filepath.Join(t.TempDir(), "policy.json")
		if err := os.WriteFile(p, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		var policy struct {
			Version int `json:"version"`
		}
		if err := ReadPolicy(p, &policy); err == nil {
			t.Fatal("invalid policy accepted", body)
		}
	}
}

// Full source-only preparation has no exported JSON inputs. Baselines are
// opened afterwards solely to audit typed fields and grant acceptance.
func TestNativeManagementCatalogParity(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE and DFO_PVF_CORE_TEST_SHA256")
	}
	s, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256")})
	if err != nil {
		t.Fatal(err)
	}
	base := "../../configs/"
	a, err := Awarder(s, base+"pvf-drop-policy.json", base+"inventory.next29.json")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Equipment.Full.Close()
	chars, err := Characters(s, base+"pvf-character-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	full := a.Equipment.Full
	// Historical JSON baselines are audit-only (server/AGENTS.md §0): the
	// exhaustive native-vs-export comparison runs only under
	// DFO_PVF_VERIFY_BASELINES=1. Normal runs keep the native assertions below.
	if os.Getenv("DFO_PVF_VERIFY_BASELINES") == "1" {
		oldChars, err := catalog.LoadCharacters(base + "characters.skycastle-release.json")
		if err != nil {
			t.Fatal(err)
		}
		if oldChars.Source.Checksum != chars.Source.Checksum {
			t.Fatal("character save source changed")
		}
		if diff := gamedata.Compare(oldChars, chars, 5); diff.Count != 0 {
			t.Fatalf("characters: %+v", diff)
		}
		oldLoot, err := catalog.LoadLoot(base + "loot.next25.json")
		if err != nil {
			t.Fatal(err)
		}
		if err := oldLoot.SupplementStackables(base + "items.index.json"); err != nil {
			t.Fatal(err)
		}
		if oldLoot.Source.Checksum != a.Catalog.Source.Checksum {
			t.Fatal("grant save source changed")
		}
		if diff := gamedata.Compare(oldLoot, a.Catalog, 5); diff.Count != 0 {
			t.Fatalf("loot/grants: %+v", diff)
		}
		oldGear, err := inventory.LoadEquipmentCatalog(base+"equipment.current37.json", a.Catalog.Source.Checksum)
		if err != nil {
			t.Fatal(err)
		}
		// Full is a native lazy provider with its own exhaustive definition audit;
		// compare the selected rows here without comparing provider internals.
		a.Equipment.Full = nil
		diff := gamedata.Compare(oldGear, a.Equipment, 5)
		a.Equipment.Full = full
		if diff.Count != 0 {
			t.Fatalf("equipment selection: %+v", diff)
		}
	}
	for _, id := range []uint32{10418036, 10418035} {
		if a.Catalog.Items[id].Kind != "stackable" {
			t.Fatal("non-drop currency unreachable", id)
		}
	}
	if _, err := a.Equipment.Reward(100050791); err != nil {
		t.Fatal("import-script equipment unreachable", err)
	}
	if _, err := a.Equipment.Reward(0); err == nil {
		t.Fatal("unknown equipment accepted")
	}
	t.Logf("source=%s professions=%d items=%d equipment_rows=%d full_bindings=%d; no storage access", chars.Source.Checksum, len(chars.Professions), len(a.Catalog.Items), len(a.Equipment.Rows), len(full.Records))
}
