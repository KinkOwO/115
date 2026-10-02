package inventory

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"os"
	"path"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// Verify every remapped binding against its original native LIST path rather
// than the historical exported catalog (which belongs to a different SHA).
func TestEquipmentProjectionLocalArchiveParity(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for complete native equipment projection parity")
	}
	a, err := catalog.OpenTestArchiveCached(p, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
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
	index := catalog.ItemIndex{Source: a.Snapshot(), IndexHashes: map[string]string{script.Path: script.SHA256}, Items: map[uint32]catalog.ItemIndexEntry{}}
	for _, r := range rows {
		nativePath := r.Path
		if !strings.HasPrefix(nativePath, "equipment/") {
			nativePath = path.Join("equipment", nativePath)
		}
		index.Items[r.ID] = catalog.ItemIndexEntry{ID: r.ID, Kind: "equipment", Path: nativePath}
	}
	projection, err := ProjectPVFEquipment(a, index)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	var restored PVFEquipmentProjection
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	c, err := RestorePVFEquipment(a, restored)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for i, b := range restored.Bindings {
		expected, err := catalog.ReadScript(a, catalog.ResolveScriptPath(a, index.Items[b.ID].Path))
		if err != nil {
			t.Fatal(b.ID, err)
		}
		got, err := c.Definition(b.ID)
		if err != nil || !reflect.DeepEqual(got, equipmentDefinitionFromScript(b.ID, expected)) {
			t.Fatal("remapped definition differs", b.ID, err)
		}
		if i%50000 == 49999 {
			t.Logf("verified %d/%d native definitions", i+1, len(restored.Bindings))
		}
	}
	if err = a.CompactRuntimeStrings(); err != nil {
		t.Fatal(err)
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	c.cache.Clear()
	if _, err = c.Definition(restored.Bindings[len(restored.Bindings)/2].ID); err != nil {
		t.Fatal("lost source after parent close", err)
	}
	t.Logf("all %d remapped equipment definitions matched native LIST scripts", len(restored.Bindings))
}

func TestPVFEquipmentLocalArchive(t *testing.T) {
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for full definition parity")
	}
	a, err := catalog.OpenTestArchiveCached(p, "")
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
