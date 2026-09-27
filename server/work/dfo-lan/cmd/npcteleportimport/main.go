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
	source := flag.String("source", "../client-build/Script.inner.pvf", "native inner PVF")
	output := flag.String("output", "configs/npc-teleport.generated.json", "output catalog")
	flag.Parse()
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		log.Fatal(err)
	}
	c, err := catalog.ImportNPCMoves(a)
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*output, append(b, '\n'), 0644); err != nil {
		log.Fatal(err)
	}
}
