// questequipmentimport widens the basic-equipment selection so every piece a
// quest can hand out is present in the catalog.
//
// Live capture 20260912T004122 refused quest 21650 twice with "equipment
// absent from source": its reward is template 100261068, which the 1536-row
// selection does not contain. An audit over the whole quest catalog found
// 1811 distinct reward templates missing, referenced 10843 times, so this is
// not one stray quest - it is most of the quest rewards in the build.
//
// The selection is widened from the source's own list/equipment.lst, never
// invented: an id that list does not carry is reported and skipped. Existing
// rows are preserved exactly, so previously verified entries keep their
// hashes. It never writes to the PVF.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path"
	"strings"
)

func main() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only source archive")
	base := flag.String("base", "configs/quest-equipment.next29.json", "existing basic item selection")
	flag.String("quests", "", "deprecated; quest rewards are read from native PVF")
	out := flag.String("output", "configs/quest-equipment.current37.json", "widened selection")
	flag.Parse()

	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	defer a.Close()
	c, e := inventory.LoadEquipmentCatalog(*base, a.Snapshot().Checksum)
	if e != nil {
		log.Fatal(e)
	}
	q, e := catalog.ImportQuests(a)
	if e != nil {
		log.Fatal(e)
	}
	index, e := catalog.ResolveScript(a, "list/equipment.lst")
	if e != nil {
		log.Fatal(e)
	}
	rows, e := catalog.ParseIndex(index.Cells)
	if e != nil {
		log.Fatal(e)
	}
	paths := map[uint32]string{}
	for _, r := range rows {
		paths[r.ID] = r.Path
	}
	log.Printf("source list carries %d equipment templates", len(paths))
	run(a, c, q, paths, *out)
}

func writeCatalog(c *inventory.EquipmentCatalog, out string) {
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(out, b, 0600); e != nil {
		log.Fatal(e)
	}
}

func resolve(a *pvf.Archive, p string) (catalog.ScriptRecord, error) {
	if !strings.HasPrefix(p, "equipment/") {
		p = path.Join("equipment", p)
	}
	return catalog.ResolveScript(a, p)
}
