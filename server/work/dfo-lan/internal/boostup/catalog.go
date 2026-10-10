// Package boostup implements the source-defined Starter Boost event.
// Operational dates/eligibility are separate from the PVF's gameplay content.
package boostup

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const ScriptPath = "live/event/kor/2026/0326_boostup/boostup.evt"
const GiftPath = "event/eventgift.evt"
const EventID uint32 = 662

// EventStart/EventEnd 是 NOTI108 活动清单里 662/665/10017/10018 的窗口。
//
// 这两个数**逐字取自官服抓包**（`internal/legion/event_info_official.plain`，同一套
// 115 客户端）里同 id 的那几条记录：662 @0x117c、665 @0x16ad、2595、10017、10018
// 全部是 `start=1785801600 / end=179490599x`（10017/10018 的终点差几秒）。
//
// 订正（2026-10-06）：原来的注释写「源脚本不含活动上下线日期」，这是错的 ——
// `boostup.evt` 与 `boostupspecupchallenge.evt` 都带
// `[event period] 2026-08-04 09:00:00 / 2026-11-17 09:00:01`，也就是官服这几个
// epoch 对应的那一段日历（起点按 KST 折算是精确相等的；终点官服自己就提前了几秒到
// 几小时，没有可证的时间区规则）。所以这里不把日期搬进 PVF 直读：源里有定义但没有
// 可信的「脚本字符串 → wire epoch」折算证据，硬编一套时区规则反而成了第二条真源
// （§0.2 第 4 条：证据未闭环时不得用新常量补成规则）。上下线窗口按运维参数处理，
// 但**取值必须与官服一致**：原先起点写 0（"无限制"）是本树与官服那几条记录之间
// 剩下的唯一字段差异，2026-10-06「665 城里没有入口」的取证就是顺着这条差异查的。
// 换期只改这两行，不在协议层另铺一张表。
const (
	EventStart uint32 = 1785801600
	EventEnd   uint32 = 1794905999
)

type Reward struct{ Item, Count uint32 }
type Step struct {
	AutoOpen                 bool
	BlockEnchantBead         bool
	Number, Area             byte
	Guide, Mission           string
	Dungeon                  uint32
	RewardType               byte
	Rewards, BufferRewards   []Reward
	GuideCells, MissionCells []pvf.Token // source-specific conditions must not be discarded
}
type Gift struct {
	ID, Event  uint16
	Items      []uint32
	Direct     bool
	Trigger    string
	FirstPopup bool
	Links      []uint16
}
type PresetCode struct {
	Job, Grow byte
	Code      string
}
type Catalog struct {
	TeachingAPCs           []TeachingAPC // same-source special APC references, not account characters
	ChallengeBuffs         []ChallengeBuff
	Challenges             []ChallengeDefinition
	ChallengeLevelRewards  map[byte]Reward
	JournalDiscounts       []JournalDiscount
	ReservedMail           *GraduationMail
	Groups                 GroupLookup // selected-source membership; reused by equipment missions
	Capsules               map[uint32]Capsule
	GoalLevel, UsableLevel byte
	// QuestClearItems 来自 [capsule info] 里的 [quest clear item]：源为直升角色
	// 准备的清主线墙用券（当前 115 版是三张，各清一条 [grade] [side] 墙任务）。
	QuestClearItems      []uint32
	FameLimit            uint32
	Town, Area           uint32
	Steps                []Step
	Gifts                []Gift
	BufferJobs, DualJobs [][2]byte
	Presets              []PresetCode // duplicate job/grow alternatives remain distinct
}
type TokenSource interface {
	Tokens(string) ([]pvf.Token, error)
}

// Reuse the already-open runtime archive; never load a second full PVF here.
func Load(src TokenSource) (*Catalog, error) {
	cells, err := src.Tokens(ScriptPath)
	if err != nil {
		return nil, err
	}
	gifts, err := src.Tokens(GiftPath)
	if err != nil {
		return nil, err
	}
	c, err := Parse(cells, gifts)
	if err != nil {
		return nil, err
	}
	challenge, err := src.Tokens(ChallengeScriptPath)
	if err != nil {
		return nil, err
	}
	c.ChallengeLevelRewards, err = ParseChallengeLevelRewards(challenge)
	if err != nil {
		return nil, err
	}
	apcs, err := src.Tokens(SpecialAPCPath)
	if err != nil {
		return nil, err
	}
	c.TeachingAPCs, err = ParseTeachingAPCs(apcs)
	return c, err
}

// Event665 is a separate, optional post-graduation activity. Its content
// parsers and raid dependencies must never block event662's eleven lessons.
// The level115 mail above belongs to the captured graduation reward sequence
// even though its source entry is stored in the neighbouring event file.
func (c *Catalog) BindChallenges(src TokenSource) error {
	challenge, err := src.Tokens(ChallengeScriptPath)
	if err != nil {
		return err
	}
	rows, err := ParseChallenges(challenge)
	if err != nil {
		return err
	}
	buffs, err := src.Tokens(VariousBuffPath)
	if err != nil {
		return err
	}
	definitions, err := ParseChallengeBuffs(challenge, buffs)
	if err != nil {
		return err
	}
	c.Challenges, c.ChallengeBuffs = rows, definitions
	return nil
}

func values(c []pvf.Token, tag string) []pvf.Token {
	for i, t := range c {
		if t.Type == 3 && t.Text == tag {
			end := i + 1
			for end < len(c) && c[end].Type != 3 {
				end++
			}
			return c[i+1 : end]
		}
	}
	return nil
}
func number(c []pvf.Token, tag string, max uint32) (uint32, error) {
	v := values(c, tag)
	if len(v) != 1 || v[0].Type != 0 || v[0].Value < 0 || uint32(v[0].Value) > max {
		return 0, fmt.Errorf("invalid boost field %s", tag)
	}
	return uint32(v[0].Value), nil
}
func label(c []pvf.Token, tag string) string {
	v := values(c, tag)
	if len(v) == 1 && v[0].Type != 0 {
		return v[0].Text
	}
	return ""
}
func sections(c []pvf.Token, open, close string) ([][]pvf.Token, error) {
	var out [][]pvf.Token
	start := -1
	for i, t := range c {
		if t.Type != 3 {
			continue
		}
		if t.Text == open {
			if start >= 0 {
				return nil, fmt.Errorf("nested %s", open)
			}
			start = i + 1
		}
		if t.Text == close {
			if start < 0 {
				return nil, fmt.Errorf("orphan %s", close)
			}
			out = append(out, c[start:i])
			start = -1
		}
	}
	if start >= 0 {
		return nil, fmt.Errorf("unterminated %s", open)
	}
	return out, nil
}
func one(c []pvf.Token, open, close string) ([]pvf.Token, error) {
	v, e := sections(c, open, close)
	if e != nil {
		return nil, e
	}
	if len(v) != 1 {
		return nil, fmt.Errorf("expected one %s", open)
	}
	return v[0], nil
}
func rewards(c []pvf.Token, tag string) ([]Reward, error) {
	v := values(c, tag)
	if len(v)%2 != 0 {
		return nil, fmt.Errorf("odd reward pairs")
	}
	out := make([]Reward, 0, len(v)/2)
	for i := 0; i < len(v); i += 2 {
		if v[i].Type != 0 || v[i+1].Type != 0 || v[i].Value <= 0 || v[i+1].Value <= 0 {
			return nil, fmt.Errorf("invalid reward pair")
		}
		out = append(out, Reward{uint32(v[i].Value), uint32(v[i+1].Value)})
	}
	return out, nil
}
func jobs(c []pvf.Token, tag string) ([][2]byte, error) {
	v := values(c, tag)
	if len(v)%2 != 0 {
		return nil, fmt.Errorf("odd job pairs")
	}
	var out [][2]byte
	for i := 0; i < len(v); i += 2 {
		if v[i].Type != 0 || v[i+1].Type != 0 || v[i].Value < 0 || v[i].Value > 255 || v[i+1].Value < 0 || v[i+1].Value > 15 {
			return nil, fmt.Errorf("invalid job pair")
		}
		out = append(out, [2]byte{byte(v[i].Value), byte(v[i+1].Value)})
	}
	return out, nil
}
func Parse(cells, giftCells []pvf.Token) (*Catalog, error) {
	c := &Catalog{}
	var discountErr error
	c.JournalDiscounts, discountErr = parseJournalDiscounts(cells)
	if discountErr != nil {
		return nil, discountErr
	}
	var mailErr error
	c.ReservedMail, mailErr = parseGraduationMail(cells)
	if mailErr != nil {
		return nil, mailErr
	}
	goal, e := number(cells, "[goal level]", 255)
	if e != nil || goal == 0 {
		return nil, fmt.Errorf("invalid boost goal: %v", e)
	}
	c.GoalLevel = byte(goal)
	limit, e := number(cells, "[capsule usable level]", 255)
	if e != nil || limit == 0 {
		return nil, fmt.Errorf("invalid capsule level: %v", e)
	}
	c.UsableLevel = byte(limit)
	// [quest clear item] 在 [capsule info] 段里；本解析器按标签扁平取值（与
	// [goal level]、[level up table] 同一口径）。源缺失 ⇒ 不补券（不把缺失
	// 当成错误，免得整张 662 目录被一张券拖下线）；出现即必须全是正模板。
	c.QuestClearItems = nil
	for _, t := range values(cells, "[quest clear item]") {
		if t.Type != 0 || t.Value <= 0 {
			return nil, fmt.Errorf("invalid boost quest clear item")
		}
		c.QuestClearItems = append(c.QuestClearItems, uint32(t.Value))
	}
	c.FameLimit, e = number(cells, "[fame value limit]", ^uint32(0))
	if e != nil {
		return nil, e
	}
	pos := values(cells, "[event town area]")
	if len(pos) != 2 || pos[0].Type != 0 || pos[1].Type != 0 || pos[0].Value <= 0 || pos[1].Value < 0 {
		return nil, fmt.Errorf("invalid boost town")
	}
	c.Town, c.Area = uint32(pos[0].Value), uint32(pos[1].Value)
	c.BufferJobs, e = jobs(cells, "[buffer job and grow]")
	if e != nil {
		return nil, e
	}
	c.DualJobs, e = jobs(cells, "[dual job and grow]")
	if e != nil {
		return nil, e
	}
	steps, e := sections(cells, "[step info]", "[/step info]")
	if e != nil {
		return nil, e
	}
	if len(steps) == 0 || len(steps) > 254 {
		return nil, fmt.Errorf("invalid boost steps")
	}
	for i, b := range steps {
		n, e := number(b, "[no]", 254)
		if e != nil || n != uint32(i+1) {
			return nil, fmt.Errorf("noncontiguous boost step")
		}
		g, e := one(b, "[guide]", "[/guide]")
		if e != nil {
			return nil, e
		}
		m, e := one(b, "[mission]", "[/mission]")
		if e != nil {
			return nil, e
		}
		r, e := number(g, "[reward type]", 2)
		if e != nil || r == 0 {
			return nil, fmt.Errorf("invalid guide reward type")
		}
		a, e := number(b, "[area index]", 255)
		if e != nil {
			return nil, e
		}
		s := Step{Number: byte(n), Area: byte(a), Guide: label(g, "[type]"), Mission: label(m, "[type]"), RewardType: byte(r), GuideCells: append([]pvf.Token(nil), g...), MissionCells: append([]pvf.Token(nil), m...)}
		s.BlockEnchantBead, e = sourceStepFlag(b, "[block equipment enchanted bead]")
		if e != nil {
			return nil, e
		}
		for _, t := range g {
			if t.Type == 3 && t.Text == "[box open option]" {
				s.AutoOpen = true
			}
		}
		if s.Guide == "" || s.Mission == "" {
			return nil, fmt.Errorf("step missing guide/mission type")
		}
		if s.Guide == "dungeon" {
			s.Dungeon, e = number(g, "[dungeon index]", ^uint32(0))
			if e != nil || s.Dungeon == 0 {
				return nil, fmt.Errorf("invalid guide dungeon")
			}
		}
		s.Rewards, e = rewards(g, "[reward]")
		if e != nil || len(s.Rewards) == 0 {
			return nil, fmt.Errorf("step missing reward: %v", e)
		}
		s.BufferRewards, e = rewards(g, "[buffer reward]")
		if e != nil {
			return nil, e
		}
		c.Steps = append(c.Steps, s)
	}
	blocks, e := sections(giftCells, "[event gift]", "[/event gift]")
	if e != nil {
		return nil, e
	}
	seen := map[uint16]bool{}
	for _, b := range blocks {
		id, e := number(b, "[gift index]", 65535)
		if e != nil || id == 0 || seen[uint16(id)] {
			return nil, fmt.Errorf("invalid/duplicate gift")
		}
		seen[uint16(id)] = true
		event, e := number(b, "[event index]", 65535)
		if e != nil {
			return nil, e
		}
		g := Gift{ID: uint16(id), Event: uint16(event), Trigger: label(b, "[reward time]"), FirstPopup: true}
		for _, t := range values(b, "[gift item index]") {
			if t.Type != 0 || t.Value <= 0 {
				return nil, fmt.Errorf("invalid gift item")
			}
			g.Items = append(g.Items, uint32(t.Value))
		}
		if len(g.Items) == 0 {
			return nil, fmt.Errorf("empty gift")
		}
		for _, t := range b {
			if t.Type == 3 && t.Text == "[gift item recv to inven]" {
				g.Direct = true
			}
		}
		if v := values(b, "[first login open popup]"); len(v) != 0 {
			n, e := number(b, "[first login open popup]", 1)
			if e != nil {
				return nil, e
			}
			g.FirstPopup = n == 1
		}
		for _, t := range values(b, "[link gift index]") {
			if t.Type != 0 || t.Value <= 0 || t.Value > 65535 {
				return nil, fmt.Errorf("invalid gift link")
			}
			g.Links = append(g.Links, uint16(t.Value))
		}
		c.Gifts = append(c.Gifts, g)
	}
	blocks, e = sections(cells, "[skill code]", "[/skill code]")
	if e != nil {
		return nil, e
	}
	for _, b := range blocks {
		job, e := number(b, "[job]", 255)
		if e != nil {
			return nil, e
		}
		grow := -1
		for i, t := range b {
			if t.Type != 3 {
				continue
			}
			if t.Text == "[grow type]" {
				if i+1 >= len(b) || b[i+1].Type != 0 || b[i+1].Value < 0 || b[i+1].Value > 15 {
					return nil, fmt.Errorf("invalid preset grow")
				}
				grow = int(b[i+1].Value)
			}
			if t.Text == "[code]" {
				v := values(b[i:], "[code]")
				if grow < 0 || len(v) == 0 {
					return nil, fmt.Errorf("invalid preset code")
				}
				for _, code := range v {
					if code.Type != 6 || code.Text == "" {
						return nil, fmt.Errorf("invalid preset alternative")
					}
					c.Presets = append(c.Presets, PresetCode{byte(job), byte(grow), code.Text})
				}
			}
		}
	}
	return c, nil
}
