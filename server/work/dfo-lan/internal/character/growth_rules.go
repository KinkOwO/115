package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// These configurable compatibility rules originate in the supplied90 server.
// They are not asserted to be the private official rules of the current DFO.
type GrowthRules struct {
	Model           string          `json:"model"`
	ReferenceSHA256 string          `json:"reference_sha256"`
	LevelCap        byte            `json:"level_cap"`
	NamedMultiplier float32         `json:"named_multiplier"`
	Penalty         map[int]float32 `json:"monster_minus_character_level_rates"`
	OutsidePenalty  float32         `json:"outside_level_range_rate"`
}

func LoadGrowthRules(path string) (GrowthRules, error) {
	var r GrowthRules
	b, e := os.ReadFile(path)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	if r.Model != "reference90-solo-v1" || len(r.ReferenceSHA256) != 64 || r.LevelCap < 2 || r.NamedMultiplier <= 0 || len(r.Penalty) == 0 {
		return r, fmt.Errorf("incomplete experience compatibility rules")
	}
	for _, v := range append([]float32{r.NamedMultiplier, r.OutsidePenalty}, growthMapValues(r.Penalty)...) {
		if !growthFinite(v) {
			return r, fmt.Errorf("invalid rule rate")
		}
	}
	return r, nil
}
func growthMapValues(m map[int]float32) []float32 {
	a := make([]float32, 0, len(m))
	for _, v := range m {
		a = append(a, v)
	}
	return a
}
func growthFinite(v float32) bool {
	return v >= 0 && !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0)
}
func growthSection(c []pvf.Token, name string) []pvf.Token {
	var r []pvf.Token
	on := false
	for _, v := range c {
		if v.Type == 3 {
			on = v.Text == name
			continue
		}
		if on {
			r = append(r, v)
		}
	}
	return r
}

// growthDifficultyIndex maps the session's native 1..5 difficulty to the five
// PVF experience columns. Native runs reporting 0 retain the first column,
// matching the admitted selection and existing ordinary reward boundary.
func growthDifficultyIndex(difficulty byte) (byte, error) {
	if difficulty > 5 {
		return 0, fmt.Errorf("unsupported experience difficulty %d", difficulty)
	}
	if difficulty == 0 {
		return 0, nil
	}
	return difficulty - 1, nil
}

// GrowthMonsterGain takes a zero-based PVF experience column, not a wire code.
func GrowthMonsterGain(c catalog.Progression, r GrowthRules, d catalog.DungeonDefinition, m protocol.DungeonMonster, level, difficulty byte) (uint64, error) {
	if m.NonCombat || m.APC || m.Level == 0 {
		return 0, nil
	}
	if level == 0 || m.Rank > 3 || int(difficulty) >= len(c.DifficultyRates) || len(c.MonsterRates) == 0 {
		return 0, fmt.Errorf("invalid monster experience source")
	}
	base, ok := c.MonsterExperience[uint16(m.Level)]
	if !ok {
		return 0, fmt.Errorf("missing monster level table")
	}
	weight := float32(1)
	if cells := growthSection(d.Script.Cells, "[experience increasing point]"); len(cells) > 0 {
		if len(cells) != 1 {
			return 0, fmt.Errorf("ambiguous dungeon experience weight")
		}
		switch cells[0].Type {
		case 0:
			weight = float32(cells[0].Value)
		case 2:
			weight = cells[0].Number
		default:
			return 0, fmt.Errorf("invalid dungeon weight")
		}
	}
	rank := 0
	if int(m.Rank) < len(c.MonsterRates) {
		rank = int(m.Rank)
	}
	rate := c.MonsterRates[rank]
	if m.Rank != 3 {
		for _, t := range growthSection(d.Script.Cells, "[named monster]") {
			if t.Type != 0 {
				return 0, fmt.Errorf("invalid named monster list")
			}
			if uint32(t.Value) == m.Template {
				rate *= r.NamedMultiplier
				break
			}
		}
	}
	penalty, ok := r.Penalty[int(m.Level)-int(level)]
	if !ok {
		penalty = r.OutsidePenalty
	}
	product := func(v float32, max float64) (uint64, error) {
		if !growthFinite(v) || float64(v) > max {
			return 0, fmt.Errorf("experience product out of range")
		}
		return uint64(v), nil
	}
	// Preserve reference C# float32 truncation at each documented boundary.
	v, e := product(float32(base)*weight, math.MaxInt32)
	if e != nil {
		return 0, e
	}
	v, e = product(float32(v)*c.DifficultyRates[difficulty]*rate, math.MaxInt32)
	if e != nil {
		return 0, e
	}
	return product(float32(v)*penalty, math.MaxUint32)
}

type GrowthAdvance struct {
	Level          byte
	Experience     uint64
	SkillPointGain uint32
}

func AddGrowthExperience(c catalog.Progression, r GrowthRules, level byte, total, gain uint64) (GrowthAdvance, error) {
	out := GrowthAdvance{Level: level, Experience: total}
	if level == 0 || level > r.LevelCap || int(r.LevelCap)-1 > len(c.Thresholds) || math.MaxUint64-total < gain {
		return out, fmt.Errorf("invalid experience ledger or cap")
	}
	if level > 1 && total < c.Thresholds[int(level)-2] {
		return out, fmt.Errorf("level exceeds cumulative experience")
	}
	out.Experience += gain
	for out.Level < r.LevelCap && out.Experience >= c.Thresholds[int(out.Level)-1] {
		out.Level++
		sp, ok := c.SkillPoints[uint16(out.Level)]
		if !ok {
			return GrowthAdvance{}, fmt.Errorf("missing crossed-level SP row")
		}
		out.SkillPointGain += uint32(sp)
	}
	return out, nil
}
