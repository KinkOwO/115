package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

type SourceMapOverlay struct {
	SourceChecksum string                  `json:"source_checksum"`
	Maps           map[uint32]ScriptRecord `json:"maps"`
}

// ImportHellPartyMaps follows only the catalog's native seal-map references.
// Missing source maps remain unavailable, as in the existing export.
func ImportHellPartyMaps(a *pvf.Archive, c DungeonCatalog) (SourceMapOverlay, []uint32, error) {
	var out SourceMapOverlay
	if a == nil || a.Snapshot().Checksum != c.Source.Checksum {
		return out, nil, fmt.Errorf("Hell Party source mismatch")
	}
	out.SourceChecksum = c.Source.Checksum
	out.Maps = map[uint32]ScriptRecord{}
	index, err := ReadScript(a, "list/map.lst")
	if err != nil {
		return out, nil, err
	}
	rows, err := ParseIndex(index.Cells)
	if err != nil {
		return out, nil, err
	}
	paths := map[uint32]string{}
	for _, row := range rows {
		paths[row.ID] = row.Path
	}
	need := map[uint32]bool{}
	for _, d := range c.Dungeons {
		if d.HellParty != nil {
			need[d.HellParty.SealMap] = true
			if d.HellParty.SeasonSealMap != 0 {
				need[d.HellParty.SeasonSealMap] = true
			}
		}
	}
	var unavailable []uint32
	for id := range need {
		if _, exists := c.Maps[id]; exists {
			continue
		}
		if paths[id] == "" {
			unavailable = append(unavailable, id)
			continue
		}
		script, err := ResolveScript(a, paths[id])
		if err != nil {
			unavailable = append(unavailable, id)
			continue
		}
		out.Maps[id] = script
	}
	return out, unavailable, nil
}

// ApplyHellPartyMaps adds source map scripts referenced by DGN seal-map fields.
// The archive checksum stays unchanged so existing character and quest saves remain valid.
func ApplyHellPartyMaps(c *DungeonCatalog, overlay SourceMapOverlay) error {
	if c == nil {
		return fmt.Errorf("nil dungeon catalog")
	}
	if overlay.SourceChecksum != c.Source.Checksum || len(overlay.Maps) == 0 {
		return fmt.Errorf("Hell Party map overlay source mismatch or empty")
	}
	referenced := map[uint32]bool{}
	for _, d := range c.Dungeons {
		if d.HellParty == nil {
			continue
		}
		referenced[d.HellParty.SealMap] = true
		if d.HellParty.SeasonSealMap != 0 {
			referenced[d.HellParty.SeasonSealMap] = true
		}
	}
	for id, script := range overlay.Maps {
		if !referenced[id] || !strings.HasPrefix(script.Path, "map/") || len(script.SHA256) != 64 || len(script.Cells) == 0 {
			return fmt.Errorf("Hell Party map %d has invalid source identity", id)
		}
		if current, ok := c.Maps[id]; ok && current.SHA256 != script.SHA256 {
			return fmt.Errorf("Hell Party map %d conflicts with catalog", id)
		}
		c.Maps[id] = script
	}
	return nil
}
