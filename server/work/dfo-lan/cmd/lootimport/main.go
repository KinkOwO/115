package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func main() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only current PVF")
	output := flag.String("output", "configs/loot.next25.json", "generated typed loot catalog")
	grade := flag.Uint("max-grade", 20, "maximum source item grade to import")
	flag.Parse()
	if *grade == 0 || *grade > 200 {
		log.Fatal("grade range1..200 required")
	}
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	c, e := catalog.ImportLoot(a, uint32(*grade))
	if e != nil {
		log.Fatal(e)
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*output, b, 0600); e != nil {
		log.Fatal(e)
	}
	counts := map[string]int{}
	for _, i := range c.Items {
		counts[i.Kind]++
	}
	log.Printf("loot source=%s max_grade=%d candidates=%v unreadable_skipped=%d", c.Source.Checksum, c.MaximumGrade, counts, len(c.Skipped))
}
