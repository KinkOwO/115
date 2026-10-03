package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/binary"
	"fmt"
	"path"
	"strings"
)

type LayerRevisitOverlay struct {
	Source struct {
		Checksum string `json:"checksum"`
	} `json:"source"`
	Scenes []DungeonLayerRevisit `json:"layer_revisits"`
}
type LayerRevisitPolicy struct {
	Version int                     `json:"version"`
	Scenes  []LayerRevisitSelection `json:"scenes"`
}
type LayerRevisitSelection struct {
	Dungeon       uint32   `json:"dungeon"`
	Maze          byte     `json:"maze"`
	CinematicPath string   `json:"cinematic_path"`
	Record        [18]byte `json:"record"`
	ResumeBase    bool     `json:"resume_base"`
}

func ImportLayerRevisits(a *pvf.Archive, d DungeonCatalog, p LayerRevisitPolicy) (LayerRevisitOverlay, error) {
	var out LayerRevisitOverlay
	if a == nil || a.Snapshot().Checksum != d.Source.Checksum || p.Version != 1 || len(p.Scenes) == 0 {
		return out, fmt.Errorf("layer revisit source or policy mismatch")
	}
	out.Source.Checksum = d.Source.Checksum
	for _, selection := range p.Scenes {
		def, ok := d.Dungeons[selection.Dungeon]
		if !ok {
			return out, fmt.Errorf("missing layer revisit dungeon")
		}
		var maze *DungeonMaze
		for i := range def.Mazes {
			if def.Mazes[i].Index == selection.Maze {
				maze = &def.Mazes[i]
			}
		}
		if maze == nil || maze.Quest == 0 {
			return out, fmt.Errorf("layer revisit needs a source quest maze")
		}
		cmt, err := ReadScript(a, selection.CinematicPath)
		if err != nil {
			return out, err
		}
		ids, err := scriptWarpIntegers(scriptWarpField(cmt.Cells, "[MAP]"), 1)
		if err != nil || ids[0] <= 0 {
			return out, fmt.Errorf("invalid layer revisit cinematic map")
		}
		id := uint32(ids[0])
		m, err := d.MapScript(id)
		if err != nil {
			return out, fmt.Errorf("layer revisit map: %w", err)
		}
		if err := scriptWarpMapCinematic(a, m, cmt); err != nil {
			return out, err
		}
		fields := scriptWarpField(m.Cells, "[basic action]")
		act, err := ReadScript(a, path.Join(path.Dir(m.Path), strings.ReplaceAll(fields[0].Text, "\\", "/")))
		if err != nil {
			return out, err
		}
		if len(scriptWarpBlocks(cmt.Cells, "[CHANGE MAP]")) != 1 {
			return out, fmt.Errorf("ambiguous layer revisit ending")
		}
		xmin, xmax, ymin, ymax, ok := changeMapArea(cmt.Cells)
		if !ok {
			return out, fmt.Errorf("invalid layer revisit landing")
		}
		if err := validateLayerRevisitRecord(selection.Record, xmin, xmax, ymin, ymax); err != nil {
			return out, err
		}
		scene := DungeonLayerRevisit{Source: d.Source.Checksum, Dungeon: def.ID, Maze: maze.Index, Quest: maze.Quest, Map: id, Record: selection.Record, DungeonSHA256: def.Script.SHA256, MapSHA256: m.SHA256, ActionSHA256: act.SHA256, CinematicPath: cmt.Path, CinematicSHA256: cmt.SHA256}
		found := false
		for _, layer := range maze.Layers {
			if len(layer.Maps) > 0 && layer.Maps[len(layer.Maps)-1] == id {
				if found {
					return out, fmt.Errorf("multiple final layer owners")
				}
				found = true
				scene.Position = layer.Position
			}
		}
		if !found {
			return out, fmt.Errorf("cinematic is not a source final layer")
		}
		if selection.ResumeBase {
			for _, room := range maze.Rooms {
				if [2]byte{room.X, room.Y} == scene.Position {
					if scene.ResumeMap != 0 || len(room.Alternates) > 0 || room.Map == id {
						return out, fmt.Errorf("ambiguous layer revisit base")
					}
					scene.ResumeMap = room.Map
					scene.ResumeMapSHA256 = d.Maps[room.Map].SHA256
				}
			}
			if scene.ResumeMap == 0 {
				return out, fmt.Errorf("missing layer revisit base")
			}
		}
		out.Scenes = append(out.Scenes, scene)
	}
	copy := d
	if err := ApplyLayerRevisits(&copy, out); err != nil {
		return out, err
	}
	return out, nil
}

func validateLayerRevisitRecord(r [18]byte, xmin, xmax, ymin, ymax uint16) error {
	if r[0] != 0 || r[1] != 0 || r[2] != 0 || r[3] != 0 || r[4] != 4 || r[5] != 5 {
		return fmt.Errorf("unverified layer revisit record header")
	}
	x, y := binary.LittleEndian.Uint16(r[6:8]), binary.LittleEndian.Uint16(r[8:10])
	if x < xmin || x > xmax || y < ymin || y > ymax {
		return fmt.Errorf("layer revisit record differs from source landing")
	}
	for _, b := range r[10:] {
		if b != 0 {
			return fmt.Errorf("unverified layer revisit record tail")
		}
	}
	return nil
}
