package towergriefimport

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func Run() {
	source := flag.String("source", "../client-build/Script.inner.pvf", "current source PVF")
	output := flag.String("output", "", "required explicit source-matched tower overlay output")
	flag.Parse()
	if *output == "" {
		log.Fatal("-output is required; choose a test/export destination explicitly")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		log.Fatal(err)
	}
	overlay, err := catalog.ImportTowerGriefOverlay(a)
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.Marshal(overlay)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*output, b, 0644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d layers and %d maps to %s", len(overlay.Layers), len(overlay.Maps), *output)
}
