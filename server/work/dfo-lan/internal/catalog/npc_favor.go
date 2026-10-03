package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const FavorRulesPath = "etc/npcfavorsystem.etc"

// NPCFavorRules contains the source-defined gift costs, eligibility and point
// ranges. Server-specific daily-limit overrides are applied by the executor.
type NPCFavorRules struct {
	Source     string
	Definition ScriptRecord
	OpenLevel  byte
	GiftCount  uint32
	DailyLimit int
	Gifts      map[uint32][2]int64
	Levels     []int64
	Decay      []int64
}

func ImportNPCFavorRules(a *pvf.Archive) (*NPCFavorRules, error) {
	if a == nil {
		return nil, fmt.Errorf("favor rules require native PVF")
	}
	s, err := ReadScript(a, FavorRulesPath)
	if err != nil {
		return nil, err
	}
	return ParseNPCFavorRules(a.Snapshot().Checksum, s)
}

func ParseNPCFavorRules(source string, s ScriptRecord) (*NPCFavorRules, error) {
	out := &NPCFavorRules{Source: source, Definition: s, Gifts: map[uint32][2]int64{}}
	scalar := func(tag string) (int32, error) {
		var hits int
		for _, c := range s.Cells {
			if c.Type == 3 && c.Text == tag {
				hits++
			}
		}
		v := sectionCells(s.Cells, tag)
		if hits != 1 || len(v) != 1 || v[0].Type != 0 || v[0].Value <= 0 {
			return 0, fmt.Errorf("invalid favor scalar %s", tag)
		}
		return v[0].Value, nil
	}
	open, err := scalar("[favor condition level]")
	if err != nil {
		return nil, err
	}
	if open > 255 {
		return nil, fmt.Errorf("favor open level exceeds role field")
	}
	out.OpenLevel = byte(open)
	count, err := scalar("[favor gift item count]")
	if err != nil {
		return nil, err
	}
	out.GiftCount = uint32(count)
	limit, err := scalar("[favor gift limit]")
	if err != nil {
		return nil, err
	}
	out.DailyLimit = int(limit)
	rows := sectionCells(s.Cells, "[favor level point up]")
	if len(rows) == 0 || len(rows)%3 != 0 {
		return nil, fmt.Errorf("invalid favor gift point rows")
	}
	for i := 0; i < len(rows); i += 3 {
		a, b, c := rows[i], rows[i+1], rows[i+2]
		if a.Type != 0 || b.Type != 0 || c.Type != 0 || a.Value <= 0 || b.Value <= 0 || c.Value < b.Value {
			return nil, fmt.Errorf("invalid favor gift point range")
		}
		id := uint32(a.Value)
		if _, ok := out.Gifts[id]; ok {
			return nil, fmt.Errorf("duplicate favor gift %d", id)
		}
		out.Gifts[id] = [2]int64{int64(b.Value), int64(c.Value)}
	}
	rows = sectionCells(s.Cells, "[favor level point down]")
	if len(rows) == 0 || len(rows)%3 != 0 {
		return nil, fmt.Errorf("invalid favor level rows")
	}
	for i := 0; i < len(rows); i += 3 {
		a, b, c := rows[i], rows[i+1], rows[i+2]
		if a.Type != 0 || b.Type != 0 || c.Type != 0 || a.Value != int32(i/3) || b.Value < 0 || c.Value <= 0 || (len(out.Levels) > 0 && int64(c.Value) <= out.Levels[len(out.Levels)-1]) {
			return nil, fmt.Errorf("invalid favor level threshold")
		}
		out.Decay = append(out.Decay, int64(b.Value))
		out.Levels = append(out.Levels, int64(c.Value))
	}
	return out, nil
}

func (r *NPCFavorRules) PointRange(template uint32) (int64, int64) {
	if r == nil {
		return 0, 0
	}
	v := r.Gifts[template]
	return v[0], v[1]
}
func (r *NPCFavorRules) MaxPoint() int64 {
	if r == nil || len(r.Levels) == 0 {
		return 0
	}
	return r.Levels[len(r.Levels)-1]
}
