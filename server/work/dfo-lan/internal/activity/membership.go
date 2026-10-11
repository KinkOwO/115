package activity

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"fmt"
	"math"
	"time"
)

const MembershipPath = "live/else/chn/common/ceravipsystem/ceravipsystem.cos"

type MembershipGrade struct {
	Index            uint32
	Name             string
	Minimum, Maximum uint64
	Rewards          []Reward
}
type MembershipPayback struct {
	Dungeon uint32
	Grades  map[uint32]Reward
}
type MembershipRules struct {
	SourceChecksum, Table string
	ItemNames             map[uint32]string // presentation resolved from this archive's native item names
	Dates                 Dates
	Grades                []MembershipGrade
	RepeatPoints          uint64
	RepeatRewards         []Reward
	Paybacks              []MembershipPayback
}
type MembershipProgress struct {
	Version       uint32          `json:"version"`
	Spent         uint64          `json:"spent"`
	HighWater     int64           `json:"high_water"`
	Claimed       map[uint32]bool `json:"claimed,omitempty"`
	RepeatClaimed uint64          `json:"repeat_claimed"`
}

// ParseMembership accepts only the paired, explicitly adapted source grammar.
// In particular it assigns no native meaning to the CN final reward field 15.
func ParseMembership(t []pvf.Token) (MembershipRules, error) {
	var r MembershipRules
	contract, e := label(t, "[local extension controller]")
	if e != nil || contract != "membership-postcap-v1" {
		return r, fmt.Errorf("unsupported membership contract")
	}
	r.Table, e = label(t, "[local campaign]")
	if e != nil {
		return r, e
	}
	d, e := value(t, "[local dates]")
	if e != nil {
		return r, e
	}
	r.Dates, e = dates(d)
	if e != nil {
		return r, e
	}
	rows, e := section(t, "[grade info]")
	if e != nil {
		return r, e
	}
	for _, row := range rows {
		var g MembershipGrade
		g.Index, e = number(row, "[grade]")
		if e != nil {
			return r, e
		}
		g.Name, e = label(row, "[name]")
		if e != nil {
			return r, e
		}
		p, e := membershipRange(row)
		if e != nil {
			return r, e
		}
		g.Minimum, g.Maximum = p[0], p[1]
		g.Rewards, e = membershipRewards(row)
		if e != nil {
			return r, e
		}
		r.Grades = append(r.Grades, g)
	}
	repeat, e := one(t, "[repeat reward info]")
	if e != nil {
		return r, e
	}
	p, e := membershipRange(repeat)
	if e != nil || p[0] != 0 {
		return r, fmt.Errorf("invalid repeat range")
	}
	r.RepeatPoints = p[1]
	r.RepeatRewards, e = membershipRewards(repeat)
	if e != nil {
		return r, e
	}
	rows, e = section(t, "[payback]")
	if e != nil {
		return r, e
	}
	for _, row := range rows {
		b := MembershipPayback{Grades: map[uint32]Reward{}}
		b.Dungeon, e = number(row, "[dungeon index]")
		if e != nil {
			return r, e
		}
		benefits, e := section(row, "[payback info]")
		if e != nil {
			return r, e
		}
		for _, x := range benefits {
			grade, e := number(x, "[grade]")
			if e != nil {
				return r, e
			}
			if _, ok := b.Grades[grade]; ok {
				return r, fmt.Errorf("duplicate payback grade")
			}
			v, e := value(x, "[reward]")
			if e != nil || len(v) != 2 {
				return r, fmt.Errorf("invalid payback reward")
			}
			item, e := num(v[0])
			if e != nil {
				return r, e
			}
			count, e := num(v[1])
			if e != nil {
				return r, e
			}
			title, e := label(x, "[title]")
			if e != nil {
				return r, e
			}
			msg, e := label(x, "[explain]")
			if e != nil {
				return r, e
			}
			b.Grades[grade] = Reward{Item: item, Count: count, MailTitle: title, MailMessage: msg}
		}
		r.Paybacks = append(r.Paybacks, b)
	}
	return r, nil
}
func membershipRange(t []pvf.Token) ([2]uint64, error) {
	var p [2]uint64
	v, e := value(t, "[point range]")
	if e != nil || len(v) != 2 {
		return p, fmt.Errorf("invalid membership range")
	}
	for i := range p {
		n, e := num(v[i])
		if e != nil {
			return p, e
		}
		p[i] = uint64(n)
	}
	return p, nil
}
func membershipRewards(t []pvf.Token) ([]Reward, error) {
	v, e := one(t, "[reward item]")
	if e != nil || len(v) == 0 || len(v)%5 != 0 {
		return nil, fmt.Errorf("invalid membership bundle")
	}
	var out []Reward
	for i := 0; i < len(v); i += 5 {
		item, e := num(v[i])
		if e != nil {
			return nil, e
		}
		count, e := num(v[i+1])
		if e != nil {
			return nil, e
		}
		title, e := text(v[i+2])
		if e != nil {
			return nil, e
		}
		msg, e := text(v[i+3])
		if e != nil {
			return nil, e
		}
		permanent, e := num(v[i+4])
		if e != nil || permanent != 0 {
			return nil, fmt.Errorf("owned membership requires permanent reward")
		}
		out = append(out, Reward{Item: item, Count: count, MailTitle: title, MailMessage: msg})
	}
	return out, nil
}
func (r MembershipRules) CampaignKey() string {
	h := sha256.Sum256([]byte("membership-postcap-v1\x00" + r.Table + "\x00" + r.Dates.Start + "\x00" + r.Dates.End))
	return hex.EncodeToString(h[:])
}
func (r MembershipRules) Cap() uint64 {
	if len(r.Grades) == 0 {
		return 0
	}
	return r.Grades[len(r.Grades)-1].Minimum
}
func (r MembershipRules) Grade(spent uint64) uint32 {
	var grade uint32
	for _, g := range r.Grades {
		if spent >= g.Minimum {
			grade = g.Index
		}
	}
	return grade
}
func (r MembershipRules) Rewards() []Reward {
	var out []Reward
	for _, g := range r.Grades {
		out = append(out, g.Rewards...)
	}
	out = append(out, r.RepeatRewards...)
	for _, b := range r.Paybacks {
		for _, x := range b.Grades {
			out = append(out, x)
		}
	}
	return out
}
func (r MembershipRules) Validate() error {
	if len(r.SourceChecksum) != 64 || r.Table == "" || len(r.Grades) < 2 || len(r.Grades) > 32 || r.RepeatPoints == 0 || len(r.RepeatRewards) == 0 {
		return fmt.Errorf("incomplete membership source")
	}
	if _, e := hex.DecodeString(r.SourceChecksum); e != nil {
		return e
	}
	start, e := time.Parse("2006-01-02 15:04:05", r.Dates.Start)
	if e != nil {
		return e
	}
	end, e := time.Parse("2006-01-02 15:04:05", r.Dates.End)
	if e != nil || !end.After(start) {
		return fmt.Errorf("invalid membership dates")
	}
	for i, g := range r.Grades {
		if g.Index != uint32(i) || g.Name == "" || g.Minimum > g.Maximum || len(g.Rewards) == 0 || (i == 0 && g.Minimum != 0) || (i > 0 && g.Minimum != r.Grades[i-1].Maximum+1) {
			return fmt.Errorf("invalid membership grade ordering")
		}
	}
	if r.Cap() == 0 || r.Grades[len(r.Grades)-1].Maximum != r.Cap() {
		return fmt.Errorf("membership maximum grade must define the point cap")
	}
	seen := map[uint32]bool{}
	for _, b := range r.Paybacks {
		if b.Dungeon == 0 || seen[b.Dungeon] || len(b.Grades) == 0 {
			return fmt.Errorf("invalid membership payback")
		}
		seen[b.Dungeon] = true
		for g := range b.Grades {
			if g == 0 || int(g) >= len(r.Grades) {
				return fmt.Errorf("invalid payback grade")
			}
		}
	}
	for _, x := range r.Rewards() {
		if x.Item < 2 || x.Count == 0 || x.MailTitle == "" || x.MailMessage == "" {
			return fmt.Errorf("invalid membership grant")
		}
	}
	return nil
}
func (r MembershipRules) Active(now time.Time) bool {
	start, e := time.ParseInLocation("2006-01-02 15:04:05", r.Dates.Start, now.Location())
	if e != nil {
		return false
	}
	end, e := time.ParseInLocation("2006-01-02 15:04:05", r.Dates.End, now.Location())
	return e == nil && !now.Before(start) && now.Before(end)
}
func (r MembershipRules) RepeatAvailable(p MembershipProgress) uint64 {
	if p.Spent <= r.Cap() {
		return 0
	}
	return (p.Spent - r.Cap()) / r.RepeatPoints
}
func (r MembershipRules) ValidateProgress(p MembershipProgress) error {
	if p.Version > 1 || p.Spent > math.MaxInt64 || p.HighWater < 0 || (p.Version == 0 && (p.Spent != 0 || p.HighWater != 0 || len(p.Claimed) != 0 || p.RepeatClaimed != 0)) || p.RepeatClaimed > r.RepeatAvailable(p) {
		return fmt.Errorf("invalid membership save")
	}
	for g, claimed := range p.Claimed {
		if !claimed || int(g) >= len(r.Grades) || p.Spent < r.Grades[g].Minimum {
			return fmt.Errorf("invalid membership claim save")
		}
	}
	return nil
}
func (r MembershipRules) Observe(p MembershipProgress, now time.Time, charged uint64) (MembershipProgress, error) {
	if e := r.Validate(); e != nil {
		return p, e
	}
	if e := r.ValidateProgress(p); e != nil {
		return p, e
	}
	if now.UnixMilli() < p.HighWater {
		return p, fmt.Errorf("membership clock moved backwards")
	}
	if !r.Active(now) {
		return p, ErrRoutineInactive
	}
	if charged > math.MaxInt64-p.Spent {
		return p, fmt.Errorf("membership points overflow")
	}
	p.Version = 1
	p.HighWater = now.UnixMilli()
	p.Spent += charged
	cloned := map[uint32]bool{}
	for k, v := range p.Claimed {
		cloned[k] = v
	}
	p.Claimed = cloned
	return p, nil
}

// Group period selects a grade bundle, cumulative selects an ordered post-cap cycle.
func (r MembershipRules) Claim(p MembershipProgress, group string, index uint32) (MembershipProgress, []Reward, error) {
	if e := r.ValidateProgress(p); e != nil {
		return p, nil, e
	}
	if p.Version != 1 || index == 0 {
		return p, nil, fmt.Errorf("invalid membership claim")
	}
	if group == "period" {
		g := index - 1
		if int(g) >= len(r.Grades) || p.Spent < r.Grades[g].Minimum {
			return p, nil, fmt.Errorf("membership grade locked")
		}
		if p.Claimed[g] {
			return p, nil, nil
		}
		next := map[uint32]bool{}
		for k, v := range p.Claimed {
			next[k] = v
		}
		next[g] = true
		p.Claimed = next
		return p, r.Grades[g].Rewards, nil
	}
	if group == "cumulative" {
		if uint64(index) <= p.RepeatClaimed {
			return p, nil, nil
		}
		if uint64(index) != p.RepeatClaimed+1 || uint64(index) > r.RepeatAvailable(p) {
			return p, nil, fmt.Errorf("membership repeat locked")
		}
		p.RepeatClaimed++
		return p, r.RepeatRewards, nil
	}
	return p, nil, fmt.Errorf("unsupported membership claim")
}
