package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// WorldDropLevel preserves the source's second column without assigning it a
// meaning. The user-selected reference policy ignores it; it is zero in the
// current archive. This table declares items independently of STK creation rate.
type WorldDropLevel struct {
	Column2 int32
	Items   []MonsterItemPair
}
type WorldDropTable struct {
	Path, SHA256 string
	Levels       map[uint32]WorldDropLevel
}

func ParseWorldDropTable(s ScriptRecord) (*WorldDropTable, error) {
	if s.Path != "etc/worlddrop.etc" || len(s.SHA256) != 64 {
		return nil, fmt.Errorf("world drop without source provenance")
	}
	t := &WorldDropTable{Path: s.Path, SHA256: s.SHA256, Levels: map[uint32]WorldDropLevel{}}
	c := s.Cells
	if len(c) < 2 || c[0].Type != 3 || c[0].Text != "[world drop]" {
		return nil, fmt.Errorf("world drop section missing")
	}
	i := 1
	for i < len(c) && c[i].Type == 0 {
		if i+1 >= len(c) || c[i+1].Type != 0 || c[i].Value < 1 || c[i].Value > 200 {
			return nil, fmt.Errorf("invalid world drop level at cell %d", i)
		}
		level := uint32(c[i].Value)
		if _, ok := t.Levels[level]; ok {
			return nil, fmt.Errorf("duplicate world drop level %d", level)
		}
		row := WorldDropLevel{Column2: c[i+1].Value}
		i += 2
		for {
			if i >= len(c) || c[i].Type != 0 {
				return nil, fmt.Errorf("unterminated world drop level %d", level)
			}
			if c[i].Value == -1 {
				i++
				break
			}
			if i+1 >= len(c) || c[i+1].Type != 0 {
				return nil, fmt.Errorf("invalid world drop pair at cell %d", i)
			}
			row.Items = append(row.Items, MonsterItemPair{Template: c[i].Value, Value: c[i+1].Value})
			i += 2
		}
		t.Levels[level] = row
	}
	if i != len(c)-1 || c[i].Type != 3 || c[i].Text != "[/world drop]" {
		return nil, fmt.Errorf("invalid world drop closing section")
	}
	return t, nil
}

// Re-read the live archive even when the ordinary catalog came from a derived
// cache. This runtime-only source never widens the generic creation-rate pool.
func (c *LootCatalog) EnableWorldDrop(a *pvf.Archive) error {
	if a == nil || a.Snapshot().Checksum != c.Source.Checksum {
		return fmt.Errorf("world drop source mismatch")
	}
	s, err := ResolveScript(a, "etc/worlddrop.etc")
	if err != nil {
		return err
	}
	c.WorldDrop, err = ParseWorldDropTable(s)
	return err
}
