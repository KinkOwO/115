package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"os"
	"reflect"
	"runtime"
	"sort"
	"testing"
)

func TestPVFEquipmentLocalArchive(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for full definition parity")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: p, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	if a.Snapshot().Checksum != os.Getenv("DFO_PVF_CORE_TEST_SHA256") {
		t.Fatal("source checksum mismatch")
	}
	// Item index parity has its own exhaustive proof. Reuse that verified source
	// binding here to avoid a second bulk stackable decode in this test.
	index, err := catalog.LoadItemIndex("../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}
	direct, err := OpenPVFEquipmentCatalog(a, index)
	if err != nil {
		t.Fatal(err)
	}
	defer direct.Close()
	a = nil
	runtime.GC()
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	t.Logf("compact equipment archive entries=%d retained_heap_bytes=%d", direct.archive.FileCount(), memory.HeapAlloc)
	legacy, err := OpenFullEquipmentCatalog("../../configs/equipment-full", direct.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	if legacy.RecordCount() != direct.RecordCount() || legacy.IndexSHA256 != direct.IndexSHA256 {
		t.Fatal("equipment index changed")
	}
	ids := make([]uint32, 0, len(legacy.Records))
	for id := range legacy.Records {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i, id := range ids {
		want, e := legacy.Definition(id)
		if e != nil {
			t.Fatal(id, e)
		}
		got, e := direct.Definition(id)
		if e != nil {
			t.Fatal(id, e)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("equipment %d definition differs", id)
		}
		if i%50000 == 49999 {
			t.Logf("verified %d/%d definitions", i+1, len(ids))
		}
	}
	if _, err := direct.Definition(0); err == nil {
		t.Fatal("unknown equipment ID accepted")
	}
	t.Logf("all %d equipment definitions matched", len(ids))
}
