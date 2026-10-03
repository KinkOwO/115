// skillaudit projects source learning metadata without inventing level/cost rules.
package main

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/gamedata"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func learnableSkillFields(cells []pvf.Token) map[string][]pvf.Token {
	return character.LearnableSkillFields(cells)
}

func main() {
	src := flag.String("source", "../client-build/Script.inner.pvf", "read-only native PVF source")
	out := flag.String("output", "", "explicit diagnostic output path (required)")
	flag.String("characters", "", "deprecated: professions are read from the same PVF")
	flag.Parse()
	if *out == "" {
		log.Fatal("diagnostic output path is required")
	}
	source, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *src})
	if e != nil {
		log.Fatal(e)
	}
	defer source.Close()
	c, e := source.Characters("")
	if e != nil {
		log.Fatal(e)
	}
	direct, e := source.Learning(c)
	if e != nil {
		log.Fatal(e)
	}
	b, e := json.MarshalIndent(direct, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*out, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("skill metadata=%d", len(direct.Rows))
}
