package gamedata

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProjectionCacheFallbackAndRestore(t *testing.T) {
	s := &Source{archive: &pvf.Archive{}, cacheDir: t.TempDir()}
	type row struct {
		Price   *uint32
		Unknown *uint32
		Value   int
	}
	zero := uint32(0)
	loads, restores := 0, 0
	load := func() (row, error) { loads++; return row{Price: &zero, Value: 7}, nil }
	restore := func(r row) (row, error) {
		restores++
		if r.Value != 7 {
			return r, fmt.Errorf("invalid projection")
		}
		return r, nil
	}
	call := func() row {
		t.Helper()
		r, e := cachedProjection(s, "test", 1, load, restore)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	if r := call(); r.Price == nil || *r.Price != 0 || r.Unknown != nil {
		t.Fatal("changed nil/zero price")
	}
	if r := call(); r.Price == nil || r.Unknown != nil || loads != 1 || restores != 1 {
		t.Fatal("cache not restored", loads, restores)
	}
	files, _ := filepath.Glob(filepath.Join(s.cacheDir, "*.pvfc"))
	b, _ := os.ReadFile(files[0])
	b[len(b)-1] ^= 1
	if e := os.WriteFile(files[0], b, 0600); e != nil {
		t.Fatal(e)
	}
	call()
	if loads != 2 || s.DerivedCacheStats().Invalid != 1 {
		t.Fatal("corrupt cache was not rebuilt")
	}
	_, e := cachedProjection(s, "test", 1, load, func(r row) (row, error) { return r, fmt.Errorf("private index restore failure") })
	if e != nil || loads != 3 {
		t.Fatal("restore failure did not fall back", e, loads)
	}
	blocked := filepath.Join(t.TempDir(), "foreign")
	os.WriteFile(blocked, []byte("preserve"), 0600)
	s.cacheDir = blocked
	call()
	after, _ := os.ReadFile(blocked)
	if string(after) != "preserve" || s.DerivedCacheStats().WriteErrors == 0 {
		t.Fatal("cache write changed foreign file")
	}
	s.cacheDir = "-"
	before := restores
	call()
	if restores != before {
		t.Fatal("disabled cache read")
	}
}

func TestProjectionDependenciesRemainPhysical(t *testing.T) {
	base, _ := projectionKey("parser", "pvf-one", "domain", map[string]int{"policy": 1})
	for _, args := range [][3]string{{"other", "pvf-one", "domain"}, {"parser", "pvf-two", "domain"}, {"parser", "pvf-one", "other"}} {
		k, _ := projectionKey(args[0], args[1], args[2], map[string]int{"policy": 1})
		if k == base {
			t.Fatal("unbound dependency")
		}
	}
	k, _ := projectionKey("parser", "pvf-one", "domain", map[string]int{"policy": 2})
	if k == base {
		t.Fatal("policy not bound")
	}
	a, b := pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}, pvf.ArchiveSnapshot{Checksum: strings.Repeat("b", 64)}
	if a.SaveIdentity() != b.SaveIdentity() {
		t.Fatal("save contract changed with resource")
	}
	x := catalog.ItemIndex{Source: a, Items: map[uint32]catalog.ItemIndexEntry{3: {ID: 3, Path: "a", Kind: "stackable"}}}
	original := itemIndexIdentity(x)
	for _, field := range []string{"path", "kind", "limit", "type", "id", "source", "index"} {
		c := x
		c.Items = map[uint32]catalog.ItemIndexEntry{3: x.Items[3]}
		r := c.Items[3]
		switch field {
		case "path":
			r.Path = "b"
		case "kind":
			r.Kind = "equipment"
		case "limit":
			r.StackLimit = 1
		case "type":
			r.StackableType = "material"
		case "id":
			r.ID = 4
		case "source":
			c.Source = b
		case "index":
			c.IndexHashes = map[string]string{"list": "other"}
		}
		c.Items[3] = r
		if itemIndexIdentity(c) == original {
			t.Fatal("unbound input", field)
		}
	}
	x.Source.CachedChunks = 99
	if itemIndexIdentity(x) != original {
		t.Fatal("diagnostic counters invalidated cache")
	}
	w := catalog.WorldCatalog{Source: a}
	other := w
	other.Source.CachedTexts = 2
	if !reflect.DeepEqual(stableWorldInput(w), stableWorldInput(other)) {
		t.Fatal("unstable world inputs")
	}
}
