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

// PhaseNPC records an NPC row from a town area's source [phase] map.
// Keeping the map identity makes the placement auditable without embedding
// every phase map script in the world catalog.
type PhaseNPC struct {
	MapPath   string `json:"map_path"`
	MapSHA256 string `json:"map_sha256"`
	ID        uint32 `json:"id"`
	X         uint16 `json:"x"`
	Y         uint16 `json:"y"`
}

type WorldArea struct {
	Town         uint32 `json:"town"`
	Area         uint32 `json:"area"`
	MapPath      string `json:"map_path"`
	MinimumLevel uint32 `json:"minimum_level"`
	// OdysseyMinimumLevel is the source [odyssey enter level] value of the
	// [permission] block. For an Arad Odyssey user (XORSTR "[is arad odyssey
	// user]") the client admits entry at the LOWER of the two source values:
	// Storm Pass (43/*, need 50 / odyssey 45) lets a 45 in and its own refusal
	// (DSTR 535) names 45, while West Coast (40/0, need 15 / odyssey 35) lets
	// a 20 in and names 15 (DSTR 30069) — a higher Odyssey value never raises
	// the entry gate, so the server has to mirror the min rule or it refuses
	// source-legal progression travel. Zero means the source defines no
	// Odyssey value for this area.
	OdysseyMinimumLevel uint32 `json:"odyssey_minimum_level,omitempty"`
	// Older generated catalogs used this field name. Preserve it during a
	// phase-row refresh so unrelated source data is never discarded.
	OdysseyEnterLevel uint32         `json:"odyssey_enter_level,omitempty"`
	Kind              string         `json:"kind"`
	Definition        []pvf.Token    `json:"definition"`
	Map               ScriptRecord   `json:"map"`
	ImportedScripts   []ScriptRecord `json:"imported_scripts,omitempty"`
	PhaseNPCs         []PhaseNPC     `json:"phase_npcs,omitempty"`
	SeriaReturnWarp   bool           `json:"seria_return_warp,omitempty"`
	ReturnWarpBounds  [][4]int32     `json:"return_warp_bounds,omitempty"`
	Walkable          [][4]int32     `json:"walkable"`
	Portals           []Portal       `json:"portals"`
	Imports           []string       `json:"imports,omitempty"`
	Pending           []string       `json:"pending,omitempty"`
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

// areaLevelGate reads one "[tag] <level>" pair out of an area definition.
// It reports whether the tag exists at all; a conditional or non-numeric value
// is an error so the area is reported as unresolved instead of silently
// gaining a guessed gate. Callers keep the source value, never a default.
func areaLevelGate(def []pvf.Token, tag string) (uint32, bool, error) {
	cells := sectionCells(def, tag)
	if len(cells) == 0 {
		return 0, false, nil
	}
	if len(cells) != 1 || cells[0].Type != 0 || cells[0].Value < 0 {
		return 0, false, fmt.Errorf("unsupported %s rule", tag)
	}
	return uint32(cells[0].Value), true, nil
}

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
		level, hasLevel, levelErr := areaLevelGate(def, "[need level]")
		if levelErr != nil {
			a.Pending = append(a.Pending, "conditional level rule requires interpretation")
		} else if hasLevel {
			a.MinimumLevel = level
		}
		odyssey, hasOdyssey, odysseyErr := areaLevelGate(def, "[odyssey enter level]")
		if odysseyErr != nil {
			a.Pending = append(a.Pending, "conditional odyssey level rule requires interpretation")
		} else if hasOdyssey {
			a.OdysseyMinimumLevel = odyssey
		}
		for _, c := range def {
			if c.Type == 6 && (c.Text == "[normal]" || c.Text == "[gate]" || c.Text == "[dungeon gate]") {
				a.Kind = c.Text
				if c.Text == "[dungeon gate]" {
					break
				}
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
			for _, phase := range sectionCells(area.Definition, "[phase]") {
				if phase.Type != 6 {
					area.Pending = append(area.Pending, "unsupported phase map cell")
					continue
				}
				name := strings.ToLower(strings.ReplaceAll(phase.Text, "\\", "/"))
				if _, found := a.FindFile(name); !found {
					name = "map/" + strings.TrimPrefix(name, "map/")
				}
				phaseMap, phaseErr := ResolveScript(a, name)
				if phaseErr != nil {
					area.Pending = append(area.Pending, phaseErr.Error())
					continue
				}
				area.PhaseNPCs = append(area.PhaseNPCs, sourcePhaseNPCs(phaseMap)...)
				phaseImports, importErr := resolveMapImports(a, phaseMap, map[string]bool{}, 0)
				if importErr != nil {
					area.Pending = append(area.Pending, importErr.Error())
					continue
				}
				for _, imported := range phaseImports {
					area.PhaseNPCs = append(area.PhaseNPCs, sourcePhaseNPCs(imported)...)
				}
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
			walk, e := sourceRectangles(nestedSectionCells(cells, "[virtual movable area]"), 4)
			if e != nil {
				area.Pending = append(area.Pending, e.Error())
			} else {
				for _, r := range walk {
					area.Walkable = append(area.Walkable, [4]int32{r[0], r[1], r[2], r[3]})
				}
			}
			portals, e := sourceRectangles(nestedSectionCells(cells, "[town movable area]"), 6)
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

func sourcePhaseNPCs(script ScriptRecord) []PhaseNPC {
	var result []PhaseNPC
	cells := script.Cells
	for i, c := range cells {
		if c.Type != 3 || c.Text != "[NPC]" {
			continue
		}
		for i++; i < len(cells) && cells[i].Type != 3; i += 5 {
			if i+4 >= len(cells) || cells[i].Type != 0 || cells[i+1].Type != 6 ||
				cells[i+2].Type != 0 || cells[i+3].Type != 0 || cells[i+4].Type != 0 {
				break
			}
			id, x, y := cells[i].Value, cells[i+2].Value, cells[i+3].Value
			if id > 0 && x >= 0 && x <= 65535 && y >= 0 && y <= 65535 {
				result = append(result, PhaseNPC{MapPath: script.Path, MapSHA256: script.SHA256, ID: uint32(id), X: uint16(x), Y: uint16(y)})
			}
		}
	}
	return result
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
		// Catalogs exported before the Odyssey gate existed still carry the
		// whole area definition, so the field is recovered from it instead of
		// forcing a re-export of the 31 MB world catalog (same compatibility
		// rule as the character catalog's backfill).
		if area.OdysseyMinimumLevel == 0 {
			if level, ok, e := areaLevelGate(area.Definition, "[odyssey enter level]"); e == nil && ok {
				area.OdysseyMinimumLevel = level
				w.Areas[key] = area
			}
		}
	}
	return w, nil
}
