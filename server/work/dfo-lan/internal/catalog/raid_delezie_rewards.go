package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const delezieRewardPath = "contents/system/raidsystem/raidreward.etc"

// 14 is the native raid-kind enum, not a dungeon or reward content ID.
const delezieRewardKind = 14

type DelezieRewardRow struct {
	Row          int
	Kind         string
	Group        int32
	Weight       uint32
	Template     int32
	Grade, Extra int32
}

// Common rows have four fields; do not fabricate a group/extra field from the
// six-field STATE REWARD format. Their raid-wide selection remains separate.
type DelezieCommonRewardRow struct {
	Row      int
	Kind     string
	Weight   uint32
	Template int32
	Grade    int32
}

type DelezieModeRewards struct {
	Rewards []DelezieRewardRow
	Common  []DelezieCommonRewardRow
}

type DelezieRewardRules struct {
	Path                  string
	Kind                  uint32
	RepresentativeDungeon uint32
	Modes                 map[DelezieDifficulty]DelezieModeRewards
}

func delezieRewardInts(ts []pvf.Token, start, n int) ([]int32, error) {
	if start < 0 || start+n > len(ts) {
		return nil, fmt.Errorf("%s token %d: truncated reward", delezieRewardPath, start)
	}
	out := make([]int32, n)
	for i := range out {
		if ts[start+i].Type != 0 {
			return nil, fmt.Errorf("%s token %d: noninteger reward", delezieRewardPath, start+i)
		}
		out[i] = ts[start+i].Value
	}
	return out, nil
}

func validDelezieReward(weight, template, grade int32) bool {
	// The current native STATE REWARD reader (147756058..1477560BD)
	// accepts a nonzero item or the -1 empty prize, with grades 0/1/2/4.
	return weight > 0 && (template > 0 || template == -1) && (grade == 0 || grade == 1 || grade == 2 || grade == 4)
}

func parseDelezieRewards(ts []pvf.Token) (*DelezieRewardRules, error) {
	r := &DelezieRewardRules{Path: delezieRewardPath, Kind: delezieRewardKind, Modes: map[DelezieDifficulty]DelezieModeRewards{}}
	matched, seen, representative := false, false, false
	var mode *DelezieDifficulty
	clear, common := false, false
	fail := func(i int, why string) error { return fmt.Errorf("%s token %d: %s", r.Path, i, why) }
	for i := 0; i < len(ts); i++ {
		switch ts[i].Text {
		case "[RAID]":
			if matched {
				return nil, fail(i, "unclosed raid reward block")
			}
			v, err := delezieRewardInts(ts, i+1, 1)
			if err != nil {
				return nil, err
			}
			matched = v[0] == delezieRewardKind
			mode, clear, common = nil, false, false
			if matched {
				if seen {
					return nil, fail(i, "duplicate Delezie reward block")
				}
				seen = true
			}
			i++
		case "[/RAID]":
			if matched && (mode != nil || clear || common) {
				return nil, fail(i, "unclosed difficulty reward block")
			}
			matched = false
		default:
			if !matched {
				continue
			}
			switch ts[i].Text {
			case "[REPRESENTATIVE DUNGEON INDEX]":
				v, err := delezieRewardInts(ts, i+1, 1)
				if err != nil {
					return nil, err
				}
				if representative || v[0] <= 0 || mode != nil {
					return nil, fail(i, "invalid representative dungeon")
				}
				r.RepresentativeDungeon, representative = uint32(v[0]), true
				i++
			case "[PHASE DIFFICULTY]":
				if mode != nil || !representative || i+1 >= len(ts) || ts[i+1].Type != 6 {
					return nil, fail(i, "invalid reward difficulty")
				}
				d, err := delezieDifficulty(ts[i+1].Text)
				if err != nil {
					return nil, fail(i, err.Error())
				}
				if _, duplicate := r.Modes[d]; duplicate {
					return nil, fail(i, "duplicate reward difficulty")
				}
				r.Modes[d] = DelezieModeRewards{}
				mode = &d
				i++
			case "[/PHASE DIFFICULTY]":
				if mode == nil || clear || common || len(r.Modes[*mode].Rewards) == 0 {
					return nil, fail(i, "incomplete reward difficulty")
				}
				mode = nil
			case "[CLEAR REWARD ITEM]":
				if mode == nil || clear || common || len(r.Modes[*mode].Rewards) != 0 {
					return nil, fail(i, "invalid clear reward block")
				}
				clear = true
			case "[/CLEAR REWARD ITEM]":
				if !clear {
					return nil, fail(i, "orphan clear reward end")
				}
				clear = false
			case "[STATE REWARD]":
				if !clear || mode == nil || i+1 >= len(ts) || ts[i+1].Type != 6 || ts[i+1].Text == "" {
					return nil, fail(i, "reward outside clear block")
				}
				v, err := delezieRewardInts(ts, i+2, 5)
				if err != nil {
					return nil, err
				}
				if v[0] < -1 || v[0] > 4 || !validDelezieReward(v[1], v[2], v[3]) {
					return nil, fail(i, "invalid reward fields")
				}
				m := r.Modes[*mode]
				m.Rewards = append(m.Rewards, DelezieRewardRow{i, ts[i+1].Text, v[0], uint32(v[1]), v[2], v[3], v[4]})
				r.Modes[*mode] = m
				i += 6
			case "[SPECIAL COMMON REWARD]":
				if mode == nil || clear || common || len(r.Modes[*mode].Common) != 0 {
					return nil, fail(i, "invalid common reward block")
				}
				common = true
				for i++; i < len(ts) && ts[i].Text != "[/SPECIAL COMMON REWARD]"; i += 4 {
					if ts[i].Type != 6 || ts[i].Text == "" {
						return nil, fail(i, "invalid common reward kind")
					}
					v, err := delezieRewardInts(ts, i+1, 3)
					if err != nil {
						return nil, err
					}
					if !validDelezieReward(v[0], v[1], v[2]) {
						return nil, fail(i, "invalid common reward fields")
					}
					m := r.Modes[*mode]
					m.Common = append(m.Common, DelezieCommonRewardRow{i, ts[i].Text, uint32(v[0]), v[1], v[2]})
					r.Modes[*mode] = m
				}
				if i == len(ts) || len(r.Modes[*mode].Common) == 0 {
					return nil, fail(i, "unclosed/empty common rewards")
				}
				common = false
			default:
				return nil, fail(i, "unsupported Delezie reward field "+ts[i].Text)
			}
		}
	}
	if !seen || matched || !representative || len(r.Modes) == 0 {
		return nil, fail(len(ts), "missing/incomplete Delezie rewards")
	}
	return r, nil
}

func (r *DelezieRaidRules) RewardProgram(mode *DelezieModeRules) (DelezieModeRewards, error) {
	if r == nil || mode == nil || mode.Difficulty.Name() == "" || r.Modes[mode.Difficulty] != mode || r.Rewards == nil {
		return DelezieModeRewards{}, fmt.Errorf("Delezie reward source/mode missing")
	}
	m, ok := r.Rewards.Modes[mode.Difficulty]
	if !ok || len(m.Rewards) == 0 {
		return DelezieModeRewards{}, fmt.Errorf("%s: missing %s reward program", r.Rewards.Path, mode.Difficulty.Name())
	}
	m.Rewards = append([]DelezieRewardRow(nil), m.Rewards...)
	m.Common = append([]DelezieCommonRewardRow(nil), m.Common...)
	return m, nil
}

func (r *DelezieRaidRules) RewardLimits() (clears, rewards uint32, err error) {
	if r == nil || r.Header == nil || r.Header.Table == nil {
		return 0, 0, fmt.Errorf("Delezie reward limits source missing")
	}
	clears, err = ctpID(r.Header, -1, "[WEEKLY CLEAR COUNT]")
	if err == nil {
		rewards, err = ctpID(r.Header, -1, "[WEEKLY REWARD COUNT]")
	}
	return
}
