package gamedata

import (
	"dfolan/internal/catalog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDerivedItemCacheLocalArchiveParity(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete derived item cache parity")
	}
	dir := t.TempDir()
	o := catalog.ItemBasicOptions{Periods: true, Prices: true, Materials: true, Skins: true, Boosters: true}
	const policy = "../../configs/pvf-enhancement-policy.json"
	open := func(cache string) *Source {
		s, err := Open(Options{Mode: PVF, ArchivePath: p, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256"), DerivedCacheDir: cache})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		return s
	}
	importAll := func(s *Source) JointItemCatalogs {
		out, err := s.ItemCatalogs(o, true, policy, true)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	first := open(dir)
	cold := importAll(first)
	if first.DerivedCacheStats().Writes != 1 {
		t.Fatal("cold cache not stored", first.DerivedCacheStats())
	}
	first.Close()
	compare := func(cached JointItemCatalogs) {
		original := cold
		// Only per-open diagnostics are rebound; all effective fields and
		// native hashes/order must remain identical.
		original.Basics.Index.Source = cached.Basics.Index.Source
		periods := *cold.Basics.Periods
		periods.Source = cached.Basics.Periods.Source
		original.Basics.Periods = &periods
		skins := *cold.Basics.Skins
		skins.Source = cached.Basics.Skins.Source
		original.Basics.Skins = &skins
		if c := Compare(original, cached, 3); c.Count != 0 {
			t.Fatal("cached item fields differ", c)
		}
		for _, r := range cold.Basics.Materials.Items {
			a, ok1 := cold.Basics.Materials.Materials(r.Template)
			b, ok2 := cached.Basics.Materials.Materials(r.Template)
			if ok1 != ok2 || !reflect.DeepEqual(a, b) {
				t.Fatal("private material lookup differs", r.Template)
			}
		}
	}
	second := open(dir)
	warm := importAll(second)
	if second.DerivedCacheStats().Hits != 1 {
		t.Fatal("warm cache not used", second.DerivedCacheStats())
	}
	if warm.Basics.Index.Source.LoadedAt != second.Snapshot().LoadedAt || warm.Basics.Index.Source.Checksum != second.Snapshot().Checksum {
		t.Fatal("per-open source not rebound")
	}
	compare(warm)
	second.Close()
	if _, err := second.ItemCatalogs(o, true, policy, true); err == nil {
		t.Fatal("cache bypassed closed native source")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.pvfc"))
	if err != nil || len(files) != 1 {
		t.Fatal(files, err)
	}
	f, err := os.OpenFile(files[0], os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.WriteAt([]byte{0}, derivedHeaderSize); err != nil {
		t.Fatal(err)
	}
	f.Close()
	third := open(dir)
	rebuilt := importAll(third)
	if stats := third.DerivedCacheStats(); stats.Invalid != 1 || stats.Writes != 1 || stats.Hits != 0 {
		t.Fatal("damaged cache not rebuilt", stats)
	}
	compare(rebuilt)
	third.Close()
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, []byte("unrelated file"), 0600); err != nil {
		t.Fatal(err)
	}
	fourth := open(blocked)
	fallback := importAll(fourth)
	if fourth.DerivedCacheStats().WriteErrors != 1 {
		t.Fatal("write failure not exercised")
	}
	compare(fallback)
	fourth.Close()
	b, err := os.ReadFile(blocked)
	if err != nil || string(b) != "unrelated file" {
		t.Fatal("unrelated file modified")
	}
	if bad, err := Open(Options{Mode: PVF, ArchivePath: p, ExpectedChecksum: strings.Repeat("0", 64), DerivedCacheDir: dir}); err == nil {
		bad.Close()
		t.Fatal("cache bypassed source SHA validation")
	}
	t.Logf("complete cold/hit/corrupt rebuild/write failure parity: items=%d prices=%d boosters=%d source=%s", len(cold.Basics.Index.Items), len(cold.Basics.Prices.Items), len(cold.Basics.Boosters), cold.Basics.Index.Source.Checksum)
}
