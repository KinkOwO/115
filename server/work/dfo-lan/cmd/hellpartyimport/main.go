// hellpartyimport exports the source maps named by ordinary DGN Hell Party
// metadata without rewriting the large dungeon catalog or its save checksum.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
)

func main() {
	source := flag.String("source", "", "current Script.inner.pvf")
	base := flag.String("catalog", "configs/dungeons.full.json", "current dungeon catalog")
	output := flag.String("output", "configs/dungeons.hell-party-maps.json", "source-matched map overlay")
	flag.Parse()
	if *source == "" {
		panic("-source is required")
	}
	c, err := catalog.LoadDungeons(*base)
	if err != nil {
		panic(err)
	}
	need := map[uint32]bool{}
	for _, d := range c.Dungeons {
		if d.HellParty == nil {
			continue
		}
		need[d.HellParty.SealMap] = true
		if d.HellParty.SeasonSealMap != 0 {
			need[d.HellParty.SeasonSealMap] = true
		}
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		panic(err)
	}
	if a.Snapshot().Checksum != c.Source.Checksum {
		panic("PVF and dungeon catalog checksums differ")
	}
	index, err := catalog.ReadScript(a, "list/map.lst")
	if err != nil {
		panic(err)
	}
	rows, err := catalog.ParseIndex(index.Cells)
	if err != nil {
		panic(err)
	}
	paths := map[uint32]string{}
	for _, row := range rows {
		paths[row.ID] = row.Path
	}
	ids := make([]uint32, 0, len(need))
	for id := range need {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	maps := map[uint32]catalog.ScriptRecord{}
	unavailable := 0
	for _, id := range ids {
		if _, exists := c.Maps[id]; exists {
			continue
		}
		path := paths[id]
		if path == "" {
			fmt.Fprintf(os.Stderr, "Hell map %d absent from current list/map.lst; skipped\n", id)
			unavailable++
			continue
		}
		script, err := catalog.ResolveScript(a, path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Hell map %d unavailable: %v; skipped\n", id, err)
			unavailable++
			continue
		}
		maps[id] = script
	}
	data, err := json.Marshal(struct {
		SourceChecksum string                          `json:"source_checksum"`
		Maps           map[uint32]catalog.ScriptRecord `json:"maps"`
	}{c.Source.Checksum, maps})
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(*output, data, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Hell Party maps: %d source references, %d newly exported, %d unavailable; %s\n", len(ids), len(maps), unavailable, *output)
}
