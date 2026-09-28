package catalog

import (
	"encoding/json"
	"fmt"
	"os"
)

// DungeonLayerRevisit describes a source CMT ending in a cached room in the
// same grid cell. ResumeMap selects its base combat room when the last layer
// contains only an isolated cinematic. It never implies dungeon completion.
type DungeonLayerRevisit struct {
	Source          string   `json:"source"`
	Dungeon         uint32   `json:"dungeon"`
	Maze            byte     `json:"maze"`
	Quest           uint16   `json:"quest"`
	Position        [2]byte  `json:"position"`
	Map             uint32   `json:"map"`
	ResumeMap       uint32   `json:"resume_map,omitempty"`
	ResumeMapSHA256 string   `json:"resume_map_sha256,omitempty"`
	Record          [18]byte `json:"record"`
	DungeonSHA256   string   `json:"dungeon_sha256"`
	MapSHA256       string   `json:"map_sha256"`
	ActionSHA256    string   `json:"action_sha256"`
	CinematicSHA256 string   `json:"cinematic_sha256"`
	CinematicPath   string   `json:"cinematic_path"`
}

func AttachLayerRevisits(c *DungeonCatalog, file string) error {
	if c == nil {
		return fmt.Errorf("layer revisits require a dungeon catalog")
	}
	b, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var overlay struct {
		Source struct {
			Checksum string `json:"checksum"`
		} `json:"source"`
		Scenes []DungeonLayerRevisit `json:"layer_revisits"`
	}
	if err := json.Unmarshal(b, &overlay); err != nil {
		return err
	}
	if overlay.Source.Checksum == "" || overlay.Source.Checksum != c.Source.Checksum || len(overlay.Scenes) == 0 {
		return fmt.Errorf("layer revisit source mismatch or empty export")
	}
	seen := map[[3]uint32]bool{}
	for _, scene := range overlay.Scenes {
		key := [3]uint32{scene.Dungeon, uint32(scene.Maze), scene.Map}
		def, ok := c.Dungeons[scene.Dungeon]
		if !ok || seen[key] || scene.Source != c.Source.Checksum || scene.Quest == 0 ||
			scene.DungeonSHA256 == "" || def.Script.SHA256 != scene.DungeonSHA256 ||
			scene.MapSHA256 == "" || c.Maps[scene.Map].SHA256 != scene.MapSHA256 ||
			scene.ActionSHA256 == "" || scene.CinematicSHA256 == "" || scene.CinematicPath == "" ||
			scene.Record[0] != 0 || scene.Record[1] != 0 || scene.Record[2] != 0 || scene.Record[3] != 0 ||
			scene.Record[4] != 4 || scene.Record[5] != 5 {
			return fmt.Errorf("invalid source layer revisit %v", key)
		}
		matched := false
		resumeMatched := scene.ResumeMap == 0
		for _, maze := range def.Mazes {
			if maze.Index != scene.Maze || maze.Quest != scene.Quest {
				continue
			}
			for _, room := range maze.Rooms {
				if scene.ResumeMap != 0 && room.Map == scene.ResumeMap && [2]byte{room.X, room.Y} == scene.Position &&
					scene.ResumeMapSHA256 != "" && c.Maps[scene.ResumeMap].SHA256 == scene.ResumeMapSHA256 {
					resumeMatched = true
				}
			}
			for _, layer := range maze.Layers {
				if layer.Position == scene.Position && len(layer.Maps) > 0 && layer.Maps[len(layer.Maps)-1] == scene.Map {
					matched = true
				}
			}
		}
		if !matched || !resumeMatched {
			return fmt.Errorf("revisit differs from source final layer %v", key)
		}
		seen[key] = true
	}
	c.LayerRevisits = overlay.Scenes
	return nil
}
