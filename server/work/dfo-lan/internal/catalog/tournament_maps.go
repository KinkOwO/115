package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Tournament quest DGN files have a maze and a quest connection, but no
// ordinary [map specification]. Their arena MAP declares the dungeon owner.
// Keep this source-matched overlay separate from the large persisted catalog:
// changing its archive checksum would invalidate existing quest save records.
func AttachTournamentQuestMaps(c *DungeonCatalog, path string) error {
	if c == nil {
		return fmt.Errorf("nil dungeon catalog")
	}
	var overlay SourceMapOverlay
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &overlay); err != nil {
		return err
	}
	return ApplyTournamentQuestMaps(c, overlay)
}

func ApplyTournamentQuestMaps(c *DungeonCatalog, overlay SourceMapOverlay) error {
	if c == nil {
		return fmt.Errorf("nil dungeon catalog")
	}
	if overlay.SourceChecksum != c.Source.Checksum || len(overlay.Maps) == 0 {
		return fmt.Errorf("tournament map overlay source mismatch or empty")
	}
	for mapID, script := range overlay.Maps {
		owner := sectionCells(script.Cells, "[dungeon]")
		kind := sectionCells(script.Cells, "[type]")
		if len(owner) != 1 || owner[0].Type != 0 || owner[0].Value <= 0 ||
			len(kind) != 1 || kind[0].Type != 6 || kind[0].Text != "[boss]" ||
			!strings.HasPrefix(script.Path, "map/tournament/") || len(script.SHA256) != 64 {
			return fmt.Errorf("tournament map %d has invalid source identity", mapID)
		}
		dungeonID := uint32(owner[0].Value)
		d, ok := c.Dungeons[dungeonID]
		if !ok || len(sectionCells(d.Script.Cells, "[tournament dungeon]")) != 1 {
			return fmt.Errorf("tournament map %d has no matching tournament dungeon", mapID)
		}
		if existing, ok := c.Maps[mapID]; ok && existing.SHA256 != script.SHA256 {
			return fmt.Errorf("tournament map %d conflicts with catalog", mapID)
		}
		matched := 0
		for i := range d.Mazes {
			m := &d.Mazes[i]
			if m.Quest == 0 || m.Size != [2]byte{1, 1} || m.Start != [2]byte{} || m.Boss != [2]byte{} ||
				len(m.Rooms) != 0 || len(m.Pending) != 1 || m.Pending[0] != "unsupported room specification" {
				continue
			}
			m.Rooms = []DungeonRoom{{X: 0, Y: 0, Map: mapID, Boss: true}}
			m.Pending = nil
			matched++
		}
		if matched != 1 {
			return fmt.Errorf("tournament map %d did not resolve exactly one quest maze", mapID)
		}
		c.Maps[mapID] = script
		c.Dungeons[dungeonID] = d
	}
	return nil
}
