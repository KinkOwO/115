package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type DazzlementDungeon struct {
	ID            uint32        `json:"id"`
	DungeonSHA256 string        `json:"dungeon_sha256"`
	Rooms         []DungeonRoom `json:"rooms"`
}

type DazzlementOverlay struct {
	SourceChecksum string                  `json:"source_checksum"`
	Dungeons       []DazzlementDungeon     `json:"dungeons"`
	Maps           map[uint32]ScriptRecord `json:"maps"`
}

// ImportDazzlementOverlay uses the tower's explicit encounter list and each
// MAP's [dungeon]/[type] fields. These DGN mazes omit [map specification].
func ImportDazzlementOverlay(a *pvf.Archive) (DazzlementOverlay, error) {
	var out DazzlementOverlay
	if a == nil {
		return out, fmt.Errorf("nil Dazzlement source archive")
	}
	out.SourceChecksum = a.Snapshot().Checksum
	rules, err := ReadScript(a, "etc/towerofdazzlement.etc")
	if err != nil {
		return out, err
	}
	max := sectionCells(rules.Cells, "[max layer]")
	base := sectionCells(rules.Cells, "[base layer dungeon index]")
	if len(max) != 1 || max[0].Type != 0 || max[0].Value != 7 || len(base) != 1 || base[0].Type != 0 || base[0].Value != 7601 {
		return out, fmt.Errorf("unexpected Dazzlement tower header")
	}
	list := sectionCells(rules.Cells, "[dungeon list]")
	if len(list) != 33*3 {
		return out, fmt.Errorf("Dazzlement encounter list has %d cells", len(list))
	}
	dungeonIndex, err := ReadScript(a, "list/dungeon.lst")
	if err != nil {
		return out, err
	}
	dungeonRows, err := ParseIndex(dungeonIndex.Cells)
	if err != nil {
		return out, err
	}
	paths := make(map[uint32]string, len(dungeonRows))
	for _, row := range dungeonRows {
		paths[row.ID] = row.Path
	}
	indexByID := make(map[uint32]int, 33)
	classByID := make(map[uint32]int32, 33)
	for i := 0; i < len(list); i += 3 {
		id, class, weight := list[i], list[i+1], list[i+2]
		if id.Type != 0 || id.Value <= 0 || class.Type != 0 || class.Value < 0 || class.Value > 2 || weight.Type != 0 || weight.Value <= 0 {
			return out, fmt.Errorf("invalid Dazzlement encounter row %d", i/3)
		}
		value := uint32(id.Value)
		if _, duplicate := indexByID[value]; duplicate {
			return out, fmt.Errorf("duplicate Dazzlement dungeon %d", value)
		}
		path := paths[value]
		if !strings.HasPrefix(path, "dungeon/towers/towerofdazzlement") || !strings.HasSuffix(path, ".dgn") {
			return out, fmt.Errorf("Dazzlement dungeon %d path mismatch", value)
		}
		script, err := ResolveScript(a, path)
		if err != nil {
			return out, err
		}
		indexByID[value] = len(out.Dungeons)
		classByID[value] = class.Value
		out.Dungeons = append(out.Dungeons, DazzlementDungeon{ID: value, DungeonSHA256: script.SHA256})
	}
	mapIndex, err := ReadScript(a, "list/map.lst")
	if err != nil {
		return out, err
	}
	mapRows, err := ParseIndex(mapIndex.Cells)
	if err != nil {
		return out, err
	}
	out.Maps = make(map[uint32]ScriptRecord, 56)
	for _, row := range mapRows {
		if !strings.HasPrefix(row.Path, "map/towerofdazzlement/") || !strings.HasSuffix(row.Path, ".map") {
			continue
		}
		script, err := ResolveScript(a, row.Path)
		if err != nil {
			return out, err
		}
		owner := sectionCells(script.Cells, "[dungeon]")
		typ := sectionCells(script.Cells, "[type]")
		if len(owner) != 1 || owner[0].Type != 0 || owner[0].Value <= 0 || len(typ) != 1 || typ[0].Type != 6 {
			return out, fmt.Errorf("Dazzlement map %d has invalid owner or type", row.ID)
		}
		id := uint32(owner[0].Value)
		i, ok := indexByID[id]
		if !ok {
			return out, fmt.Errorf("Dazzlement map %d has unmatched owner %d", row.ID, id)
		}
		var room DungeonRoom
		room.Map = row.ID
		switch classByID[id] {
		case 1:
			if typ[0].Text != "[normal]" {
				return out, fmt.Errorf("Dazzlement single room %d has type %q", row.ID, typ[0].Text)
			}
			room.Boss = true
		default:
			switch typ[0].Text {
			case "[normal]":
			case "[boss]":
				room.X, room.Boss = 1, true
			default:
				return out, fmt.Errorf("Dazzlement map %d has type %q", row.ID, typ[0].Text)
			}
		}
		for _, existing := range out.Dungeons[i].Rooms {
			if existing.X == room.X {
				return out, fmt.Errorf("Dazzlement dungeon %d has duplicate room", id)
			}
		}
		out.Dungeons[i].Rooms = append(out.Dungeons[i].Rooms, room)
		out.Maps[row.ID] = script
	}
	if len(out.Maps) != 56 {
		return out, fmt.Errorf("Dazzlement map count %d, want 56", len(out.Maps))
	}
	for _, d := range out.Dungeons {
		want := 2
		if classByID[d.ID] == 1 {
			want = 1
		}
		if len(d.Rooms) != want {
			return out, fmt.Errorf("Dazzlement dungeon %d has %d rooms, want %d", d.ID, len(d.Rooms), want)
		}
	}
	return out, nil
}

// AttachDazzlementMaps resolves all source-listed encounters without changing
// their separate weekly progression or reward protocol.
func AttachDazzlementMaps(c *DungeonCatalog, path string) error {
	if c == nil {
		return fmt.Errorf("nil dungeon catalog")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var overlay DazzlementOverlay
	if err := json.Unmarshal(b, &overlay); err != nil {
		return err
	}
	if overlay.SourceChecksum != c.Source.Checksum || len(overlay.Dungeons) != 33 || len(overlay.Maps) != 56 {
		return fmt.Errorf("Dazzlement overlay source mismatch or incomplete")
	}
	resolved := make(map[uint32]DungeonDefinition, 33)
	used := make(map[uint32]bool, 56)
	for _, encounter := range overlay.Dungeons {
		d, ok := c.Dungeons[encounter.ID]
		if !ok || !strings.HasPrefix(d.Script.Path, "dungeon/towers/towerofdazzlement") || d.Script.SHA256 != encounter.DungeonSHA256 || len(d.Mazes) != 1 || len(encounter.Rooms) < 1 || len(encounter.Rooms) > 2 {
			return fmt.Errorf("Dazzlement dungeon %d identity mismatch", encounter.ID)
		}
		m := &d.Mazes[0]
		if m.Quest != 0 || m.Size != [2]byte{byte(len(encounter.Rooms)), 1} || m.Start != [2]byte{} || m.Boss != [2]byte{byte(len(encounter.Rooms) - 1), 0} || len(m.Rooms) != 0 || len(m.Pending) != 1 || m.Pending[0] != "unsupported room specification" {
			return fmt.Errorf("Dazzlement dungeon %d maze mismatch", encounter.ID)
		}
		rooms := make([]DungeonRoom, len(encounter.Rooms))
		for _, room := range encounter.Rooms {
			if room.Y != 0 || int(room.X) >= len(rooms) || used[room.Map] || room.Boss != (int(room.X) == len(rooms)-1) {
				return fmt.Errorf("Dazzlement dungeon %d room mismatch", encounter.ID)
			}
			script, ok := overlay.Maps[room.Map]
			owner := sectionCells(script.Cells, "[dungeon]")
			if !ok || len(script.SHA256) != 64 || !strings.HasPrefix(script.Path, "map/towerofdazzlement/") || len(owner) != 1 || owner[0].Type != 0 || owner[0].Value != int32(encounter.ID) {
				return fmt.Errorf("Dazzlement map %d source mismatch", room.Map)
			}
			if existing, ok := c.Maps[room.Map]; ok && existing.SHA256 != script.SHA256 {
				return fmt.Errorf("Dazzlement map %d conflicts with catalog", room.Map)
			}
			rooms[room.X] = room
			used[room.Map] = true
		}
		for x, room := range rooms {
			if room.Map == 0 || room.X != byte(x) {
				return fmt.Errorf("Dazzlement dungeon %d missing room %d", encounter.ID, x)
			}
		}
		m.Rooms, m.Pending = rooms, nil
		resolved[encounter.ID] = d
	}
	if len(resolved) != 33 || len(used) != len(overlay.Maps) {
		return fmt.Errorf("Dazzlement overlay has duplicate or unused entries")
	}
	for id, d := range resolved {
		c.Dungeons[id] = d
	}
	for id, script := range overlay.Maps {
		c.Maps[id] = script
	}
	return nil
}
