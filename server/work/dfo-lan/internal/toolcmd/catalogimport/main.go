package catalogimport

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
)

func Run() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only inner archive")
	output := flag.String("output", "configs/characters.generated.json", "generated catalog")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	c, e := catalog.ImportCharacters(a)
	if e != nil {
		log.Fatal(e)
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.MkdirAll(filepath.Dir(*output), 0700); e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*output, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("imported professions=%d source=%s output=%s", len(c.Professions), c.Source.Checksum, *output)
}
