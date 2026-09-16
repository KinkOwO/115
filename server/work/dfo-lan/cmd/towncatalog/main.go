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
	source := flag.String("source", "", "read-only source PVF")
	town := flag.Uint("town", 0, "town ID from list/town.lst")
	area := flag.Uint("area", 0, "area ID from the town script")
	out := flag.String("output", "configs/town.generated.json", "catalog output")
	flag.Parse()
	if *town > 65535 || *area > 65535 {
		log.Fatal("invalid town/area")
	}
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	c, e := catalog.ImportTownArea(a, uint32(*town), uint32(*area))
	if e != nil {
		log.Fatal(e)
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*out, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("town=%d area=%d map=%s walkable=%d source=%s", c.TownID, c.AreaID, c.MapPath, len(c.Walkable), c.Source.Checksum)
}
