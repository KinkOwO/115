package audit36

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
	"strings"
)

// reportFeatureSource surveys what the current archive actually carries for
// mail, avatars, the shop and item use. Data availability and protocol
// availability are separate problems: this answers only the first.
func reportFeatureSource(sourcePath string) {
	a, e := pvf.LoadArchive(pvf.Options{Path: sourcePath, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		fmt.Printf("\n== feature source scan unavailable: %v\n", e)
		return
	}
	fmt.Printf("\n== feature source survey\n")

	// Index files tell us which typed object lists exist at all.
	for _, name := range []string{"list/stackable.lst", "list/equipment.lst"} {
		s, err := catalog.ResolveScript(a, name)
		if err != nil {
			fmt.Printf("  %-28s MISSING (%v)\n", name, err)
			continue
		}
		rows, err := catalog.ParseIndex(s.Cells)
		if err != nil {
			fmt.Printf("  %-28s unparsed (%v)\n", name, err)
			continue
		}
		groups := map[string]int{}
		sample := map[string]string{}
		for _, r := range rows {
			p := strings.ToLower(r.Path)
			// Index paths are prefixed with their own list name; the category
			// that matters is the segment after it.
			parts := strings.Split(strings.TrimPrefix(strings.TrimPrefix(p, "stackable/"), "equipment/"), "/")
			seg := "(root)"
			if len(parts) > 1 {
				seg = parts[0]
			}
			groups[seg]++
			if sample[seg] == "" {
				sample[seg] = r.Path
			}
		}
		var keys []string
		for k := range groups {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return groups[keys[i]] > groups[keys[j]] })
		if len(keys) > 12 {
			keys = keys[:12]
		}
		fmt.Printf("  %-28s rows=%d\n", name, len(rows))
		for _, k := range keys {
			fmt.Printf("      %-22s %-7d %s\n", k, groups[k], sample[k])
		}
		// Locate the feature categories by keyword anywhere in the path, since
		// they are not top-level directories in this archive.
		fmt.Printf("    keyword hits in %s:\n", name)
		for _, kw := range []string{"avatar", "potion", "consumable", "cash", "shop", "mail",
			"recovery", "hpmp", "quest", "creature", "emblem", "title"} {
			n := 0
			first := ""
			for _, r := range rows {
				if strings.Contains(strings.ToLower(r.Path), kw) {
					n++
					if first == "" {
						first = r.Path
					}
				}
			}
			if n > 0 {
				fmt.Printf("      %-14s %-7d %s\n", kw, n, first)
			}
		}
	}
}

// reportItemUseFields reports which source fields a consumable carries, which
// is what an item-use implementation would have to read.
func reportItemUseFields(sourcePath string, samples int) {
	a, e := pvf.LoadArchive(pvf.Options{Path: sourcePath, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		fmt.Printf("\n== item use scan unavailable: %v\n", e)
		return
	}
	idx, e := catalog.ResolveScript(a, "list/stackable.lst")
	if e != nil {
		fmt.Printf("\n== stackable index missing: %v\n", e)
		return
	}
	rows, e := catalog.ParseIndex(idx.Cells)
	if e != nil {
		return
	}
	fieldCount := map[string]int{}
	usableTypes := map[string]int{}
	var shown int
	fmt.Printf("\n== consumable source fields\n")
	for _, r := range rows {
		p := strings.ToLower(r.Path)
		if !strings.HasPrefix(p, "consumable/") && !strings.Contains(p, "/consumable/") {
			continue
		}
		s, err := catalog.ResolveScript(a, r.Path)
		if err != nil {
			continue
		}
		var section string
		local := map[string][]pvf.Token{}
		for _, t := range s.Cells {
			if t.Type == 3 {
				section = t.Text
				fieldCount[section]++
				continue
			}
			if section != "" {
				local[section] = append(local[section], t)
			}
		}
		if v := local["[stackable type]"]; len(v) == 1 && v[0].Type == 6 {
			usableTypes[v[0].Text]++
		}
		if shown < samples {
			shown++
			fmt.Printf("  %s\n", r.Path)
			for _, key := range []string{"[name]", "[stackable type]", "[grade]", "[usable]",
				"[use type]", "[cooltime]", "[hp]", "[mp]", "[recovery]", "[physical]", "[attach type]"} {
				if v := local[key]; len(v) > 0 {
					fmt.Printf("      %-18s %s\n", key, brief(v))
				}
			}
		}
	}

	fmt.Printf("  consumable stackable types: ")
	var types []string
	for k := range usableTypes {
		types = append(types, k)
	}
	sort.Slice(types, func(i, j int) bool { return usableTypes[types[i]] > usableTypes[types[j]] })
	for i, k := range types {
		if i >= 14 {
			fmt.Printf("…")
			break
		}
		fmt.Printf("%s:%d ", k, usableTypes[k])
	}
	fmt.Println()
	fmt.Printf("  most common sections across consumables:\n")
	var keys []string
	for k := range fieldCount {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return fieldCount[keys[i]] > fieldCount[keys[j]] })
	for i, k := range keys {
		if i >= 22 {
			break
		}
		fmt.Printf("      %-30s %d\n", k, fieldCount[k])
	}
}
