package progression

import (
	"dfolan/internal/catalog"
	"fmt"
	"math"
	"strconv"
)

type ClearGain struct {
	Base, Score uint32
	Grade, Rank byte
}

// The reference consumes PVF's shortest round-trip float32 text as float64.
// Keep that conversion so1.3 does not become1.299999952 before floor().
func sourceDecimal(v float32) float64 {
	out, _ := strconv.ParseFloat(strconv.FormatFloat(float64(v), 'g', -1, 32), 64)
	return out
}

func DungeonClear(c catalog.Progression, d catalog.DungeonDefinition, difficulty, rank byte) (ClearGain, error) {
	out := ClearGain{Rank: rank}
	cells := section(c.Scripts["n_quest/questparameter.etc"].Cells, "[exp reward table]")
	at := (int(d.BasisLevel) - 1) * 2
	if at < 0 || at+1 >= len(cells) || cells[at].Type != 0 || cells[at].Value <= 0 || cells[at+1].Value != -1 || int(difficulty) >= len(c.DifficultyRates) {
		return out, fmt.Errorf("missing clear experience source row")
	}
	weight := float64(1)
	a := section(d.Script.Cells, "[experience increasing point]")
	if len(a) > 0 {
		if len(a) != 1 {
			return out, fmt.Errorf("ambiguous clear weight")
		}
		switch a[0].Type {
		case 0:
			weight = float64(a[0].Value)
		case 2:
			weight = sourceDecimal(a[0].Number)
		default:
			return out, fmt.Errorf("invalid clear weight")
		}
		if weight < 0 {
			weight = 1
		} // explicit compatibility90 absent/negative default
	}
	gain := math.Floor(float64(cells[at].Value) * weight * sourceDecimal(c.DifficultyRates[difficulty]))
	if math.IsNaN(gain) || math.IsInf(gain, 0) || gain <= 0 || gain > math.MaxUint32 {
		return out, fmt.Errorf("clear experience out of range")
	}
	out.Base = uint32(gain)
	index := 0
	for _, threshold := range c.RankThresholds {
		if rank >= threshold {
			if index == 0 {
				out.Grade = threshold
			}
			index++
		}
	}
	// Compatibility90 awards no score bonus when the table has no row for
	// this index. It must not wrap or invent a high-rank multiplier.
	if index > 0 && index <= len(c.ClearRankRates) {
		score := math.Floor(gain * sourceDecimal(c.ClearRankRates[index-1]))
		if math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score+gain > math.MaxUint32 {
			return out, fmt.Errorf("clear score experience overflow")
		}
		out.Score = uint32(score)
	}
	return out, nil
}
