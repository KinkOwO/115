package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func main() {
	source := flag.String("source", "", "read-only source PVF")
	out := flag.String("output", "configs/world.generated.json", "source catalog output")
	base := flag.String("base", "", "preserve an existing catalog and refresh its phase NPC rows and source graphs")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	w, e := catalog.ImportWorld(a)
	if e != nil {
		log.Fatal(e)
	}
	lineEnding := []byte("\n")
	if *base != "" {
		original, readErr := os.ReadFile(*base)
		if readErr != nil {
			log.Fatal(readErr)
		}
		var existing catalog.WorldCatalog
		if decodeErr := json.Unmarshal(original, &existing); decodeErr != nil {
			log.Fatal(decodeErr)
		}
		if existing.Source.Checksum != w.Source.Checksum || len(existing.Areas) != len(w.Areas) {
			log.Fatal("base catalog does not match the source archive")
		}
		if bytes.Contains(original, []byte("\r\n")) {
			lineEnding = []byte("\r\n")
		}
		for key, area := range existing.Areas {
			fromSource, found := w.Areas[key]
			if !found {
				log.Fatalf("base area %s is absent from the source archive", key)
			}
			area.PhaseNPCs = fromSource.PhaseNPCs
			area.PhaseMaps = fromSource.PhaseMaps
			existing.Areas[key] = area
		}
		w = existing
	}
	b, e := json.MarshalIndent(w, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if len(lineEnding) == 2 {
		b = bytes.ReplaceAll(b, []byte("\n"), lineEnding)
	}
	if e = os.WriteFile(*out, b, 0600); e != nil {
		log.Fatal(e)
	}
	pending := 0
	for _, area := range w.Areas {
		if len(area.Pending) > 0 {
			pending++
		}
	}
	log.Printf("towns=%d areas=%d dungeons=%d areas_with_unresolved_data=%d", len(w.Towns), len(w.Areas), len(w.Dungeons), pending)
}
