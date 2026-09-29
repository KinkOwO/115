package catalog

import (
	"encoding/json"
	"fmt"
	"os"
)

// AttachTerminalScenes loads PVF-derived terminal scene evidence separately
// from the large dungeon export, so a source update cannot silently reuse it.
func AttachTerminalScenes(c *DungeonCatalog, file string) error {
	if c == nil {
		return fmt.Errorf("terminal scenes require a dungeon catalog")
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var overlay struct {
		Source struct {
			Checksum string `json:"checksum"`
		} `json:"source"`
		Scenes []DungeonTerminalScene `json:"terminal_scenes"`
	}
	if err := json.Unmarshal(data, &overlay); err != nil {
		return err
	}
	if overlay.Source.Checksum == "" || overlay.Source.Checksum != c.Source.Checksum || len(overlay.Scenes) == 0 {
		return fmt.Errorf("terminal scene source mismatch or empty export")
	}
	seen := map[[3]uint32]bool{}
	for _, scene := range overlay.Scenes {
		key := [3]uint32{scene.Dungeon, uint32(scene.Maze), scene.FinalMap}
		if seen[key] || scene.Source != c.Source.Checksum || scene.XMin == 0 || scene.YMin == 0 ||
			scene.XMax < scene.XMin || scene.YMax < scene.YMin || scene.XMax == 255 || scene.YMax == 255 ||
			scene.ActionSHA256 == "" || scene.CinematicSHA256 == "" || scene.CinematicPath == "" {
			return fmt.Errorf("invalid terminal scene %v", key)
		}
		seen[key] = true
		def, ok := c.Dungeons[scene.Dungeon]
		if !ok || def.Script.SHA256 != scene.DungeonSHA256 || c.Maps[scene.FinalMap].SHA256 != scene.MapSHA256 {
			return fmt.Errorf("terminal scene source map mismatch %v", key)
		}
		matched := false
		for _, maze := range def.Mazes {
			if maze.Index != scene.Maze || maze.Quest != scene.Quest || scene.Quest == 0 {
				continue
			}
			for _, layer := range maze.Layers {
				if layer.Position != scene.Position || len(layer.Maps) == 0 || layer.Maps[len(layer.Maps)-1] != scene.FinalMap {
					continue
				}
				for _, room := range maze.Rooms {
					if room.Boss && room.Map == scene.ObjectiveMap && [2]byte{room.X, room.Y} == scene.Position {
						matched = true
					}
				}
			}
		}
		if !matched {
			return fmt.Errorf("terminal scene differs from quest boss layer %v", key)
		}
	}
	c.TerminalScenes = overlay.Scenes
	return nil
}
