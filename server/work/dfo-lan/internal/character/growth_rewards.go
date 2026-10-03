package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// GrowthQuestExperience uses current typed tables with the reference integer formula.
// Current difficulty keys include "10".."36"; do not truncate them to a rune.
func GrowthQuestExperience(c catalog.Progression, d catalog.QuestDefinition, level byte) (uint32, error) {
	cells := c.Scripts["n_quest/questparameter.etc"].Cells
	difficulty := growthSection(d.Script.Cells, "[difficulty]")
	if len(difficulty) > 1 || len(difficulty) == 1 && difficulty[0].Type != 6 {
		return 0, fmt.Errorf("unsupported quest difficulty")
	}
	weights := growthSection(cells, "[difficulty]")
	var weight uint64
	// 347 of this build's 2844 quests carry no [difficulty] section at all
	// (pet evolution, guide and season quests among them). Refusing them made
	// every one impossible to hand in - live capture 20260912T004120 shows
	// quest 2109 refused three times in a row for exactly this. A quest that
	// declares no difficulty contributes no difficulty weight, so it settles
	// with no quest experience rather than with an invented one.
	found := len(difficulty) == 0
	matched := ""
	if len(weights)%2 != 0 {
		return 0, fmt.Errorf("invalid quest difficulty table")
	}
	for i := 0; i < len(weights); i += 2 {
		if weights[i].Type != 6 || weights[i+1].Type != 0 || weights[i+1].Value < 0 {
			return 0, fmt.Errorf("invalid quest difficulty row")
		}
		if len(difficulty) == 0 {
			continue
		}
		// The table spells its letter keys in upper case; 81 quests spell
		// theirs lower ("g" 55 times, "f" 25, "i" once). Each of those differs
		// from exactly one table key by case alone, so fold the comparison -
		// and refuse if the table ever grows a second key that only differs
		// by case, because then the intended row would be a guess.
		if !strings.EqualFold(weights[i].Text, difficulty[0].Text) {
			continue
		}
		if found && weights[i].Text != matched {
			return 0, fmt.Errorf("ambiguous quest difficulty key")
		}
		weight = uint64(weights[i+1].Value)
		matched = weights[i].Text
		found = true
	}
	if !found {
		return 0, fmt.Errorf("quest difficulty absent from source")
	}
	if len(growthSection(d.Script.Cells, "[ignore level]")) > 0 {
		return 0, fmt.Errorf("quest ignore-level branch needs a separate rule")
	}
	rows := growthSection(cells, "[exp reward table]")
	at := (int(d.MinimumLevel) - 1) * 2
	if level == 0 || at < 0 || at+1 >= len(rows) || rows[at].Type != 0 || rows[at].Value < 0 || rows[at+1].Type != 0 || rows[at+1].Value != -1 {
		return 0, fmt.Errorf("unsupported quest experience level row")
	}
	scalar := func(name string) (uint64, error) {
		a := growthSection(cells, name)
		if len(a) != 1 || a[0].Type != 0 || a[0].Value < 0 || a[0].Value > 100 {
			return 0, fmt.Errorf("invalid quest penalty scalar")
		}
		return uint64(a[0].Value), nil
	}
	penalty, e := scalar("[basic level penalty]")
	if e != nil {
		return 0, e
	}
	prefix := ""
	grade := growthSection(d.Script.Cells, "[grade]")
	if len(grade) == 1 && grade[0].Text == "[epic]" {
		prefix = "epic "
	}
	for _, name := range []string{"green level penalty", "grey level penalty"} {
		a := growthSection(cells, "["+prefix+name+"]")
		if len(a) != 3 {
			return 0, fmt.Errorf("current quest penalty requires lower/upper/rate")
		}
		for _, v := range a {
			if v.Type != 0 {
				return 0, fmt.Errorf("invalid quest level penalty token")
			}
		}
		if a[0].Value > a[1].Value || a[2].Value < 0 || a[2].Value > 100 {
			return 0, fmt.Errorf("invalid quest penalty range")
		}
		delta := int32(level) - int32(d.MinimumLevel)
		if delta >= a[0].Value && delta <= a[1].Value {
			penalty = uint64(a[2].Value)
		}
	}
	value := penalty * ((weight * uint64(rows[at].Value) / 100) / 100)
	if value > math.MaxUint32 {
		return 0, fmt.Errorf("quest gain exceeds current32-bit gain field")
	}
	return uint32(value), nil
}

// GrowthSelectedItemRewards keeps the original [job] exact profession/grow filter.
// A matching row must be awarded by the inventory owner, never silently lost.
func GrowthSelectedItemRewards(c []pvf.Token, job, grow byte) (bool, error) {
	items, e := GrowthItemRewards(c, job, grow)
	return len(items) > 0, e
}

type GrowthQuestItemAward struct{ Template, Amount uint32 }

func GrowthItemRewards(c []pvf.Token, job, grow byte) ([]GrowthQuestItemAward, error) {
	var items []GrowthQuestItemAward
	for i := 0; i < len(c); {
		if c[i].Type != 0 || c[i].Value < 0 {
			return nil, fmt.Errorf("invalid quest reward item")
		}
		id := c[i].Value
		i++
		applies := true
		if i < len(c) && c[i].Type == 6 && c[i].Text == "[job]" {
			if i+3 >= len(c) || c[i+1].Type != 0 || c[i+2].Type != 0 || c[i+3].Type != 0 {
				return nil, fmt.Errorf("incomplete quest reward job tuple")
			}
			applies = c[i+1].Value == int32(job) && (c[i+2].Value < 0 || c[i+2].Value == int32(grow&15))
			i += 3
		}
		if i >= len(c) || c[i].Type != 0 || c[i].Value < 0 || (id != 0 && c[i].Value == 0) {
			return nil, fmt.Errorf("invalid quest reward count")
		}
		// id0 is currency, still a real reward requiring its own transaction owner.
		if applies && c[i].Value > 0 {
			items = append(items, GrowthQuestItemAward{uint32(id), uint32(c[i].Value)})
		}
		i++
	}
	return items, nil
}

// growthDifficultyWeight resolves a quest's [difficulty] letter to its weight in the
// questparameter table, folding case (81 quests spell the key lower while the
// table spells every key upper). A quest with no [difficulty] section carries
// no weight and settles on zero reward rather than an invented one - the same
// rule the experience path has used since live capture 20260912T004120.
func growthDifficultyWeight(c catalog.Progression, d catalog.QuestDefinition) (uint64, error) {
	difficulty := growthSection(d.Script.Cells, "[difficulty]")
	if len(difficulty) > 1 || len(difficulty) == 1 && difficulty[0].Type != 6 {
		return 0, fmt.Errorf("unsupported quest difficulty")
	}
	if len(difficulty) == 0 {
		return 0, nil
	}
	weights := growthSection(c.Scripts["n_quest/questparameter.etc"].Cells, "[difficulty]")
	if len(weights)%2 != 0 {
		return 0, fmt.Errorf("invalid quest difficulty table")
	}
	var weight uint64
	matched := ""
	for i := 0; i < len(weights); i += 2 {
		if weights[i].Type != 6 || weights[i+1].Type != 0 || weights[i+1].Value < 0 {
			return 0, fmt.Errorf("invalid quest difficulty row")
		}
		if !strings.EqualFold(weights[i].Text, difficulty[0].Text) {
			continue
		}
		if matched != "" && weights[i].Text != matched {
			return 0, fmt.Errorf("ambiguous quest difficulty key")
		}
		weight, matched = uint64(weights[i+1].Value), weights[i].Text
	}
	if matched == "" {
		return 0, fmt.Errorf("quest difficulty absent from source")
	}
	return weight, nil
}

// growthLevelPenalty is the basic penalty, replaced by the green or grey band when
// the character is above the quest's own level. Epic quests use the epic bands.
func growthLevelPenalty(c catalog.Progression, d catalog.QuestDefinition, level byte) (uint64, error) {
	cells := c.Scripts["n_quest/questparameter.etc"].Cells
	scalar := func(name string) (uint64, error) {
		a := growthSection(cells, name)
		if len(a) != 1 || a[0].Type != 0 || a[0].Value < 0 || a[0].Value > 100 {
			return 0, fmt.Errorf("invalid quest penalty scalar")
		}
		return uint64(a[0].Value), nil
	}
	penalty, e := scalar("[basic level penalty]")
	if e != nil {
		return 0, e
	}
	prefix := ""
	if g := growthSection(d.Script.Cells, "[grade]"); len(g) == 1 && g[0].Text == "[epic]" {
		prefix = "epic "
	}
	for _, name := range []string{"green level penalty", "grey level penalty"} {
		a := growthSection(cells, "["+prefix+name+"]")
		if len(a) != 3 {
			return 0, fmt.Errorf("current quest penalty requires lower/upper/rate")
		}
		for _, v := range a {
			if v.Type != 0 {
				return 0, fmt.Errorf("invalid quest level penalty token")
			}
		}
		if a[0].Value > a[1].Value || a[2].Value < 0 || a[2].Value > 100 {
			return 0, fmt.Errorf("invalid quest penalty range")
		}
		if delta := int32(level) - int32(d.MinimumLevel); delta >= a[0].Value && delta <= a[1].Value {
			penalty = uint64(a[2].Value)
		}
	}
	return penalty, nil
}

// GrowthQuestGold is the completion gold for a quest, from [gold reward table] - the
// sibling of [exp reward table] in the same questparameter section. That table
// is one gold value per level (1..200), so a quest's base gold is the row at
// its own minimum level; the difficulty weight and the level penalty then scale
// it by the same integer formula the experience reward uses. A quest with no
// difficulty pays no gold, exactly as it earns no experience.
//
// The tables are computed together by the server-side reward routine (the
// client never sees this formula), so this mirrors the experience path rather
// than inventing a second one. It stays flagged for capture confirmation.
func GrowthQuestGold(c catalog.Progression, d catalog.QuestDefinition, level byte) (uint32, error) {
	weight, e := growthDifficultyWeight(c, d)
	if e != nil {
		return 0, e
	}
	if weight == 0 {
		return 0, nil
	}
	if len(growthSection(d.Script.Cells, "[ignore level]")) > 0 {
		return 0, fmt.Errorf("quest ignore-level branch needs a separate rule")
	}
	rows := growthSection(c.Scripts["n_quest/questparameter.etc"].Cells, "[gold reward table]")
	at := int(d.MinimumLevel) - 1
	if level == 0 || at < 0 || at >= len(rows) || rows[at].Type != 0 || rows[at].Value < 0 {
		return 0, fmt.Errorf("unsupported quest gold level row")
	}
	penalty, e := growthLevelPenalty(c, d, level)
	if e != nil {
		return 0, e
	}
	value := penalty * ((weight * uint64(rows[at].Value) / 100) / 100)
	if value > math.MaxUint32 {
		return 0, fmt.Errorf("quest gold exceeds 32-bit field")
	}
	return uint32(value), nil
}

type GrowthClearGain struct {
	Base, Score uint32
	Grade, Rank byte
}

// The reference consumes PVF's shortest round-trip float32 text as float64.
// Keep that conversion so1.3 does not become1.299999952 before floor().
func growthSourceDecimal(v float32) float64 {
	out, _ := strconv.ParseFloat(strconv.FormatFloat(float64(v), 'g', -1, 32), 64)
	return out
}

// GrowthDungeonClear takes a zero-based PVF experience column, not a wire code.
func GrowthDungeonClear(c catalog.Progression, d catalog.DungeonDefinition, difficulty, rank byte) (GrowthClearGain, error) {
	out := GrowthClearGain{Rank: rank}
	cells := growthSection(c.Scripts["n_quest/questparameter.etc"].Cells, "[exp reward table]")
	at := (int(d.BasisLevel) - 1) * 2
	if at < 0 || at+1 >= len(cells) || cells[at].Type != 0 || cells[at].Value <= 0 || cells[at+1].Value != -1 || int(difficulty) >= len(c.DifficultyRates) {
		return out, fmt.Errorf("missing clear experience source row")
	}
	weight := float64(1)
	a := growthSection(d.Script.Cells, "[experience increasing point]")
	if len(a) > 0 {
		if len(a) != 1 {
			return out, fmt.Errorf("ambiguous clear weight")
		}
		switch a[0].Type {
		case 0:
			weight = float64(a[0].Value)
		case 2:
			weight = growthSourceDecimal(a[0].Number)
		default:
			return out, fmt.Errorf("invalid clear weight")
		}
		if weight < 0 {
			weight = 1
		} // explicit compatibility90 absent/negative default
	}
	gain := math.Floor(float64(cells[at].Value) * weight * growthSourceDecimal(c.DifficultyRates[difficulty]))
	// [MERGE-20261001-ZERO-CLEAR-WEIGHT] 权重为 0 是源数据允许的取值，不是错误：
	// 奥德赛「大陆漂移」组（100004984..989）的脚本在 [experience increasing point]
	// 里就写着 0，该图的通关经验因此确实是 0。原来把 gain==0 当越界，CMD46 直接被
	// 拒（"clear experience out of range"），结算批次 34/37/35/261 一条都不发，客户端
	// 一直重试，玩家卡在副本里出不来（实机 2026-10-01：走进「前往天界」传送点，客户端
	// 本地播完传送动画后服务端仍认为人在副本内）。同一个权重在 GrowthMonsterGain 里一直是
	// 允许为 0 的（0 倍 → 该图怪物经验为 0，实机 100004984 进图与清图后的经验快照一致），
	// GrowthDungeonClear 不该对同一份源数据硬失败。真正越界的只有负值、NaN/Inf 与溢出。
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
		score := math.Floor(gain * growthSourceDecimal(c.ClearRankRates[index-1]))
		if math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score+gain > math.MaxUint32 {
			return out, fmt.Errorf("clear score experience overflow")
		}
		out.Score = uint32(score)
	}
	return out, nil
}
