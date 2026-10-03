package main

import (
	"dfolan/internal/gamedata"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func main() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only current source")
	output := flag.String("output", "", "required explicit generated progression output")
	flag.Parse()
	if *output == "" {
		log.Fatal("-output is required; the retired configs default is not written")
	}
	a, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	defer a.Close()
	c, e := a.Progression("")
	if e != nil {
		log.Fatal(e)
	}
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*output, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("progression thresholds=%d SP=%d monster_exp=%d source=%s", len(c.Thresholds), len(c.SkillPoints), len(c.MonsterExperience), c.Source.Checksum)
}
