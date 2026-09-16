// skillaudit projects source learning metadata without inventing level/cost rules.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path"
	"strings"
)

func main() {
	src := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only source")
	out := flag.String("output", "runtime/skill_learning_audit.json", "metadata audit")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *src, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	c, e := catalog.LoadCharacters("configs/characters.next25.json")
	if e != nil {
		log.Fatal(e)
	}
	type row struct {
		Job          byte
		ID           uint32
		Path, SHA256 string
		Fields       map[string][]pvf.Token
	}
	var result []row
	counts := map[string]int{}
	for job, p := range c.Professions {
		stem := strings.TrimSuffix(path.Base(p.Path), ".chr")
		idx, e := catalog.ResolveScript(a, "skill/"+stem+"skill.lst")
		if e != nil {
			idx, e = catalog.ResolveScript(a, "skill/"+stem+".lst")
		}
		if e != nil {
			log.Fatal(e)
		}
		refs, e := catalog.ParseIndex(idx.Cells)
		if e != nil {
			log.Fatal(e)
		}
		for _, ref := range refs {
			name := ref.Path
			if !strings.HasPrefix(name, "skill/") {
				name = "skill/" + name
			}
			s, e := catalog.ResolveScript(a, name)
			if e != nil {
				counts["unreadable"]++
				continue
			}
			fields := map[string][]pvf.Token{}
			tag := ""
			seen := map[string]bool{}
			enabled := false
			for _, t := range s.Cells {
				if t.Type == 3 {
					tag = t.Text
					enabled = !seen[tag]
					seen[tag] = true
					continue
				}
				if !enabled {
					continue
				}
				switch tag {
				case "[name]", "[type]", "[required level]", "[required level range]", "[maximum level]", "[growtype maximum level]", "[skill fitness growtype]", "[skill fitness second growtype]", "[pre required skill]", "[purchase cost]", "[special purchase cost]", "[feature skill type]", "[fixed level skill]", "[interval level]", "[add level per interval]", "[skill class]":
					fields[tag] = append(fields[tag], t)
				}
			}
			result = append(result, row{job, ref.ID, s.Path, s.SHA256, fields})
		}
	}
	b, e := json.MarshalIndent(map[string]any{"source": a.Snapshot(), "rows": result, "counts": counts}, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*out, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("skill metadata=%d counts=%v", len(result), counts)
}
