package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Portal struct {
	Bounds [4]int32 `json:"bounds"`
	Town   uint32   `json:"town"`
	Area   uint32   `json:"area"`
}

type WorldArea struct {
	Town             uint32         `json:"town"`
	Area             uint32         `json:"area"`
	MapPath          string         `json:"map_path"`
	MinimumLevel     uint32         `json:"minimum_level"`
	Kind             string         `json:"kind"`
	Definition       []pvf.Token    `json:"definition"`
	Map              ScriptRecord   `json:"map"`
	ImportedScripts  []ScriptRecord `json:"imported_scripts,omitempty"`
	SeriaReturnWarp  bool           `json:"seria_return_warp,omitempty"`
	ReturnWarpBounds [][4]int32     `json:"return_warp_bounds,omitempty"`
	Walkable         [][4]int32     `json:"walkable"`
	Portals          []Portal       `json:"portals"`
	Imports          []string       `json:"imports,omitempty"`
	Pending          []string       `json:"pending,omitempty"`
}

type WorldCatalog struct {
	Source       pvf.ArchiveSnapshot  `json:"source"`
	TownIndex    ScriptRecord         `json:"town_index"`
	Towns        []ScriptRecord       `json:"towns"`
	Areas        map[string]WorldArea `json:"areas"`
	DungeonIndex ScriptRecord         `json:"dungeon_index"`
	Dungeons     []IndexEntry         `json:"dungeons"`
}

func AreaKey(town, area uint32) string { return fmt.Sprintf("%d/%d", town, area) }

func parseWorldAreas(town uint32, cells []pvf.Token) ([]WorldArea, error) {
	var rows []WorldArea
	for i := 0; i < len(cells); i++ {
		if cells[i].Type != 3 || cells[i].Text != "[area]" {
			continue
		}
		start := i + 1
		for i++; i < len(cells) && !(cells[i].Type == 3 && cells[i].Text == "[/area]"); i++ {
		}
		if i == len(cells) || i-start < 2 || cells[start].Type != 0 || cells[start].Value < 0 || cells[start+1].Type != 6 {
			return nil, fmt.Errorf("malformed area in town %d", town)
		}
		def := append([]pvf.Token(nil), cells[start:i]...)
		a := WorldArea{Town: town, Area: uint32(cells[start].Value), MapPath: "map/" + strings.ToLower(strings.ReplaceAll(cells[start+1].Text, "\\", "/")), Definition: def}
		permission := false
		for _, c := range def {
			if c.Type != 3 {
				continue
			}
			if c.Text == "[permission]" {
				permission = true
				continue
			}
			if c.Text == "[/permission]" {
				permission = false
				continue
			}
			if permission && c.Text != "[need level]" && c.Text != "[odyssey enter level]" {
				a.Pending = append(a.Pending, "unsupported permission "+c.Text)
			}
		}
		level := sectionCells(def, "[need level]")
		if len(level) > 0 {
			if len(level) != 1 || level[0].Type != 0 || level[0].Value < 0 {
				a.Pending = append(a.Pending, "conditional level rule requires interpretation")
			} else {
				a.MinimumLevel = uint32(level[0].Value)
			}
		}
		for _, c := range def {
			if c.Type == 6 && (c.Text == "[normal]" || c.Text == "[gate]" || c.Text == "[dungeon gate]") {
				a.Kind = c.Text
			}
		}
		rows = append(rows, a)
	}
	return rows, nil
}

func sourceRectangles(cells []pvf.Token, stride int) ([][]int32, error) {
	if len(cells)%stride != 0 {
		return nil, fmt.Errorf("incomplete rectangle rows")
	}
	var rows [][]int32
	for i := 0; i < len(cells); i += stride {
		row := make([]int32, stride)
		for j := range row {
			if cells[i+j].Type != 0 {
				return nil, fmt.Errorf("non-numeric rectangle cell")
			}
			row[j] = cells[i+j].Value
		}
		if row[2] < 0 || row[3] < 0 {
			return nil, fmt.Errorf("negative rectangle dimensions")
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func ImportWorld(a *pvf.Archive) (WorldCatalog, error) {
	w := WorldCatalog{Source: a.Snapshot(), Areas: map[string]WorldArea{}}
	var e error
	w.TownIndex, e = ReadScript(a, "list/town.lst")
	if e != nil {
		return w, e
	}
	list, e := ParseIndex(w.TownIndex.Cells)
	if e != nil {
		return w, e
	}
	for _, t := range list {
		s, e := ReadScript(a, t.Path)
		if e != nil {
			return w, e
		}
		w.Towns = append(w.Towns, s)
		areas, e := parseWorldAreas(t.ID, s.Cells)
		if e != nil {
			return w, e
		}
		for _, area := range areas {
			// Newer source entries already contain a root-qualified Contents/,
			// CommonMap/ or Live/ path; older ones are relative to Map/.
			rootPath := strings.TrimPrefix(area.MapPath, "map/")
			if f, ok := a.FindFile(rootPath); ok {
				area.MapPath = f.ArchivePath
			}
			key := AreaKey(area.Town, area.Area)
			if _, ok := w.Areas[key]; ok {
				return w, fmt.Errorf("duplicate area %s", key)
			}
			area.Map, e = ResolveScript(a, area.MapPath)
			if e != nil {
				area.Pending = append(area.Pending, e.Error())
				w.Areas[key] = area
				continue
			}
			cells := area.Map.Cells
			for i, c := range cells {
				if c.Type == 3 && c.Text == "[is seria room warp]" && i+1 < len(cells) && cells[i+1].Type == 0 && cells[i+1].Value == 1 {
					for j := i - 1; j >= 0; j-- {
						if cells[j].Type == 3 && cells[j].Text == "[movable area]" {
							break
						}
						if cells[j].Type == 3 && cells[j].Text == "[area]" && j+4 < i {
							row, e := sourceRectangles(cells[j+1:j+5], 4)
							if e == nil && len(row) == 1 {
								area.SeriaReturnWarp = true
								area.ReturnWarpBounds = append(area.ReturnWarpBounds, [4]int32{row[0][0], row[0][1], row[0][2], row[0][3]})
							}
							break
						}
					}
				}
			}
			imports, importErr := resolveMapImports(a, area.Map, map[string]bool{}, 0)
			if importErr != nil {
				area.Pending = append(area.Pending, importErr.Error())
			} else {
				area.ImportedScripts = imports
				for _, imported := range imports {
					cells = append(append([]pvf.Token(nil), cells...), imported.Cells...)
				}
			}
			walk, e := sourceRectangles(sectionCells(cells, "[virtual movable area]"), 4)
			if e != nil {
				area.Pending = append(area.Pending, e.Error())
			} else {
				for _, r := range walk {
					area.Walkable = append(area.Walkable, [4]int32{r[0], r[1], r[2], r[3]})
				}
			}
			portals, e := sourceRectangles(sectionCells(cells, "[town movable area]"), 6)
			if e != nil {
				area.Pending = append(area.Pending, e.Error())
			} else {
				for _, r := range portals {
					if r[4] < 0 || r[5] < 0 {
						area.Pending = append(area.Pending, "dynamic portal destination")
						continue
					}
					area.Portals = append(area.Portals, Portal{[4]int32{r[0], r[1], r[2], r[3]}, uint32(r[4]), uint32(r[5])})
				}
			}
			for _, c := range sectionCells(area.Map.Cells, "[import script]") {
				if c.Type == 6 {
					area.Imports = append(area.Imports, c.Text)
				} else {
					area.Pending = append(area.Pending, "unresolved import cell")
				}
			}
			if len(area.Walkable) == 0 {
				area.Pending = append(area.Pending, "no resolved walkable rectangles")
			}
			w.Areas[key] = area
		}
	}
	w.DungeonIndex, e = ReadScript(a, "list/dungeon.lst")
	if e != nil {
		return w, e
	}
	w.Dungeons, e = ParseIndex(w.DungeonIndex.Cells)
	return w, e
}

func resolveMapImports(a *pvf.Archive, script ScriptRecord, visiting map[string]bool, depth int) ([]ScriptRecord, error) {
	if visiting[script.Path] || depth > 16 {
		return nil, fmt.Errorf("cyclic or excessive map imports at %s", script.Path)
	}
	visiting[script.Path] = true
	defer delete(visiting, script.Path)
	var out []ScriptRecord
	for _, cell := range sectionCells(script.Cells, "[import script]") {
		if cell.Type != 6 {
			return nil, fmt.Errorf("unsupported map import cell")
		}
		name := strings.ToLower(strings.ReplaceAll(cell.Text, "\\", "/"))
		if _, ok := a.FindFile(name); !ok {
			name = "map/" + strings.TrimPrefix(name, "map/")
		}
		child, e := ResolveScript(a, name)
		if e != nil {
			return nil, e
		}
		more, e := resolveMapImports(a, child, visiting, depth+1)
		if e != nil {
			return nil, e
		}
		out = append(out, child)
		out = append(out, more...)
	}
	return out, nil
}

func LoadWorld(file string) (WorldCatalog, error) {
	var w WorldCatalog
	b, e := os.ReadFile(file)
	if e != nil {
		return w, e
	}
	if e = json.Unmarshal(b, &w); e != nil {
		return w, e
	}
	if len(w.Source.Checksum) != 64 || len(w.Areas) == 0 {
		return w, fmt.Errorf("invalid world catalog")
	}
	for key, area := range w.Areas {
		if key != AreaKey(area.Town, area.Area) {
			return w, fmt.Errorf("world area key mismatch")
		}
	}
	return w, nil
}
