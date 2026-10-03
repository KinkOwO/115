package questcatalog

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func Run() {
	source := flag.String("source", "", "source PVF")
	out := flag.String("output", "", "catalog output")
	flag.Parse()
	if *out == "" {
		log.Fatal("explicit -output is required for a diagnostic export")
	}
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	defer a.Close()
	q, e := catalog.ImportQuests(a)
	if e != nil {
		log.Fatal(e)
	}
	b, e := json.MarshalIndent(q, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*out, b, 0600); e != nil {
		log.Fatal(e)
	}
	pending := 0
	for _, d := range q.Quests {
		if len(d.Pending) > 0 {
			pending++
		}
	}
	log.Printf("quests=%d unresolved=%d", len(q.Quests), pending)
}
