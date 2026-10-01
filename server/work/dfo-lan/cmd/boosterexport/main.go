// boosterexport and the runtime share one source reward projection.
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
	source := flag.String("source", "../client-build/Script.inner.pvf", "read-only inner PVF")
	indexFile := flag.String("index", "configs/items.index.json", "same-source item index")
	out := flag.String("output", "configs/booster-catalog.json", "reward catalog")
	flag.Parse()
	index, err := catalog.LoadItemIndex(*indexFile)
	if err != nil {
		log.Fatal(err)
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1 << 30})
	if err != nil {
		log.Fatal(err)
	}
	c, err := catalog.ImportBoosters(a, index)
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err = os.WriteFile(*out, b, 0600); err != nil {
		log.Fatal(err)
	}
	log.Printf("exported %d booster definitions", len(c))
}
