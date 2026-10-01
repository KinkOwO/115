package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"path"
)

type EmblemCompoundKey struct {
	Grade int32
	Count int
}

type EmblemCompoundRules struct {
	Source string
	Rolls  map[EmblemCompoundKey][4]uint32
	Pools  map[int32][]uint32
	Grades map[uint32]int32
}

func emblemCompoundSection(cells []pvf.Token, name string) []pvf.Token {
	var out []pvf.Token
	active := false
	for _, t := range cells {
		if t.Type == 3 {
			active = t.Text == name
			continue
		}
		if active {
			out = append(out, t)
		}
	}
	return out
}

// Native 147AB2B80 indexes probability rows by (emblem grade,input count).
// Four probabilities correspond to emblemlist.lst grades 1 through 4.
func ImportEmblemCompoundRules(a *pvf.Archive, index catalog.ItemIndex) (*EmblemCompoundRules, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("emblem compound source mismatch")
	}
	r := &EmblemCompoundRules{Source: a.Snapshot().Checksum, Rolls: map[EmblemCompoundKey][4]uint32{}, Pools: map[int32][]uint32{}, Grades: map[uint32]int32{}}
	s, e := catalog.ResolveScript(a, "etc/emblem/emblemcompound.etc")
	if e != nil {
		return nil, e
	}
	rows := emblemCompoundSection(s.Cells, "[emblem compound info]")
	if len(rows) == 0 || len(rows)%6 != 0 {
		return nil, fmt.Errorf("invalid source emblem compound rows")
	}
	for i := 0; i < len(rows); i += 6 {
		for _, t := range rows[i : i+6] {
			if t.Type != 0 || t.Value < 0 {
				return nil, fmt.Errorf("invalid source emblem compound value")
			}
		}
		key := EmblemCompoundKey{rows[i].Value, int(rows[i+1].Value)}
		if key.Grade < 1 || key.Grade > 4 || key.Count < 2 || key.Count > 5 {
			return nil, fmt.Errorf("unsupported source emblem compound key")
		}
		if _, exists := r.Rolls[key]; exists {
			return nil, fmt.Errorf("duplicate source emblem compound key")
		}
		v := [4]uint32{uint32(rows[i+2].Value), uint32(rows[i+3].Value), uint32(rows[i+4].Value), uint32(rows[i+5].Value)}
		if uint64(v[0])+uint64(v[1])+uint64(v[2])+uint64(v[3]) != 100 {
			return nil, fmt.Errorf("invalid source emblem compound probability")
		}
		r.Rolls[key] = v
	}
	s, e = catalog.ResolveScript(a, "etc/emblem/emblemlist.lst")
	if e != nil {
		return nil, e
	}
	lists, e := catalog.ParseIndex(s.Cells)
	if e != nil {
		return nil, e
	}
	for _, list := range lists {
		if list.ID < 1 || list.ID > 4 {
			continue
		}
		s, e = catalog.ResolveScript(a, path.Join("etc/emblem", list.Path))
		if e != nil {
			return nil, e
		}
		inGroup, common, inItems := false, false, false
		seen := map[uint32]bool{}
		for i, t := range s.Cells {
			if t.Type == 3 {
				switch t.Text {
				case "[group]":
					inGroup, common, inItems = true, true, false
				case "[/group]":
					inGroup, inItems = false, false
				case "[grow type]":
					if i+2 >= len(s.Cells) || s.Cells[i+1].Type != 6 || s.Cells[i+2].Type != 0 {
						return nil, fmt.Errorf("invalid source emblem compound group")
					}
					common = s.Cells[i+1].Text == "common" && s.Cells[i+2].Value == 0
				case "[item list]":
					inItems = true
				default:
					inItems = false
				}
				continue
			}
			if !inGroup || !common || !inItems {
				continue
			}
			if t.Type != 0 || t.Value < 2 {
				return nil, fmt.Errorf("invalid source emblem compound template")
			}
			id := uint32(t.Value)
			item, ok := index.Items[id]
			if !ok || item.Kind != "stackable" || item.StackableType != "[avatar emblem]" {
				return nil, fmt.Errorf("source emblem %d absent or wrong type", id)
			}
			definition, e := catalog.ResolveScript(a, item.Path)
			if e != nil {
				return nil, e
			}
			grade := emblemCompoundSection(definition.Cells, "[grade]")
			if len(grade) != 1 || grade[0].Type != 0 || grade[0].Value != int32(list.ID) {
				return nil, fmt.Errorf("source emblem %d grade mismatch", id)
			}
			if seen[id] {
				return nil, fmt.Errorf("duplicate source emblem compound pool item")
			}
			seen[id] = true
			r.Grades[id] = int32(list.ID)
			r.Pools[int32(list.ID)] = append(r.Pools[int32(list.ID)], id)
		}
	}
	for key, row := range r.Rolls {
		if len(r.Pools[key.Grade]) == 0 {
			return nil, fmt.Errorf("missing source input emblem pool")
		}
		for col, prob := range row {
			if prob > 0 && len(r.Pools[int32(col+1)]) == 0 {
				return nil, fmt.Errorf("missing source reward emblem pool")
			}
		}
	}
	return r, nil
}
