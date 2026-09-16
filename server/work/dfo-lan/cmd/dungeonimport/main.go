package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
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
	dest := flag.String("output", "configs/dungeons.generated.json", "output catalog")
	flag.Parse()
	var list []uint32
	for _, s := range strings.Split(*ids, ",") {
		v, e := strconv.ParseUint(s, 10, 32)
		if e != nil {
			log.Fatal(e)
		}
		list = append(list, uint32(v))
	}
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	c, e := catalog.ImportDungeons(a, list)
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
