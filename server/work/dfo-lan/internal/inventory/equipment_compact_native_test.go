package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestCompactEquipmentLocalArchiveParity(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for compact equipment complete parity")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	script, err := catalog.ResolveScript(a, "list/equipment.lst")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := catalog.ParseIndex(script.Cells)
	if err != nil {
		t.Fatal(err)
	}
	idx := catalog.ItemIndex{Source: a.Snapshot(), IndexHashes: map[string]string{script.Path: script.SHA256}, Items: map[uint32]catalog.ItemIndexEntry{}}
	ids := make([]uint32, 0, len(rows))
	for _, r := range rows {
		path := r.Path
		if !strings.HasPrefix(path, "equipment/") {
			path = "equipment/" + path
		}
		idx.Items[r.ID] = catalog.ItemIndexEntry{ID: r.ID, Kind: "equipment", Path: path}
		ids = append(ids, r.ID)
	}
	c, err := OpenPVFEquipmentCatalog(a, idx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if c.RecordCount() != len(rows) || c.Records != nil {
		t.Fatal("compact count/storage differs")
	}
	if err = a.CompactRuntimeStrings(); err != nil {
		t.Fatal(err)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	begin := time.Now()
	for _, id := range ids {
		if !c.HasDefinition(id) {
			t.Fatal("missing compact binding", id)
		}
		s, err := catalog.ResolveScript(a, idx.Items[id].Path)
		if err != nil {
			t.Fatal(err)
		}
		want := equipmentDefinitionFromScript(id, s)
		got, err := c.Definition(id)
		if err != nil || !reflect.DeepEqual(want, got) {
			t.Fatalf("equipment %d differs: %v", id, err)
		}
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	c.cache.Clear()
	if _, err = c.Definition(ids[0]); err != nil {
		t.Fatal("closed parent invalidated equipment", err)
	}
	if n, count := c.cache.Usage(); n > 32*1024*1024 || count > 2048 {
		t.Fatal(n, count)
	}
	if c.HasDefinition(0) {
		t.Fatal("unknown equipment authorized")
	}
	if _, err = c.Definition(0); err == nil {
		t.Fatal("unknown equipment accepted")
	}
	t.Logf("source=%s complete equipment=%d compare=%s", idx.Source.Checksum, len(ids), time.Since(begin))
}
