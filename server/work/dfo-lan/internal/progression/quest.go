package progression

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"strings"
)

// QuestExperience uses current typed tables with the reference integer formula.
// Current difficulty keys include "10".."36"; do not truncate them to a rune.
func QuestExperience(c catalog.Progression, d catalog.QuestDefinition, level byte) (uint32, error) {
	cells := c.Scripts["n_quest/questparameter.etc"].Cells
	difficulty := section(d.Script.Cells, "[difficulty]")
	if len(difficulty) > 1 || len(difficulty) == 1 && difficulty[0].Type != 6 {
		return 0, fmt.Errorf("unsupported quest difficulty")
	}
	weights := section(cells, "[difficulty]")
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
	if len(section(d.Script.Cells, "[ignore level]")) > 0 {
		return 0, fmt.Errorf("quest ignore-level branch needs a separate rule")
	}
	rows := section(cells, "[exp reward table]")
	at := (int(d.MinimumLevel) - 1) * 2
	if level == 0 || at < 0 || at+1 >= len(rows) || rows[at].Type != 0 || rows[at].Value < 0 || rows[at+1].Type != 0 || rows[at+1].Value != -1 {
		return 0, fmt.Errorf("unsupported quest experience level row")
	}
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
	grade := section(d.Script.Cells, "[grade]")
	if len(grade) == 1 && grade[0].Text == "[epic]" {
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

// SelectedItemRewards keeps the original [job] exact profession/grow filter.
// A matching row must be awarded by the inventory owner, never silently lost.
func SelectedItemRewards(c []pvf.Token, job, grow byte) (bool, error) {
	items, e := ItemRewards(c, job, grow)
	return len(items) > 0, e
}

type QuestItemAward struct{ Template, Amount uint32 }

func ItemRewards(c []pvf.Token, job, grow byte) ([]QuestItemAward, error) {
	var items []QuestItemAward
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
			items = append(items, QuestItemAward{uint32(id), uint32(c[i].Value)})
		}
		i++
	}
	return items, nil
}
