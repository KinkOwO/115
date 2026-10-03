// equipfields prints every source cell of an equipment template, so a
// client-side equip refusal can be traced to a requirement the catalog import
// dropped (gender, body type, sub-job, and the like). Read-only.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
)

func equipmentPaths(source *gamedata.Source) (map[uint32]string, error) {
	list, err := source.Script("list/equipment.lst")
	if err != nil {
		return nil, err
	}
	return parseEquipmentPaths(list.Cells)
}

func parseEquipmentPaths(cells []pvf.Token) (map[uint32]string, error) {
	rows, err := catalog.ParseIndex(cells)
	if err != nil {
		return nil, err
	}
	paths := make(map[uint32]string, len(rows))
	for _, row := range rows {
		paths[row.ID] = row.Path
	}
	return paths, nil
}

func main() {
	sourcePath := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only source archive")
	flag.Parse()

	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *sourcePath, MaxBytes: gamedata.DefaultMaxBytes})
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()

	paths, err := equipmentPaths(source)
	if err != nil {
		log.Fatal(err)
	}
	for _, arg := range flag.Args() {
		parsedID, err := strconv.ParseUint(arg, 10, 32)
		if err != nil || parsedID == 0 {
			fmt.Printf("\n%s: invalid equipment ID\n", arg)
			continue
		}
		id := uint32(parsedID)
		path, ok := paths[id]
		if !ok {
			fmt.Printf("\n%d: not in PVF equipment list\n", id)
			continue
		}
		script, err := source.ResolveScript(path)
		if err != nil {
			fmt.Printf("\n%d: %v\n", id, err)
			continue
		}
		fmt.Printf("\n===== %d  %s\n", id, script.Path)
		printCells(script.Cells)
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
	for _, cell := range cells {
		if cell.Type == 3 {
			flush()
			tag, vals = cell.Text, nil
			continue
		}
		if cell.Text != "" {
			vals = append(vals, cell.Text)
		} else {
			vals = append(vals, fmt.Sprintf("%d", cell.Value))
		}
	}
	flush()
}
