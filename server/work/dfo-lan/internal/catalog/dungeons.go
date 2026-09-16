package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
)

type DungeonRoom struct {
	X, Y byte
	Map  uint32
	Boss bool
}
type DungeonMaze struct {
	Index             byte   `json:"index"`
	Quest             uint16 `json:"quest"`
	Size, Start, Boss [2]byte
	Rooms             []DungeonRoom `json:"rooms"`
	Pending           []string      `json:"pending,omitempty"`
}
type DungeonDefinition struct {
	ID                       uint32       `json:"id"`
	Script                   ScriptRecord `json:"script"`
	MinimumLevel, BasisLevel uint32
	Tutorial, NoFatigue      bool
	Mazes                    []DungeonMaze `json:"mazes"`
}
type DungeonCatalog struct {
	Source   pvf.ArchiveSnapshot          `json:"source"`
	Dungeons map[uint32]DungeonDefinition `json:"dungeons"`
	Maps     map[uint32]ScriptRecord      `json:"maps"`
}

func dungeonPair(c []pvf.Token) (r [2]byte, e error) {
	if len(c) != 2 {
		return r, fmt.Errorf("coordinate pair required")
	}
	for i, v := range c {
		if v.Type != 0 || v.Value < 0 || v.Value > 255 {
			return r, fmt.Errorf("invalid coordinate")
		}
		r[i] = byte(v.Value)
	}
	return r, nil
}
func ParseDungeon(id uint32, s ScriptRecord) (DungeonDefinition, error) {
	d := DungeonDefinition{ID: id, Script: s}
	for _, pair := range []struct {
		name string
		dst  *uint32
	}{{"[minimum required level]", &d.MinimumLevel}, {"[basis level]", &d.BasisLevel}} {
		v := sectionCells(s.Cells, pair.name)
		if len(v) != 1 || v[0].Type != 0 || v[0].Value < 1 {
			return d, fmt.Errorf("invalid %s", pair.name)
		}
		*pair.dst = uint32(v[0].Value)
	}
	for _, c := range s.Cells {
		if c.Type == 3 {
			d.Tutorial = d.Tutorial || c.Text == "[tutorial dungeon]"
			d.NoFatigue = d.NoFatigue || c.Text == "[no fatigue]"
		}
	}
	for i := 0; i < len(s.Cells); i++ {
		if s.Cells[i].Type != 3 || s.Cells[i].Text != "[maze info]" {
			continue
		}
		start := i + 1
		i++
		for i < len(s.Cells) && !(s.Cells[i].Type == 3 && s.Cells[i].Text == "[maze info]") {
			i++
		}
		c := s.Cells[start:i]
		i--
		if len(d.Mazes) >= 256 {
			return d, fmt.Errorf("too many mazes")
		}
		m := DungeonMaze{Index: byte(len(d.Mazes))}
		q := sectionCells(c, "[quest connection]")
		if len(q) > 0 {
			if len(q) != 3 || q[0].Type != 0 || q[0].Value != 0 || q[1].Type != 0 || q[1].Value < 1 || q[1].Value > 65535 || q[2].Type != 0 || q[2].Value != -1 {
				m.Pending = append(m.Pending, "unsupported quest connection")
			} else {
				m.Quest = uint16(q[1].Value)
			}
		}
		for _, p := range []struct {
			name string
			dst  *[2]byte
		}{{"[size]", &m.Size}, {"[start map]", &m.Start}, {"[boss map]", &m.Boss}} {
			v, e := dungeonPair(sectionCells(c, p.name))
			if e != nil {
				m.Pending = append(m.Pending, p.name+": "+e.Error())
			} else {
				*p.dst = v
			}
		}
		nodes := sectionCells(c, "[map specification]")
		if len(nodes) == 0 || len(nodes)%4 != 0 {
			m.Pending = append(m.Pending, "unsupported room specification")
		} else {
			for j := 0; j < len(nodes); j += 4 {
				xy, e := dungeonPair(nodes[j+1 : j+3])
				label := nodes[j]
				if e != nil || label.Type != 6 || (label.Text != "map" && label.Text != "boss") || nodes[j+3].Type != 0 || nodes[j+3].Value <= 0 || xy[0] >= m.Size[0] || xy[1] >= m.Size[1] {
					m.Pending = append(m.Pending, "invalid room specification")
					break
				}
				m.Rooms = append(m.Rooms, DungeonRoom{xy[0], xy[1], uint32(nodes[j+3].Value), label.Text == "boss"})
			}
		}
		d.Mazes = append(d.Mazes, m)
	}
	if len(d.Mazes) == 0 {
		return d, fmt.Errorf("no source maze")
	}
	return d, nil
}
func ImportDungeons(a *pvf.Archive, ids []uint32) (DungeonCatalog, error) {
	out := DungeonCatalog{Source: a.Snapshot(), Dungeons: map[uint32]DungeonDefinition{}, Maps: map[uint32]ScriptRecord{}}
	indices := make([]map[uint32]string, 2)
	for i, name := range []string{"list/dungeon.lst", "list/map.lst"} {
		s, e := ReadScript(a, name)
		if e != nil {
			return out, e
		}
		rows, e := ParseIndex(s.Cells)
		if e != nil {
			return out, e
		}
		indices[i] = map[uint32]string{}
		for _, r := range rows {
			indices[i][r.ID] = r.Path
		}
	}
	for _, id := range ids {
		name, ok := indices[0][id]
		if !ok {
			return out, fmt.Errorf("unknown dungeon %d", id)
		}
		s, e := ResolveScript(a, name)
		if e != nil {
			return out, e
		}
		d, e := ParseDungeon(id, s)
		if e != nil {
			return out, e
		}
		out.Dungeons[id] = d
		for _, m := range d.Mazes {
			for _, r := range m.Rooms {
				if _, ok := out.Maps[r.Map]; ok {
					continue
				}
				name, ok := indices[1][r.Map]
				if !ok {
					return out, fmt.Errorf("unknown map %d", r.Map)
				}
				s, e := ResolveScript(a, name)
				if e != nil {
					return out, e
				}
				out.Maps[r.Map] = s
			}
		}
	}
	return out, nil
}
func LoadDungeons(path string) (DungeonCatalog, error) {
	var c DungeonCatalog
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	if len(c.Source.Checksum) != 64 || len(c.Dungeons) == 0 {
		return c, fmt.Errorf("invalid dungeon catalog")
	}
	for id, d := range c.Dungeons {
		if id != d.ID {
			return c, fmt.Errorf("dungeon key mismatch")
		}
		parsed, e := ParseDungeon(id, d.Script)
		if e != nil {
			return c, e
		}
		c.Dungeons[id] = parsed
	}
	return c, nil
}
