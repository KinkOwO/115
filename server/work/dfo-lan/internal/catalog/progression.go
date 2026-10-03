package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type Progression struct {
	Source            pvf.ArchiveSnapshot     `json:"source"`
	Scripts           map[string]ScriptRecord `json:"scripts"`
	Thresholds        []uint64                `json:"cumulative_experience_thresholds"`
	SkillPoints       map[uint16]uint16       `json:"skill_points_by_level"`
	MonsterExperience map[uint16]uint64       `json:"monster_experience_by_level"`
	MonsterRates      []float32               `json:"monster_rates"`
	DifficultyRates   []float32               `json:"difficulty_rates"`
	ClearRankRates    []float32               `json:"clear_rank_rates"`
	RankThresholds    []byte                  `json:"rank_thresholds"`
}

// Current loader14709d250 consumes type9 pairs in high/low order through
// 147c99070. This differs from the reference90 unsigned32 threshold reader.
func ExperienceThresholds(c []pvf.Token) ([]uint64, error) {
	var values []uint64
	for i := 0; i < len(c); i++ {
		var v uint64
		switch c[i].Type {
		case 0:
			if c[i].Value < 0 {
				return nil, fmt.Errorf("negative32-bit experience threshold")
			}
			v = uint64(c[i].Value)
		case 9:
			if i+1 >= len(c) || c[i+1].Type != 9 {
				return nil, fmt.Errorf("unpaired64-bit experience threshold")
			}
			v = uint64(uint32(c[i].Value))<<32 | uint64(uint32(c[i+1].Value))
			i++
		default:
			return nil, fmt.Errorf("unsupported experience token %d", c[i].Type)
		}
		if v == 0 || len(values) > 0 && v <= values[len(values)-1] {
			return nil, fmt.Errorf("experience thresholds are not strictly increasing")
		}
		values = append(values, v)
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("empty experience table")
	}
	return values, nil
}

func ImportProgression(a *pvf.Archive) (Progression, error) {
	p := Progression{Source: a.Snapshot(), Scripts: map[string]ScriptRecord{}}
	for _, name := range []string{"character/exptable.tbl", "monster/monsterexp.tbl", "etc/sptable.etc", "n_quest/questparameter.etc", "etc/serverparameter.etc", "etc/ranksysteminfo.etc"} {
		s, e := ResolveScript(a, name)
		if e != nil {
			return p, e
		}
		p.Scripts[name] = s
	}
	return ParseProgression(p)
}

func ParseProgression(p Progression) (Progression, error) {
	var e error
	p.Thresholds, e = ExperienceThresholds(p.Scripts["character/exptable.tbl"].Cells)
	if e != nil {
		return p, e
	}
	parsePairs := func(c []pvf.Token) (map[uint16]uint64, error) {
		if len(c) == 0 || len(c)%2 != 0 {
			return nil, fmt.Errorf("unpaired progression rows")
		}
		m := map[uint16]uint64{}
		for i := 0; i < len(c); i += 2 {
			a, b := c[i], c[i+1]
			if a.Type != 0 || b.Type != 0 || a.Value <= 0 || a.Value > 65535 || b.Value < 0 {
				return nil, fmt.Errorf("invalid progression row")
			}
			id := uint16(a.Value)
			if _, ok := m[id]; ok {
				return nil, fmt.Errorf("duplicate progression level")
			}
			m[id] = uint64(b.Value)
		}
		return m, nil
	}
	p.MonsterExperience, e = parsePairs(p.Scripts["monster/monsterexp.tbl"].Cells)
	if e != nil {
		return p, e
	}
	sp, e := parsePairs(sectionCells(p.Scripts["etc/sptable.etc"].Cells, "[sp table]"))
	if e != nil {
		return p, e
	}
	p.SkillPoints = map[uint16]uint16{}
	for l, v := range sp {
		if v > 65535 {
			return p, fmt.Errorf("SP overflow")
		}
		p.SkillPoints[l] = uint16(v)
	}
	rates := func(name string) ([]float32, error) {
		c := sectionCells(p.Scripts["etc/serverparameter.etc"].Cells, name)
		var out []float32
		for _, t := range c {
			var v float32
			switch t.Type {
			case 0:
				v = float32(t.Value)
			case 2:
				v = t.Number
			default:
				return nil, fmt.Errorf("unsupported rate type")
			}
			if v < 0 || math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, fmt.Errorf("invalid rate")
			}
			out = append(out, v)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("missing %s", name)
		}
		return out, nil
	}
	p.MonsterRates, e = rates("[monster exp bonusrate]")
	if e != nil {
		return p, e
	}
	p.DifficultyRates, e = rates("[dungeon difficulty exp bonusrate]")
	if e != nil {
		return p, e
	}
	p.ClearRankRates, e = rates("[clear rank exp bonusrate]")
	if e != nil {
		return p, e
	}
	p.RankThresholds = nil
	previous := int32(256)
	for _, c := range sectionCells(p.Scripts["etc/ranksysteminfo.etc"].Cells, "[rank grade]") {
		if c.Type != 0 || c.Value < 0 || c.Value >= previous {
			return p, fmt.Errorf("invalid source rank thresholds")
		}
		p.RankThresholds = append(p.RankThresholds, byte(c.Value))
		previous = c.Value
	}
	if len(p.RankThresholds) == 0 || len(p.ClearRankRates) == 0 {
		return p, fmt.Errorf("incomplete rank tables")
	}
	return p, nil
}

func LoadProgression(path string) (Progression, error) {
	// LoadProgression reads an explicit historical baseline for offline audits
	// and tests. Runtime callers must use a prepared gamedata Source.
	var p Progression
	b, e := os.ReadFile(path)
	if e != nil {
		return p, e
	}
	if e = json.Unmarshal(b, &p); e != nil {
		return p, e
	}
	// Regenerate projections from typed sources rather than trusting edits to a
	// derived numeric field. Gameplay overrides belong in a separate rule file.
	return ParseProgression(p)
}
