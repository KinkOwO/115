package catalog

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// AttachHellPartyMaps adds map scripts referenced by DGN [seal door map index]
// and [season seal door map index]. The archive checksum stays unchanged so
// existing character and quest save versions remain valid.
func AttachHellPartyMaps(c *DungeonCatalog, path string) error {
	if c == nil {
		return fmt.Errorf("nil dungeon catalog")
	}
	var overlay struct {
		SourceChecksum string                  `json:"source_checksum"`
		Maps           map[uint32]ScriptRecord `json:"maps"`
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, &overlay); err != nil {
		return err
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
