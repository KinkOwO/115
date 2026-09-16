// equipmentwearimport enriches the already selected basic equipment catalog
// with exact-source wear eligibility fields; it never changes the PVF.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func main() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "source archive")
	base := flag.String("base", "configs/quest-equipment.next29.json", "existing basic item selection")
	out := flag.String("output", "configs/equipment.current35.json", "enriched catalog")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	c, e := inventory.LoadEquipmentCatalog(*base, a.Snapshot().Checksum)
	if e != nil {
		log.Fatal(e)
	}
	wanted := map[string]bool{"[usable job]": true, "[usable grow type]": true, "[minimum level]": true, "[equipment type]": true, "[attach type]": true, "[rarity]": true, "[durability]": true, "[name]": true, "[grade]": true}
	for i := range c.Rows {
		r := &c.Rows[i]
		s, e := catalog.ResolveScript(a, r.Path)
		if e != nil {
			log.Fatal(e)
		}
		if s.SHA256 != r.SHA256 {
			log.Fatalf("source equipment changed: %d", r.ID)
		}
		fields := map[string][]pvf.Token{}
		tag := ""
		for _, cell := range s.Cells {
			if cell.Type == 3 {
				tag = cell.Text
				continue
			}
			if wanted[tag] {
				fields[tag] = append(fields[tag], cell)
			}
		}
		r.Fields = fields
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*out, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("equipment eligibility exported: %d source records", len(c.Rows))
}
