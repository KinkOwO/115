// Export source Odyssey reward items and source script paths read-only.
package odysseyaudit

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func Run() {
	a, e := pvf.LoadArchive(pvf.Options{Path: "../client-build/Script.inner.pvf", MaxBytes: 1024 * 1024 * 1024})
	must(e)
	dest := "docs/evidence/odyssey-rewards-revive-20260917"
	must(os.MkdirAll(dest, 0755))
	var paths []map[string]any
	wanted := map[int32]bool{}
	for _, kind := range []string{"stackable", "equipment"} {
		s, e := catalog.ReadScript(a, "list/"+kind+".lst")
		must(e)
		rows, e := catalog.ParseIndex(s.Cells)
		must(e)
		for _, r := range rows {
			if r.ID == 10417791 || r.ID == 10417789 || r.ID == 10417790 || r.ID == 10418028 || strings.Contains(strings.ToLower(r.Path), "aradodyssey") || strings.Contains(strings.ToLower(r.Path), "arad_odyssey") {
				paths = append(paths, map[string]any{"id": r.ID, "kind": kind, "path": r.Path})
				if r.ID == 10417791 {
					s, e := catalog.ResolveScript(a, r.Path)
					must(e)
					write(filepath.Join(dest, "create-reward.json"), s)
				}
				if r.ID != 10417791 {
					s, e := catalog.ResolveScript(a, r.Path)
					must(e)
					write(filepath.Join(dest, fmt.Sprintf("item-%d.json", r.ID)), s)
					if r.ID == 10417789 || r.ID == 10417790 {
						inside := false
						for _, c := range s.Cells {
							if c.Text == "[equipment]" {
								inside = true
							}
							if c.Text == "[/equipment]" {
								inside = false
							}
							if inside && c.Type == 0 && c.Value > 100000000 {
								wanted[c.Value] = true
							}
						}
					}
				}
			}
			if kind == "equipment" && wanted[int32(r.ID)] {
				s, e := catalog.ResolveScript(a, r.Path)
				must(e)
				write(filepath.Join(dest, fmt.Sprintf("equipment-%d.json", r.ID)), s)
			}
		}
	}
	write(filepath.Join(dest, "odyssey-items.json"), paths)
	fmt.Printf("AUDIT PASS source=%s matching_items=%d\n", a.Snapshot().Checksum, len(paths))
}
func write(p string, v any) {
	b, e := json.MarshalIndent(v, "", "  ")
	must(e)
	must(os.WriteFile(p, b, 0644))
}
func must(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
