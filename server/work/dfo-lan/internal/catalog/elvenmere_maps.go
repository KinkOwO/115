package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// The dungeon maze names only zone one's waiting map. The other waiting maps
// are dependencies of elvenmere.cos, so bind them even after a cache hit.
func AttachElvenmereMaps(c *DungeonCatalog, a *pvf.Archive) error {
	if c.Elvenmere == nil {
		return nil
	}
	index, err := ReadScript(a, "list/map.lst")
	if err != nil {
		return err
	}
	rows, err := ParseIndex(index.Cells)
	if err != nil {
		return err
	}
	paths := map[uint32]string{}
	for _, r := range rows {
		paths[r.ID] = r.Path
	}
	for _, area := range c.Elvenmere.Areas {
		if _, ok := c.Maps[area.WaitingMap]; ok {
			continue
		}
		path, ok := paths[area.WaitingMap]
		if !ok {
			return fmt.Errorf("elvenmere waiting map %d has no source binding", area.WaitingMap)
		}
		script, err := ResolveScript(a, path)
		if err != nil {
			return err
		}
		c.Maps[area.WaitingMap] = script
	}
	return nil
}
