// skillaudit projects source learning metadata without inventing level/cost rules.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func learnableSkillFields(cells []pvf.Token) map[string][]pvf.Token {
	return character.LearnableSkillFields(cells)
}

func main() {
	src := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only source")
	out := flag.String("output", "runtime/skill_learning_audit.json", "metadata audit")
	characterFile := flag.String("characters", "configs/characters.next25.json", "profession catalog")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *src, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	c, e := catalog.LoadCharacters(*characterFile)
	if e != nil {
		log.Fatal(e)
	}
	direct, e := character.ImportLearningCatalog(a, c)
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
