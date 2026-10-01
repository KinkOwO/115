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
	// [MERGE-20261001-ZERO-CLEAR-WEIGHT] 权重为 0 是源数据允许的取值，不是错误：
	// 奥德赛「大陆漂移」组（100004984..989）的脚本在 [experience increasing point]
	// 里就写着 0，该图的通关经验因此确实是 0。原来把 gain==0 当越界，CMD46 直接被
	// 拒（"clear experience out of range"），结算批次 34/37/35/261 一条都不发，客户端
	// 一直重试，玩家卡在副本里出不来（实机 2026-10-01：走进「前往天界」传送点，客户端
	// 本地播完传送动画后服务端仍认为人在副本内）。同一个权重在 MonsterGain 里一直是
	// 允许为 0 的（0 倍 → 该图怪物经验为 0，实机 100004984 进图与清图后的经验快照一致），
	// DungeonClear 不该对同一份源数据硬失败。真正越界的只有负值、NaN/Inf 与溢出。
	if math.IsNaN(gain) || math.IsInf(gain, 0) || gain < 0 || gain > math.MaxUint32 {
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
