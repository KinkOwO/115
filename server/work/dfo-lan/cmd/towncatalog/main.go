package main

import (
	"dfolan/internal/gamedata"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func main() {
	source := flag.String("source", "", "read-only source PVF")
	town := flag.Uint("town", 0, "town ID from list/town.lst")
	area := flag.Uint("area", 0, "area ID from the town script")
	out := flag.String("output", "", "explicit diagnostic output path (required)")
	flag.Parse()
	if *out == "" {
		log.Fatal("explicit -output is required")
	}
	if *town > 65535 || *area > 65535 {
		log.Fatal("invalid town/area")
	}
	a, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *source})
	if e != nil {
		log.Fatal(e)
	}
	defer a.Close()
	c, e := a.Town(uint32(*town), uint32(*area))
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
