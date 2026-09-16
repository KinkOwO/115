// equipmentaudit exports current source fields without assigning drop semantics.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
	"strings"
)

func main() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only source")
	out := flag.String("output", "runtime/equipment_audit.json", "audit output")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	idx, e := catalog.ResolveScript(a, "list/equipment.lst")
	if e != nil {
		log.Fatal(e)
	}
	rows, e := catalog.ParseIndex(idx.Cells)
	if e != nil {
		log.Fatal(e)
	}
	type row struct {
		ID     uint32
		Path   string
		Fields map[string][]pvf.Token
		SHA256 string
	}
	var selected []row
	counts := map[string]int{}
	for _, r := range rows {
		p := r.Path
		if !strings.HasPrefix(p, "equipment/") {
			p = "equipment/" + p
		}
		if strings.Contains(p, "/avatar/") {
			continue
		}
		s, e := catalog.ResolveScript(a, p)
		if e != nil {
			counts["unreadable"]++
			continue
		}
		fields := map[string][]pvf.Token{}
		name := ""
		for _, t := range s.Cells {
			if t.Type == 3 {
				name = t.Text
				continue
			}
			switch name {
			case "[name]", "[grade]", "[rarity]", "[creation rate]", "[minimum level]", "[equipment type]", "[durability]", "[attach type]":
				fields[name] = append(fields[name], t)
			}
		}
		counts["read"]++
		rate := fields["[creation rate]"]
		if len(rate) > 0 {
			counts["has_creation_rate"]++
		}
		grade := fields["[grade]"]
		level := fields["[minimum level]"]
		low := len(grade) > 0 && grade[0].Type == 0 && grade[0].Value <= 20 && grade[0].Value > 0 || len(level) > 0 && level[0].Type == 0 && level[0].Value <= 20 && level[0].Value > 0
		if low {
			selected = append(selected, row{r.ID, s.Path, fields, s.SHA256})
		}
	}
	data := map[string]any{"source": a.Snapshot(), "index_hash": idx.SHA256, "counts": counts, "rows": selected}
	b, e := json.MarshalIndent(data, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*out, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("equipment audited=%v low_grade_or_level=%d", counts, len(selected))
}
