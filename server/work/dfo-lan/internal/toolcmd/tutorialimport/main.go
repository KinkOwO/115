// Export exact-source starting routes and their dungeon maps, without
// changing player progress or granting any tutorial completion.
package tutorialimport

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
)

func Run() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only source")
	out := flag.String("output", "runtime/tutorial-source35", "isolated output directory")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	c, e := catalog.ImportTutorials(a)
	if e != nil {
		log.Fatal(e)
	}
	if e = os.MkdirAll(*out, 0700); e != nil {
		log.Fatal(e)
	}
	write := func(name string, value any) {
		p, e := json.MarshalIndent(value, "", "  ")
		if e != nil {
			log.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(*out, name), p, 0600); e != nil {
			log.Fatal(e)
		}
	}
	write("routes.json", c)
	var ids []uint32
	seen := map[uint32]bool{}
	for _, f := range c.Flows {
		if !f.EventOnly && !seen[f.Dungeon] {
			seen[f.Dungeon] = true
			ids = append(ids, f.Dungeon)
		}
	}
	d, e := catalog.ImportDungeons(a, ids)
	if e != nil {
		log.Fatal(e)
	}
	write("dungeons.json", d)
	log.Printf("tutorial source routes=%d ordinary_dungeons=%d maps=%d", len(c.Flows), len(d.Dungeons), len(d.Maps))
}
