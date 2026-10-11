package gamedata

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestTruthCurrentPVFDirectSource(t *testing.T) {
	filename := os.Getenv("DFO_TRUTH_TEST_ARCHIVE")
	if filename == "" {
		t.Skip("set DFO_TRUTH_TEST_ARCHIVE for current archive audit")
	}
	s, err := Open(Options{Mode: PVF, ArchivePath: filename})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	script, err := catalog.ResolveScript(s.archive, "list/stackable.lst")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := catalog.ParseIndex(script.Cells)
	if err != nil {
		t.Fatal(err)
	}
	index := catalog.ItemIndex{Source: s.Snapshot(), Items: map[uint32]catalog.ItemIndexEntry{}}
	for _, row := range entries {
		path := row.Path
		if !strings.HasPrefix(path, "stackable/") {
			path = "stackable/" + path
		}
		index.Items[row.ID] = catalog.ItemIndexEntry{ID: row.ID, Kind: "stackable", Path: path}
	}
	rules, err := s.OathTruth(index)
	if err != nil {
		t.Fatal(err)
	}
	if rules.RawSHA256 != "997a8714a984ecf1869f0305bb8680ed12336204060e38a44a553a1228f77986" || rules.MaxLevel != 4 || len(rules.Alternatives[10421371]) != 1 || rules.Alternatives[10421371][0] != 10421372 {
		t.Fatal("current source contract", rules)
	}
	encoded, _ := json.Marshal(rules)
	t.Log(string(encoded))
}

// The transform domain is prepared before the item domain. Exercise the public
// startup facade so the truth/material dependency cannot regress to that order.
func TestTruthStartupPreparesImplicitMaterialIndex(t *testing.T) {
	filename := os.Getenv("DFO_TRUTH_TEST_ARCHIVE")
	if filename == "" {
		t.Skip("set DFO_TRUTH_TEST_ARCHIVE for startup preparation audit")
	}
	c, err := PrepareCatalogs(CatalogInputs{Selection: "transform", ArchivePath: filename, DerivedCacheDir: "-"}, CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	if c.Items == nil || c.Truth == nil || c.Truth.MaxLevel != 4 || c.Truth.Source != c.Items.Source.Checksum || !c.Prepared("transform") {
		t.Fatal("truth startup dependency missing", c)
	}
}
