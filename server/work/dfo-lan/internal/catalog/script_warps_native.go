package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/binary"
	"fmt"
	"path"
	"strings"
)

// Scope identifiers, witnessed records and key-room admission are server policy.
// Grid routes, maps, action bindings and hashes are read from the archive.
type ScriptWarpPolicy struct {
	Version int                   `json:"version"`
	Routes  []ScriptWarpSelection `json:"routes"`
}
type ScriptWarpSelection struct {
	Dungeon         uint32   `json:"dungeon"`
	Maze            byte     `json:"maze"`
	CinematicPath   string   `json:"cinematic_path,omitempty"`
	Monster         uint32   `json:"monster,omitempty"`
	ActionName      string   `json:"action_name,omitempty"`
	Record          [18]byte `json:"record"`
	RequiredKeyMaps []uint32 `json:"required_key_maps"`
}
type ScriptWarpRoute struct {
	Source          string   `json:"source"`
	Dungeon         uint32   `json:"dungeon"`
	Maze            byte     `json:"maze"`
	From            uint32   `json:"from_map"`
	To              uint32   `json:"to_map"`
	Position        [2]byte  `json:"position"`
	Target          [2]byte  `json:"target"`
	Record          [18]byte `json:"record"`
	DungeonSHA256   string   `json:"dungeon_sha256"`
	MapSHA256       string   `json:"map_sha256"`
	RequiredKeyMaps []uint32 `json:"required_key_maps"`
	CinematicPath   string   `json:"cinematic_path,omitempty"`
	CinematicSHA256 string   `json:"cinematic_sha256,omitempty"`
	ActionPath      string   `json:"action_path"`
	ActionSHA256    string   `json:"action_sha256"`
	ObjectSHA256    string   `json:"object_sha256,omitempty"`
}

func scriptWarpIndex(a *pvf.Archive, listing string, id uint32) (ScriptRecord, error) {
	s, err := ReadScript(a, listing)
	if err != nil {
		return ScriptRecord{}, err
	}
	var selected string
	for i, t := range s.Cells {
		if t.Type != 0 || uint32(t.Value) != id {
			continue
		}
		if i+1 >= len(s.Cells) || s.Cells[i+1].Type != 6 || selected != "" {
			return ScriptRecord{}, fmt.Errorf("ambiguous or invalid source binding %s:%d", listing, id)
		}
		selected = s.Cells[i+1].Text
	}
	if selected == "" {
		return ScriptRecord{}, fmt.Errorf("missing source binding %s:%d", listing, id)
	}
	return ResolveScript(a, selected)
}

func scriptWarpField(cells []pvf.Token, tag string) []pvf.Token {
	for i, t := range cells {
		if t.Type == 3 && t.Text == tag {
			end := i + 1
			for end < len(cells) && cells[end].Type != 3 {
				end++
			}
			return cells[i+1 : end]
		}
	}
	return nil
}

func scriptWarpIntegers(cells []pvf.Token, count int) ([]int32, error) {
	if len(cells) != count {
		return nil, fmt.Errorf("script warp requires %d numeric cells", count)
	}
	out := make([]int32, count)
	for i, t := range cells {
		if t.Type != 0 {
			return nil, fmt.Errorf("invalid script warp numeric cell")
		}
		out[i] = t.Value
	}
	return out, nil
}

func scriptWarpBlocks(cells []pvf.Token, tag string) [][]pvf.Token {
	var out [][]pvf.Token
	start := -1
	for i, t := range cells {
		if t.Type != 3 {
			continue
		}
		if t.Text == tag {
			start = i + 1
		}
		if t.Text == "[/"+tag[1:] && start >= 0 {
			out = append(out, cells[start:i])
			start = -1
		}
	}
	return out
}

func scriptWarpCinematicAction(a *pvf.Archive, cmt, m ScriptRecord) (ScriptRecord, ScriptRecord, error) {
	var action, object ScriptRecord
	for _, scene := range scriptWarpBlocks(cmt.Cells, "[BEHAVIOR]") {
		if len(scriptWarpBlocks(scene, "[SET ACTION]")) == 0 {
			continue
		}
		for _, actor := range scriptWarpBlocks(scene, "[ACTOR]") {
			var reference []pvf.Token
			if scriptWarpHas(actor, "[PASSIVE OBJECT INDEX]") {
				reference = scriptWarpField(actor, "[INDEX]")
			} else if scriptWarpHas(actor, "[PASSIVE OBJECT]") {
				reference = scriptWarpField(actor, "[OBJECT INDEX]")
				if len(reference) == 0 {
					ranks, err := scriptWarpIntegers(scriptWarpField(actor, "[INDEX]"), 1)
					if err != nil {
						return action, object, err
					}
					objects := scriptWarpField(m.Cells, "[passive object]")
					if len(objects)%4 != 0 || ranks[0] < 0 || int(ranks[0]) >= len(objects)/4 {
						return action, object, fmt.Errorf("unresolved cinematic map object owner")
					}
					reference = objects[4*ranks[0] : 4*ranks[0]+1]
				}
			} else {
				continue
			}
			ids, err := scriptWarpIntegers(reference, 1)
			if err != nil {
				return action, object, err
			}
			for _, set := range scriptWarpBlocks(scene, "[SET ACTION]") {
				ranks, err := scriptWarpIntegers(scriptWarpField(set, "[CUSTOM]"), 1)
				if err != nil {
					return action, object, err
				}
				obj, err := scriptWarpIndex(a, "list/passiveobject.lst", uint32(ids[0]))
				if err != nil {
					return action, object, err
				}
				paths := scriptWarpField(obj.Cells, "[etc action]")
				if ranks[0] < 0 || int(ranks[0]) >= len(paths) || paths[ranks[0]].Type != 6 {
					return action, object, fmt.Errorf("custom script warp action out of source bounds")
				}
				act, err := ReadScript(a, path.Join(path.Dir(obj.Path), strings.ReplaceAll(paths[ranks[0]].Text, "\\", "/")))
				if err != nil {
					return action, object, err
				}
				if !scriptWarpHas(act.Cells, "[MOVE MAP]") {
					continue
				}
				if action.Path != "" {
					return action, object, fmt.Errorf("multiple cinematic move actions")
				}
				action, object = act, obj
			}
		}
	}
	if action.Path == "" {
		return action, object, fmt.Errorf("cinematic has no source custom move action")
	}
	return action, object, nil
}

func scriptWarpHas(cells []pvf.Token, tag string) bool {
	for _, t := range cells {
		if t.Type == 3 && t.Text == tag {
			return true
		}
	}
	return false
}

func scriptWarpMapCinematic(a *pvf.Archive, m ScriptRecord, cmt ScriptRecord) error {
	field := scriptWarpField(m.Cells, "[basic action]")
	if len(field) != 1 || field[0].Type != 6 {
		return fmt.Errorf("script warp map has no unique basic action")
	}
	act, err := ReadScript(a, path.Join(path.Dir(m.Path), strings.ReplaceAll(field[0].Text, "\\", "/")))
	if err != nil {
		return err
	}
	matched := 0
	for i, t := range act.Cells {
		if t.Type == 3 && t.Text == "[CINEMATIC]" && i+1 < len(act.Cells) && act.Cells[i+1].Type == 0 {
			s, err := scriptWarpIndex(a, "list/cinematic.lst", uint32(act.Cells[i+1].Value))
			if err != nil {
				return err
			}
			if s.Path == cmt.Path && s.SHA256 == cmt.SHA256 {
				matched++
			}
		}
	}
	if matched != 1 {
		return fmt.Errorf("source map action does not uniquely bind selected cinematic")
	}
	return nil
}

func scriptWarpPosition(maze DungeonMaze, id uint32) ([2]byte, error) {
	var pos [2]byte
	found := false
	add := func(p [2]byte) error {
		if found && p != pos {
			return fmt.Errorf("map has multiple grid owners")
		}
		pos = p
		found = true
		return nil
	}
	for _, r := range maze.Rooms {
		if r.Map == id {
			if err := add([2]byte{r.X, r.Y}); err != nil {
				return pos, err
			}
		}
	}
	for _, l := range maze.Layers {
		for _, v := range l.Maps {
			if v == id {
				if err := add(l.Position); err != nil {
					return pos, err
				}
			}
		}
	}
	if !found {
		return pos, fmt.Errorf("warp source map absent from selected maze")
	}
	return pos, nil
}

func scriptWarpGrid(cells []pvf.Token) ([2]byte, error) {
	var out [2]byte
	v, err := scriptWarpIntegers(cells, 2)
	if err != nil {
		return out, err
	}
	for i, n := range v {
		if n < 0 || n > 255 {
			return out, fmt.Errorf("script warp grid overflow")
		}
		out[i] = byte(n)
	}
	return out, nil
}

func validateScriptWarpRecord(record [18]byte, pos []int32, random []int32) error {
	if record[0] != 1 || record[1] != 0 || record[2] != 0 || record[3] != 0 || record[4] != 5 || record[5] != 5 || len(pos) != 3 || len(random) != 3 {
		return fmt.Errorf("unverified script warp record header")
	}
	for i, n := range append(append([]int32(nil), pos...), random...) {
		if n < -32768 || n > 32767 || int16(binary.LittleEndian.Uint16(record[6+2*i:])) != int16(n) {
			return fmt.Errorf("witnessed record differs from source landing field %d", i)
		}
	}
	return nil
}

func ImportScriptWarpRoutes(a *pvf.Archive, d DungeonCatalog, policy ScriptWarpPolicy) ([]ScriptWarpRoute, error) {
	if a == nil || a.Snapshot().Checksum != d.Source.Checksum || policy.Version != 1 || len(policy.Routes) == 0 {
		return nil, fmt.Errorf("script warp source or policy mismatch")
	}
	out := make([]ScriptWarpRoute, 0, len(policy.Routes))
	seen := map[[3]uint32]bool{}
	for _, p := range policy.Routes {
		def, ok := d.Dungeons[p.Dungeon]
		if !ok {
			return nil, fmt.Errorf("missing warp dungeon %d", p.Dungeon)
		}
		var maze *DungeonMaze
		for i := range def.Mazes {
			if def.Mazes[i].Index == p.Maze {
				maze = &def.Mazes[i]
			}
		}
		if maze == nil {
			return nil, fmt.Errorf("missing warp maze")
		}
		r := ScriptWarpRoute{Source: d.Source.Checksum, Dungeon: p.Dungeon, Maze: p.Maze, Record: p.Record, DungeonSHA256: def.Script.SHA256, RequiredKeyMaps: append([]uint32{}, p.RequiredKeyMaps...)}
		var act ScriptRecord
		var landing, random []int32
		if p.CinematicPath != "" {
			if p.Monster != 0 || p.ActionName != "" {
				return nil, fmt.Errorf("mixed cinematic and monster warp selection")
			}
			cmt, err := ReadScript(a, p.CinematicPath)
			if err != nil {
				return nil, err
			}
			maps, err := scriptWarpIntegers(scriptWarpField(cmt.Cells, "[MAP]"), 1)
			if err != nil || maps[0] <= 0 {
				return nil, fmt.Errorf("invalid cinematic map")
			}
			r.From = uint32(maps[0])
			// A CMT may start from a monster action later in the run rather
			// than the map's basic action. Its own [MAP] and native list binding
			// establish ownership; the selected path only limits enabled scope.
			listing, err := ReadScript(a, "list/cinematic.lst")
			if err != nil {
				return nil, err
			}
			bindings := 0
			for n, t := range listing.Cells {
				if t.Type == 6 && strings.ToLower(strings.ReplaceAll(t.Text, "\\", "/")) == cmt.Path {
					if n == 0 || listing.Cells[n-1].Type != 0 || listing.Cells[n-1].Value <= 0 {
						return nil, fmt.Errorf("invalid cinematic list owner")
					}
					bindings++
				}
			}
			if bindings != 1 {
				return nil, fmt.Errorf("selected cinematic lacks a unique native list binding")
			}
			var obj ScriptRecord
			from, err := d.MapScript(r.From)
			if err != nil {
				return nil, err
			}
			act, obj, err = scriptWarpCinematicAction(a, cmt, from)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", cmt.Path, err)
			}
			moves := scriptWarpBlocks(act.Cells, "[MOVE MAP]")
			if len(moves) != 1 || !scriptWarpHas(moves[0], "[IS FORCE]") || !scriptWarpHas(moves[0], "[IS IGNORE STATE CHECK]") {
				return nil, fmt.Errorf("unsupported cinematic move")
			}
			values, err := scriptWarpIntegers(scriptWarpField(moves[0], "[POS]"), 5)
			if err != nil {
				return nil, err
			}
			r.Target, err = scriptWarpGrid(scriptWarpField(moves[0], "[POS]")[:2])
			if err != nil {
				return nil, err
			}
			landing, random = values[2:], []int32{0, 0, 0}
			r.CinematicPath, r.CinematicSHA256, r.ObjectSHA256 = cmt.Path, cmt.SHA256, obj.SHA256
		} else {
			if p.Monster == 0 || p.ActionName == "" {
				return nil, fmt.Errorf("missing forced warp actor selection")
			}
			mob, err := scriptWarpIndex(a, "list/monster.lst", p.Monster)
			if err != nil {
				return nil, err
			}
			for _, t := range scriptWarpField(mob.Cells, "[etc action]") {
				if t.Type == 6 && strings.EqualFold(t.Text, p.ActionName) {
					if act.Path != "" {
						return nil, fmt.Errorf("duplicate monster move action")
					}
					act, err = ReadScript(a, path.Join(path.Dir(mob.Path), strings.ReplaceAll(t.Text, "\\", "/")))
					if err != nil {
						return nil, err
					}
				}
			}
			moves := scriptWarpBlocks(act.Cells, "[KICK OUT MAP CHARACTER]")
			if len(moves) != 1 || !scriptWarpHas(moves[0], "[INCLUDE DEAD]") {
				return nil, fmt.Errorf("unverified forced move action")
			}
			r.Target, err = scriptWarpGrid(scriptWarpField(moves[0], "[MOVE MAP GRID]"))
			if err != nil {
				return nil, err
			}
			landing, err = scriptWarpIntegers(scriptWarpField(moves[0], "[MOVE MAP START POS]"), 3)
			if err != nil {
				return nil, err
			}
			rangeCells, err := scriptWarpIntegers(scriptWarpField(moves[0], "[MOVE MAP START POS RANDOM RANGE]"), 2)
			if err != nil {
				return nil, err
			}
			random = append(rangeCells, 0)
			for _, room := range maze.Rooms {
				if _, ok := d.Maps[room.Map]; !ok {
					continue
				}
				script, err := d.MapScript(room.Map)
				if err != nil {
					return nil, err
				}
				for _, monster := range scriptWarpBlocks(script.Cells, "[monster]") {
					if len(monster) > 0 && monster[0].Type == 0 && uint32(monster[0].Value) == p.Monster {
						if r.From != 0 && r.From != room.Map {
							return nil, fmt.Errorf("multiple forced actor map owners")
						}
						r.From = room.Map
					}
				}
			}
			if r.From == 0 {
				return nil, fmt.Errorf("forced actor absent from selected maze")
			}
		}
		var err error
		r.Position, err = scriptWarpPosition(*maze, r.From)
		if err != nil {
			return nil, err
		}
		matches, declaredFrom := 0, 0
		for _, warp := range scriptWarpBlocks(def.Script.Cells, "[warp map condition]") {
			from, err := scriptWarpGrid(scriptWarpField(warp, "[source grid pos]"))
			if err != nil {
				return nil, err
			}
			to, err := scriptWarpGrid(scriptWarpField(warp, "[dest grid pos]"))
			if err != nil {
				return nil, err
			}
			if from == r.Position {
				declaredFrom++
				if to == r.Target {
					matches++
				}
			}
		}
		// Force/ignore-state ACT routes may omit a DGN warp condition.
		// An explicit condition for this source grid must agree, if present.
		if declaredFrom > 0 && (matches != 1 || declaredFrom != 1) {
			return nil, fmt.Errorf("warp action differs from dungeon condition: dungeon=%d maze=%d from=%d position=%v target=%v matches=%d", r.Dungeon, r.Maze, r.From, r.Position, r.Target, matches)
		}
		for _, room := range maze.Rooms {
			if [2]byte{room.X, room.Y} == r.Target {
				if r.To != 0 || len(room.Alternates) > 0 {
					return nil, fmt.Errorf("ambiguous warp target room")
				}
				r.To = room.Map
			}
		}
		if r.To == 0 || d.Maps[r.To].SHA256 == "" {
			return nil, fmt.Errorf("warp target absent from source catalog")
		}
		if err := validateScriptWarpRecord(r.Record, landing, random); err != nil {
			return nil, err
		}
		for _, id := range r.RequiredKeyMaps {
			if _, err := scriptWarpPosition(*maze, id); err != nil {
				return nil, fmt.Errorf("required key room: %w", err)
			}
		}
		r.MapSHA256 = d.Maps[r.From].SHA256
		r.ActionPath, r.ActionSHA256 = act.Path, act.SHA256
		key := [3]uint32{r.Dungeon, uint32(r.Maze), r.From}
		if seen[key] {
			return nil, fmt.Errorf("duplicate selected script warp")
		}
		seen[key] = true
		out = append(out, r)
	}
	return out, nil
}
