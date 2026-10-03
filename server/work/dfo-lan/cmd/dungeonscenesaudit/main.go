// Export all currently configured Odyssey maps and their original scene scripts.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	archive := flag.String("archive", "../client-build/Script.inner.pvf", "read-only inner PVF")
	output := flag.String("output", "", "explicit directory for diagnostic evidence")
	flag.Parse()
	if *output == "" {
		log.Fatal("-output is required; no runtime export is maintained")
	}
	dest := *output
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *archive})
	must(err)
	defer source.Close()
	world, err := source.World("")
	must(err)
	full, err := source.FullDungeons(world, nil)
	must(err)
	var ids []uint32
	for id, d := range full.Dungeons {
		if d.Odyssey {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	c, err := source.Dungeons(ids)
	must(err)
	must(os.MkdirAll(dest, 0755))
	write(filepath.Join(dest, "dungeons.json"), c)
	scripts := map[string]catalog.ScriptRecord{}
	var lists []string
	for _, f := range source.Files() {
		p := strings.ToLower(f.ArchivePath)
		if strings.HasPrefix(p, "list/") && (strings.Contains(p, "cinematic") || strings.Contains(p, "aicharacter") || p == "list/monster.lst") {
			lists = append(lists, p)
		}
		if !strings.HasPrefix(p, "contents/2026/aradodyssey/") || !(strings.HasSuffix(p, ".act") || strings.HasSuffix(p, ".cmt") || strings.HasSuffix(p, ".aic") || strings.HasSuffix(p, ".mob")) {
			continue
		}
		s, e := source.Script(p)
		must(e)
		scripts[p] = s
	}
	for _, p := range lists {
		s, e := source.Script(p)
		must(e)
		scripts[p] = s
	}
	// Follow map action and APC references outside the Odyssey source folder.
	ai, err := catalog.ParseIndex(scripts["list/aicharacter.lst"].Cells)
	must(err)
	aiPaths := map[uint32]string{}
	for _, row := range ai {
		aiPaths[row.ID] = row.Path
	}
	cinematic, err := catalog.ParseIndex(scripts["list/cinematic.lst"].Cells)
	must(err)
	cinematicPaths := map[uint32]string{}
	for _, row := range cinematic {
		cinematicPaths[row.ID] = row.Path
	}
	missing := map[string]string{}
	add := func(p string) catalog.ScriptRecord {
		s, e := source.ResolveScript(p)
		if e != nil {
			missing[p] = e.Error()
			return s
		}
		scripts[s.Path] = s
		return s
	}
	for _, m := range c.Maps {
		for i, t := range m.Cells {
			if t.Type != 3 || i+1 >= len(m.Cells) {
				continue
			}
			if t.Text == "[basic action]" && m.Cells[i+1].Type == 6 {
				action := add(path.Join(path.Dir(m.Path), m.Cells[i+1].Text))
				for j, t := range action.Cells {
					if t.Type == 3 && t.Text == "[CINEMATIC]" && j+1 < len(action.Cells) && action.Cells[j+1].Type == 0 {
						add(cinematicPaths[uint32(action.Cells[j+1].Value)])
					}
				}
			}
			if t.Text == "[ai character]" {
				for j := i + 1; j < len(m.Cells) && m.Cells[j].Type != 3; j++ {
					if m.Cells[j].Type == 0 {
						if p, ok := aiPaths[uint32(m.Cells[j].Value)]; ok {
							add(p)
						}
					}
				}
			}
		}
	}
	write(filepath.Join(dest, "missing-source-references.json"), missing)
	write(filepath.Join(dest, "scripts.json"), scripts)
	write(filepath.Join(dest, "indices.json"), lists)
	fmt.Printf("EXPORT PASS: dungeons=%d maps=%d scripts=%d source=%s\n", len(c.Dungeons), len(c.Maps), len(scripts), c.Source.Checksum)
}
func must(e error) {
	if e != nil {
		log.Fatal(e)
	}
}
func write(p string, v any) {
	b, e := json.MarshalIndent(v, "", "  ")
	must(e)
	must(os.WriteFile(p, b, 0644))
}
