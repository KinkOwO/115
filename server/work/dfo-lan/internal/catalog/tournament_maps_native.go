package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

// ImportTournamentQuestMaps binds an indexed arena to its native DGN owner.
// The directory limits inspection; ownership and the unresolved quest maze
// decide whether a MAP is applicable. Duplicate owners are refused.
func ImportTournamentQuestMaps(a *pvf.Archive, c DungeonCatalog) (SourceMapOverlay, error) {
	var out SourceMapOverlay
	if a == nil || c.Source.Checksum == "" || a.Snapshot().Checksum != c.Source.Checksum {
		return out, fmt.Errorf("tournament PVF/dungeon source mismatch")
	}
	out.SourceChecksum = c.Source.Checksum
	out.Maps = map[uint32]ScriptRecord{}
	index, err := ReadScript(a, "list/map.lst")
	if err != nil {
		return out, err
	}
	rows, err := ParseIndex(index.Cells)
	if err != nil {
		return out, err
	}
	owners := map[uint32]uint32{}
	for _, row := range rows {
		p := strings.ToLower(strings.ReplaceAll(row.Path, "\\", "/"))
		if !strings.HasPrefix(p, "map/tournament/") {
			continue
		}
		script, err := ResolveScript(a, row.Path)
		if err != nil {
			return out, err
		}
		owner, kind := sectionCells(script.Cells, "[dungeon]"), sectionCells(script.Cells, "[type]")
		if len(owner) != 1 || owner[0].Type != 0 || owner[0].Value <= 0 || len(kind) != 1 || kind[0].Type != 6 || kind[0].Text != "[boss]" {
			continue
		}
		id := uint32(owner[0].Value)
		d, ok := c.Dungeons[id]
		if !ok || len(sectionCells(d.Script.Cells, "[tournament dungeon]")) != 1 {
			continue
		}
		matched := 0
		for _, m := range d.Mazes {
			if m.Quest != 0 && m.Size == [2]byte{1, 1} && m.Start == [2]byte{} && m.Boss == [2]byte{} && len(m.Rooms) == 0 && len(m.Pending) == 1 && m.Pending[0] == "unsupported room specification" {
				matched++
			}
		}
		if matched == 0 {
			continue
		}
		if matched != 1 {
			return out, fmt.Errorf("tournament owner %d has ambiguous quest mazes", id)
		}
		if previous, ok := owners[id]; ok {
			return out, fmt.Errorf("tournament owner %d has duplicate arenas %d/%d", id, previous, row.ID)
		}
		owners[id] = row.ID
		out.Maps[row.ID] = script
	}
	return out, nil
}
