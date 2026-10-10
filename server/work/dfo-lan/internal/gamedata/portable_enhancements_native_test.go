package gamedata

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"os"
	"testing"
)

func TestPortableEnhancementsCurrentSource(t *testing.T) {
	path := os.Getenv("DFO_PORTABLE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PORTABLE_TEST_ARCHIVE to audit current portable STK definitions")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 2 << 30}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	// Consume the complete native index through the shared importer, including
	// duplicate source aliases; this avoids inferring template IDs from names.
	s, err := Open(Options{Mode: PVF, ArchivePath: path})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	index, err := s.ItemIndex("")
	if err != nil {
		t.Fatal(err)
	}
	got, err := inventory.ImportEnhancements(a, index, "../../configs/pvf-enhancement-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	if a.Snapshot().Checksum == "83830f0693f1ceac104df5230782bf6527f2ca35580bfdf365eb94dcb2beb9ce" && len(got.Portable) != 367 {
		t.Fatalf("current portable source count: %d", len(got.Portable))
	}
	if len(got.Portable) == 0 {
		t.Fatal("portable source definitions were not imported")
	}
	t.Logf("source=%s portable=%d", a.Snapshot().Checksum, len(got.Portable))
}
