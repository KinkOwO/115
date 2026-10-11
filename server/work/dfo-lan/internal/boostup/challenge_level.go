package boostup

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const ChallengeScriptPath = "live/event/kor/2026/0326_boostup/boostupspecupchallenge.evt"

// Level-up bonus is a separate mail from boostup.evt's reserved reward.
// Challenge unlock and repeated-clear rewards are not part of this table.
func ParseChallengeLevelRewards(cells []pvf.Token) (map[byte]Reward, error) {
	root, e := one(cells, "[level up bonus info]", "[/level up bonus info]")
	if e != nil {
		return nil, e
	}
	mail := false
	for _, t := range root {
		if t.Type == 3 && t.Text == "[mail]" {
			mail = true
		}
	}
	if !mail {
		return nil, fmt.Errorf("challenge level reward delivery is not mail")
	}
	block, e := one(root, "[level bonus]", "[/level bonus]")
	if e != nil {
		return nil, e
	}
	if len(block) == 0 || len(block)%3 != 0 {
		return nil, fmt.Errorf("invalid challenge level triples")
	}
	out := map[byte]Reward{}
	for i := 0; i < len(block); i += 3 {
		level, item, count := block[i], block[i+1], block[i+2]
		if level.Type != 0 || item.Type != 0 || count.Type != 0 || level.Value < 1 || level.Value > 255 || item.Value < 2 || count.Value < 1 {
			return nil, fmt.Errorf("invalid challenge level reward")
		}
		if _, ok := out[byte(level.Value)]; ok {
			return nil, fmt.Errorf("duplicate challenge level reward")
		}
		out[byte(level.Value)] = Reward{uint32(item.Value), uint32(count.Value)}
	}
	return out, nil
}

func (c *Catalog) CapsuleLevelMail(level byte) *GraduationMail {
	if c == nil || c.GoalLevel == 0 || level < c.GoalLevel {
		return nil
	}
	r, ok := c.ChallengeLevelRewards[c.GoalLevel]
	if !ok {
		return nil
	}
	// This source has no sender/body at this node; these are local UI labels.
	return &GraduationMail{Item: r.Item, Count: r.Count, Sender: "Starter Boost", Body: "Starter Boost level-up bonus."}
}
