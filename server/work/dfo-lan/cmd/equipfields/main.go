// equipfields prints every source cell of an equipment template, so a
// client-side equip refusal can be traced to a requirement the catalog import
// dropped (gender, body type, sub-job, and the like). Read-only.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func loadPaths(file string) (map[uint32]string, error) {
	b, e := os.ReadFile(file)
	if e != nil {
		return nil, e
	}
	var c struct {
		Rows []struct {
			ID   uint32
			Path string
		} `json:"rows"`
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return nil, e
	}
	m := map[uint32]string{}
	for _, r := range c.Rows {
		m[r.ID] = r.Path
	}
	return m, nil
}

func main() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only source")
	base := flag.String("base", "configs/quest-equipment.current37.json", "selection with the paths")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	c, e := loadPaths(*base)
	if e != nil {
		log.Fatal(e)
	}
	for _, s := range flag.Args() {
		var id uint32
		fmt.Sscan(s, &id)
		p, ok := c[id]
		if !ok {
			fmt.Printf("\n%d: not in selection\n", id)
			continue
		}
		sc, e := catalog.ResolveScript(a, p)
		if e != nil {
			fmt.Printf("\n%d: %v\n", id, e)
			continue
		}
		fmt.Printf("\n===== %d  %s\n", id, p)
		printCells(sc.Cells)
	}
}

func printCells(cells []pvf.Token) {
	tag := ""
	var vals []string
	flush := func() {
		if tag != "" {
			fmt.Printf("  %-28s %s\n", tag, strings.Join(vals, " "))
		}
	}
	for _, c := range cells {
		if c.Type == 3 {
			flush()
			tag, vals = c.Text, nil
			continue
		}
		if c.Text != "" {
			vals = append(vals, c.Text)
		} else {
			vals = append(vals, fmt.Sprintf("%d", c.Value))
		}
	}
	flush()
}
