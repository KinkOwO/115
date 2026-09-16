package progression

import (
	"dfolan/internal/catalog"
	"fmt"
	"math"
	"strings"
)

// difficultyWeight resolves a quest's [difficulty] letter to its weight in the
// questparameter table, folding case (81 quests spell the key lower while the
// table spells every key upper). A quest with no [difficulty] section carries
// no weight and settles on zero reward rather than an invented one - the same
// rule the experience path has used since live capture 20260912T004120.
func difficultyWeight(c catalog.Progression, d catalog.QuestDefinition) (uint64, error) {
	difficulty := section(d.Script.Cells, "[difficulty]")
	if len(difficulty) > 1 || len(difficulty) == 1 && difficulty[0].Type != 6 {
		return 0, fmt.Errorf("unsupported quest difficulty")
	}
	if len(difficulty) == 0 {
		return 0, nil
	}
	weights := section(c.Scripts["n_quest/questparameter.etc"].Cells, "[difficulty]")
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

// levelPenalty is the basic penalty, replaced by the green or grey band when
// the character is above the quest's own level. Epic quests use the epic bands.
func levelPenalty(c catalog.Progression, d catalog.QuestDefinition, level byte) (uint64, error) {
	cells := c.Scripts["n_quest/questparameter.etc"].Cells
	scalar := func(name string) (uint64, error) {
		a := section(cells, name)
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
	if g := section(d.Script.Cells, "[grade]"); len(g) == 1 && g[0].Text == "[epic]" {
		prefix = "epic "
	}
	for _, name := range []string{"green level penalty", "grey level penalty"} {
		a := section(cells, "["+prefix+name+"]")
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

// QuestGold is the completion gold for a quest, from [gold reward table] - the
// sibling of [exp reward table] in the same questparameter section. That table
// is one gold value per level (1..200), so a quest's base gold is the row at
// its own minimum level; the difficulty weight and the level penalty then scale
// it by the same integer formula the experience reward uses. A quest with no
// difficulty pays no gold, exactly as it earns no experience.
//
// The tables are computed together by the server-side reward routine (the
// client never sees this formula), so this mirrors the experience path rather
// than inventing a second one. It stays flagged for capture confirmation.
func QuestGold(c catalog.Progression, d catalog.QuestDefinition, level byte) (uint32, error) {
	weight, e := difficultyWeight(c, d)
	if e != nil {
		return 0, e
	}
	if weight == 0 {
		return 0, nil
	}
	if len(section(d.Script.Cells, "[ignore level]")) > 0 {
		return 0, fmt.Errorf("quest ignore-level branch needs a separate rule")
	}
	rows := section(c.Scripts["n_quest/questparameter.etc"].Cells, "[gold reward table]")
	at := int(d.MinimumLevel) - 1
	if level == 0 || at < 0 || at >= len(rows) || rows[at].Type != 0 || rows[at].Value < 0 {
		return 0, fmt.Errorf("unsupported quest gold level row")
	}
	penalty, e := levelPenalty(c, d, level)
	if e != nil {
		return 0, e
	}
	value := penalty * ((weight * uint64(rows[at].Value) / 100) / 100)
	if value > math.MaxUint32 {
		return 0, fmt.Errorf("quest gold exceeds 32-bit field")
	}
	return uint32(value), nil
}
