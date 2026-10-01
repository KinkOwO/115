package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"path"
)

// Native 147CA96E0 groups consecutive probability rows by avatar [grade].
// Row count is the number of rewards, and its three columns are emblem
// grades 2,3,4 (native record offsets 8,12,16), rather than avatar rarity.
type AvatarDisjointRules struct {
	Source string
	Rolls  map[int32][][3]uint32
	Pools  map[int32][]uint32
}

func avatarDisjointSection(cells []pvf.Token, name string) []pvf.Token {
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

func ImportAvatarDisjointRules(a *pvf.Archive, index catalog.ItemIndex) (*AvatarDisjointRules, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("avatar disjoint source mismatch")
	}
	r := &AvatarDisjointRules{Source: a.Snapshot().Checksum, Rolls: map[int32][][3]uint32{}, Pools: map[int32][]uint32{}}
	s, e := catalog.ResolveScript(a, "etc/emblem/avatardisjoint.etc")
	if e != nil {
		return nil, e
	}
	rows := avatarDisjointSection(s.Cells, "[avatar disjoint info]")
	if len(rows) == 0 || len(rows)%4 != 0 {
		return nil, fmt.Errorf("invalid source avatar disjoint rows")
	}
	for i := 0; i < len(rows); i += 4 {
		for _, t := range rows[i : i+4] {
			if t.Type != 0 || t.Value < 0 {
				return nil, fmt.Errorf("invalid source avatar disjoint value")
			}
		}
		if rows[i].Value == 0 {
			return nil, fmt.Errorf("invalid source avatar grade")
		}
		v := [3]uint32{uint32(rows[i+1].Value), uint32(rows[i+2].Value), uint32(rows[i+3].Value)}
		if uint64(v[0])+uint64(v[1])+uint64(v[2]) != 100 {
			return nil, fmt.Errorf("invalid source avatar disjoint probability")
		}
		r.Rolls[rows[i].Value] = append(r.Rolls[rows[i].Value], v)
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
		if list.ID < 2 || list.ID > 4 {
			continue
		}
		s, e := catalog.ResolveScript(a, path.Join("etc/emblem", list.Path))
		if e != nil {
			return nil, e
		}
		// The common/0 group is the source's global random pool. The following
		// profession/grow-type groups belong to directed emblem operations.
		inGroup, common, inItems := false, false, false
		for i, t := range s.Cells {
			if t.Type == 3 {
				switch t.Text {
				case "[group]":
					inGroup, common, inItems = true, true, false
				case "[/group]":
					inGroup, inItems = false, false
				case "[grow type]":
					if i+2 >= len(s.Cells) || s.Cells[i+1].Type != 6 || s.Cells[i+2].Type != 0 {
						return nil, fmt.Errorf("invalid emblem group")
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
			if t.Type != 0 || t.Value <= 0 {
				return nil, fmt.Errorf("invalid source emblem template")
			}
			id := uint32(t.Value)
			item, ok := index.Items[id]
			if !ok || item.Kind != "stackable" || item.StackableType != "[avatar emblem]" {
				return nil, fmt.Errorf("source emblem %d absent or wrong type", id)
			}
			r.Pools[int32(list.ID)] = append(r.Pools[int32(list.ID)], id)
		}
		if len(r.Pools[int32(list.ID)]) == 0 {
			return nil, fmt.Errorf("empty source emblem pool %d", list.ID)
		}
	}
	for _, rows := range r.Rolls {
		for _, row := range rows {
			for col, prob := range row {
				if prob > 0 && len(r.Pools[int32(col+2)]) == 0 {
					return nil, fmt.Errorf("missing source emblem pool")
				}
			}
		}
	}
	return r, nil
}
