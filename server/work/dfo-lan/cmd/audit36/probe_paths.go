package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
)

// dumpPaths prints every section of a handful of named source objects, so the
// fields a feature would have to read are established from this archive
// rather than assumed from another build.
func dumpPaths(sourcePath string, paths []string) {
	a, e := pvf.LoadArchive(pvf.Options{Path: sourcePath, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		fmt.Printf("\n== path dump unavailable: %v\n", e)
		return
	}
	fmt.Printf("\n== source object fields\n")
	for _, p := range paths {
		s, err := catalog.ResolveScript(a, p)
		if err != nil {
			fmt.Printf("  %s\n      UNREADABLE: %v\n", p, err)
			continue
		}
		fmt.Printf("  %s  (sha %s…)\n", p, s.SHA256[:12])
		order := []string{}
		fields := map[string][]pvf.Token{}
		var section string
		for _, t := range s.Cells {
			if t.Type == 3 {
				section = t.Text
				if _, seen := fields[section]; !seen {
					fields[section] = nil
					order = append(order, section)
				}
				continue
			}
			if section != "" {
				fields[section] = append(fields[section], t)
			}
		}
		sort.Strings(order)
		for _, k := range order {
			fmt.Printf("      %-30s %s\n", k, brief(fields[k]))
		}
	}
}
