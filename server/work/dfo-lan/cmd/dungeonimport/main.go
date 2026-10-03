package main

import (
	"dfolan/internal/gamedata"
	"encoding/json"
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	source := flag.String("source", "", "original PVF (read-only)")
	ids := flag.String("ids", "", "source dungeon IDs to import")
	dest := flag.String("output", "", "explicit diagnostic output catalog")
	flag.Parse()
	if *dest == "" {
		log.Fatal("-output is required; no runtime export is maintained")
	}
	var list []uint32
	for _, s := range strings.Split(*ids, ",") {
		v, e := strconv.ParseUint(s, 10, 32)
		if e != nil {
			log.Fatal(e)
		}
		list = append(list, uint32(v))
	}
	a, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *source})
	if e != nil {
		log.Fatal(e)
	}
	defer a.Close()
	c, e := a.Dungeons(list)
	if e != nil {
		log.Fatal(e)
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*dest, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("source=%s dungeons=%d maps=%d", c.Source.Checksum, len(c.Dungeons), len(c.Maps))
}
