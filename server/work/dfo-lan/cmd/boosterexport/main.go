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
	out := flag.String("output", "", "required diagnostic output; not a runtime configuration")
	flag.Parse()
	if *out == "" {
		log.Fatal("explicit -output is required for diagnostic export")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1 << 30})
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	index, err := catalog.ImportItemIndex(a)
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
