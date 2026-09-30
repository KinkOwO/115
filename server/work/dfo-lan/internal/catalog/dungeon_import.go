package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
)

// ImportFullDungeons selects the archive's dungeon list and the world's source
// gate references. Exclusions keep separately loaded training rooms separate.
func ImportFullDungeons(a *pvf.Archive, world WorldCatalog, excluded []uint32) (DungeonCatalog, error) {
	if a == nil || world.Source.Checksum != a.Snapshot().Checksum {
		return DungeonCatalog{}, fmt.Errorf("full dungeon/world source mismatch")
	}
	script, err := ReadScript(a, "list/dungeon.lst")
	if err != nil {
		return DungeonCatalog{}, err
	}
	rows, err := ParseIndex(script.Cells)
	if err != nil {
		return DungeonCatalog{}, err
	}
	ids := map[uint32]bool{}
	for _, row := range rows {
		ids[row.ID] = true
	}
	for _, gate := range world.Dungeons {
		ids[gate.ID] = true
	}
	for _, id := range excluded {
		if !ids[id] {
			return DungeonCatalog{}, fmt.Errorf("excluded dungeon %d absent from source", id)
		}
		delete(ids, id)
	}
	ordered := make([]uint32, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	c, err := ImportDungeons(a, ordered)
	if err != nil {
		return c, err
	}
	return ValidateDungeons(c)
}
