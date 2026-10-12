package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const StackableDungeonLimitPath = "etc/stackabledungeonlimit.etc"

// Native14742be50 checks only [potion list]; 1473a1280 looks up the
// [dungeon list] count. Recovery names/effects do not define membership.
type StackableDungeonLimits struct {
	Path, SHA256    string
	Potions         map[uint32]bool
	Dungeons, Raids map[uint32]uint32
}

func ParseStackableDungeonLimits(s ScriptRecord) (*StackableDungeonLimits, error) {
	r := &StackableDungeonLimits{Path: s.Path, SHA256: s.SHA256, Potions: map[uint32]bool{}, Dungeons: map[uint32]uint32{}, Raids: map[uint32]uint32{}}
	for _, v := range sectionCells(s.Cells, "[potion list]") {
		if v.Type != 0 || v.Value <= 0 {
			return nil, fmt.Errorf("invalid limited potion source")
		}
		r.Potions[uint32(v.Value)] = true
	}
	for tag, target := range map[string]map[uint32]uint32{"[dungeon list]": r.Dungeons, "[raid list]": r.Raids} {
		vs := sectionCells(s.Cells, tag)
		if len(vs)%2 != 0 {
			return nil, fmt.Errorf("incomplete %s", tag)
		}
		for i := 0; i < len(vs); i += 2 {
			if vs[i].Type != 0 || vs[i].Value <= 0 || vs[i+1].Type != 0 || vs[i+1].Value < 0 {
				return nil, fmt.Errorf("invalid %s", tag)
			}
			id, count := uint32(vs[i].Value), uint32(vs[i+1].Value)
			if prior, dup := target[id]; dup && prior != count {
				return nil, fmt.Errorf("conflicting stackable limit")
			}
			target[id] = count
		}
	}
	if len(r.Potions) == 0 || len(r.Dungeons) == 0 {
		return nil, fmt.Errorf("empty potion limit source")
	}
	return r, nil
}

func importStackableDungeonLimits(a *pvf.Archive) (*StackableDungeonLimits, error) {
	s, err := ResolveScript(a, StackableDungeonLimitPath)
	if err != nil {
		return nil, err
	}
	return ParseStackableDungeonLimits(s)
}

// These are current native factory branches. Values, difficulty, weights
// and parameters remain source-owned; an unfamiliar kind is refused.
func IspinsOperationKindSupported(kind uint32) bool {
	switch kind {
	case 3, 6, 8, 11, 13, 22, 23:
		return true
	}
	return false
}
