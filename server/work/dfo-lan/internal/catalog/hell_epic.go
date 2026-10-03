package catalog

import (
	"fmt"
	"math"
)

type HellEpicList struct {
	Area, ListType uint32
	Items          []DropWeight
	TotalWeight    uint32
}

type HellEpicTable struct {
	Path, SHA256 string
	Lists        []HellEpicList
}

// ParseHellEpicTable follows 115 reader 1473E9E00: each [drop area] contains
// lists keyed by (area, list type), with template/weight pairs accumulated in
// source order. This proves the pool shape, not the dungeon-to-area mapping or
// when an epic drop should be rolled.
func ParseHellEpicTable(s ScriptRecord) (HellEpicTable, error) {
	out := HellEpicTable{Path: s.Path, SHA256: s.SHA256}
	seen := map[[2]uint32]bool{}
	i := 0
	integer := func() (uint32, error) {
		if i >= len(s.Cells) || s.Cells[i].Type != 0 || s.Cells[i].Value < 0 {
			return 0, fmt.Errorf("%s: expected nonnegative integer at cell %d", s.Path, i)
		}
		v := uint32(s.Cells[i].Value)
		i++
		return v, nil
	}
	tag := func(name string) error {
		if i >= len(s.Cells) || s.Cells[i].Type != 3 || s.Cells[i].Text != name {
			return fmt.Errorf("%s: expected %s at cell %d", s.Path, name, i)
		}
		i++
		return nil
	}
	for i < len(s.Cells) {
		if err := tag("[drop area]"); err != nil {
			return out, err
		}
		area, err := integer()
		if err != nil {
			return out, err
		}
		for i < len(s.Cells) && s.Cells[i].Text != "[/drop area]" {
			if err := tag("[drop epic item list]"); err != nil {
				return out, err
			}
			kind, err := integer()
			if err != nil {
				return out, err
			}
			key := [2]uint32{area, kind}
			if seen[key] {
				return out, fmt.Errorf("%s: duplicate Hell epic area/list %v", s.Path, key)
			}
			seen[key] = true
			list := HellEpicList{Area: area, ListType: kind}
			for i < len(s.Cells) && s.Cells[i].Text != "[/drop epic item list]" {
				id, err := integer()
				if err != nil || id == 0 {
					return out, fmt.Errorf("%s: invalid Hell epic template at cell %d", s.Path, i)
				}
				weight, err := integer()
				if err != nil {
					return out, err
				}
				if uint64(list.TotalWeight)+uint64(weight) > math.MaxUint32 {
					return out, fmt.Errorf("%s: Hell epic weight overflow", s.Path)
				}
				list.TotalWeight += weight
				list.Items = append(list.Items, DropWeight{Template: id, Weight: weight})
			}
			if err := tag("[/drop epic item list]"); err != nil {
				return out, err
			}
			out.Lists = append(out.Lists, list)
		}
		if err := tag("[/drop area]"); err != nil {
			return out, err
		}
	}
	if len(out.Lists) == 0 {
		return out, fmt.Errorf("%s: Hell epic table is empty", s.Path)
	}
	return out, nil
}

// Kept separate from ordinary/Attunement pools; consumers must supply a proven
// area and list type rather than flattening every epic into one generic pool.
func (t HellEpicTable) List(area, kind uint32) (HellEpicList, bool) {
	for _, list := range t.Lists {
		if list.Area == area && list.ListType == kind {
			return list, true
		}
	}
	return HellEpicList{}, false
}
