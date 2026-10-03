package main

import (
	"dfolan/internal/gamedata"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func main() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only current PVF")
	output := flag.String("output", "", "required diagnostic output path")
	grade := flag.Uint("max-grade", 20, "maximum source item grade to import")
	flag.Parse()
	if *output == "" {
		log.Fatal("-output is required")
	}
	if *grade == 0 || *grade > 200 {
		log.Fatal("grade range1..200 required")
	}
	a, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *source, MaxBytes: gamedata.DefaultMaxBytes})
	if e != nil {
		log.Fatal(e)
	}
	defer a.Close()
	c, e := a.Loot(uint32(*grade))
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
