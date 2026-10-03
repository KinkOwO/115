// selectionboximport exports a diagnostic snapshot through the native runtime projection.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func main() {
	source := flag.String("source", "../client-build/Script.inner.pvf", "read-only inner PVF")
	output := flag.String("output", "", "required diagnostic output; not a runtime configuration")
	flag.Parse()
	if *output == "" {
		log.Fatal("explicit -output is required for diagnostic export")
	}
	s, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *source, MaxBytes: 1 << 30})
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()
	index, err := s.ItemIndex("")
	if err != nil {
		log.Fatal(err)
	}
	boxes, err := s.SelectionBoxes(index, catalog.SelectionBoxPolicy{Version: 1})
	if err != nil {
		log.Fatal(err)
	}
	out, err := json.Marshal(boxes)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*output, out, 0600); err != nil {
		log.Fatal(err)
	}
	log.Printf("diagnostic native selection boxes: %d -> %s", len(boxes.Boxes), *output)
}
