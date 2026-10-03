package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
)

func writePVFHeapProfile(path string, catalogs pvfCoreCatalogs) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	err = pprof.WriteHeapProfile(f)
	closeErr := f.Close()
	runtime.KeepAlive(catalogs)
	if err != nil {
		return err
	}
	return closeErr
}

// checkReport is called only after source preparation and before installing
// runtime globals, opening storage, creating capture files or listeners.
func (c pvfCoreCatalogs) checkReport(selection string) (map[string]any, error) {
	domains, err := parsePVFCatalogSelection(selection)
	if err != nil {
		return nil, err
	}
	checksum, err := hex.DecodeString(c.sourceChecksum)
	if err != nil || len(checksum) != 32 || len(domains) == 0 {
		return nil, fmt.Errorf("no prepared PVF source for catalog check")
	}
	ordered := make([]string, 0, len(domains))
	for _, domain := range strings.Split(pvfSupportedDomains, ",") {
		if domains[domain] {
			ordered = append(ordered, domain)
		}
	}
	r := map[string]any{"source": c.sourceChecksum, "domain_count": len(domains), "domains": ordered, "storage_accessed": false, "runtime_started": false}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	r["memory"] = map[string]any{"heap_alloc_bytes": memory.HeapAlloc, "heap_inuse_bytes": memory.HeapInuse, "heap_sys_bytes": memory.HeapSys, "total_alloc_bytes": memory.TotalAlloc, "gc_count": memory.NumGC}
	if c.characters != nil {
		r["professions"] = len(c.characters.Professions)
	}
	if c.quests != nil {
		r["quests"] = len(c.quests.Quests)
	}
	if c.items != nil {
		r["items"] = len(c.items.Items)
	}
	if c.equipment != nil {
		r["equipment_bindings"] = c.equipment.RecordCount()
	}
	if c.selection != nil {
		r["equipment_selection"] = len(c.selection.Rows)
	}
	if c.dungeons != nil {
		r["dungeons"] = len(c.dungeons.Dungeons)
	}
	if c.cashshop != nil {
		r["cashshop_products"] = c.cashshop.EnabledCount()
	}
	if c.boxes != nil {
		r["boxes"] = c.boxes.TableCount()
	}
	if c.itemShops != nil {
		r["item_shops"] = len(c.itemShops.Shops)
	}
	return r, nil
}

func logPVFMemory(stage string, elapsed time.Duration) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	log.Printf("PVF memory stage=%s elapsed=%s heap=%.1fMiB inuse=%.1fMiB reserved=%.1fMiB total_alloc=%.1fMiB gc=%d", stage, elapsed, float64(m.HeapAlloc)/(1<<20), float64(m.HeapInuse)/(1<<20), float64(m.HeapSys)/(1<<20), float64(m.TotalAlloc)/(1<<20), m.NumGC)
}
