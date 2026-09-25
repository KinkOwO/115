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
	source := flag.String("source", "../client-build/Script.inner.pvf", "read-only source inner PVF")
	output := flag.String("output", "configs/item-period-tags.json", "output catalog")
	flag.Parse()
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		log.Fatal(err)
	}
	data, err := catalog.ImportItemPeriods(a)
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.Marshal(data)
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(*output, b, 0644); err != nil {
		log.Fatal(err)
	}
	log.Printf("item period templates=%d source=%s output=%s", len(data.Templates), data.Source.Checksum, *output)
}
