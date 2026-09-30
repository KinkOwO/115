package gamedata

import (
	"dfolan/internal/catalog"
	"os"
	"strings"
	"testing"
)

// Opt-in local proof uses the real archive without opening player storage.
func TestItemIndexLocalArchive(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for source parity")
	}
	s, err := Open(Options{Mode: PVF, ArchivePath: p, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256")})
	if err != nil {
		t.Fatal(err)
	}
	direct, err := catalog.ImportItemIndex(s.archive)
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := catalog.LoadItemIndex("../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}
	if legacy.Source.Checksum != direct.Source.Checksum {
		t.Fatal("source mismatch")
	}
	comparison := Compare(legacy, direct, 200000)
	counts := map[string]int{}
	first := map[string]Difference{}
	for _, d := range comparison.Differences {
		parts := strings.Split(d.Path, "/")
		field := parts[len(parts)-1]
		counts[field]++
		if _, ok := first[field]; !ok {
			first[field] = d
		}
	}
	t.Logf("items=%d differences=%d counts=%v first=%+v", len(direct.Items), comparison.Count, counts, first)
	if comparison.Count != 0 {
		t.Fatal("item index parity failed")
	}
}
