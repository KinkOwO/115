package boostup

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const ChallengeEventID uint32 = 665

type ChallengeDefinition struct {
	Index                       byte
	UnlockKind                  string
	UnlockValue                 uint32
	Kind                        string
	Conditions                  []uint32
	Goal, Repeat                uint32
	UnlockRewards, ClearRewards []Reward
	UnlockMail, ClearMail       bool
	GuideDungeon                uint32
	// GoTarget is the source `[go contents town area]` landing of this row's Go
	// button (town, area, x, y); HasGoTarget is false when the row has none.
	GoTarget    [4]uint32
	HasGoTarget bool
}
type ChallengeProgress struct {
	Unlocked      bool   `json:"unlocked"`
	UnlockClaimed bool   `json:"unlock_claimed"`
	Progress      uint32 `json:"progress"`
	Claims        uint32 `json:"claims"`
}
type ChallengeState struct {
	Version  int                        `json:"version"`
	Enrolled bool                       `json:"enrolled"`
	Rows     map[byte]ChallengeProgress `json:"rows"`
}

func ParseChallenges(cells []pvf.Token) ([]ChallengeDefinition, error) {
	root, e := one(cells, "[challenge info]", "[/challenge info]")
	if e != nil {
		return nil, e
	}
	rows, e := sections(root, "[info]", "[/info]")
	if e != nil {
		return nil, e
	}
	if len(rows) == 0 || len(rows) > 32 {
		return nil, fmt.Errorf("invalid challenge row count")
	}
	var out []ChallengeDefinition
	seen := map[byte]bool{}
	for _, row := range rows {
		id, e := number(row, "[no]", 31)
		if e != nil || seen[byte(id)] {
			return nil, fmt.Errorf("invalid/duplicate challenge index")
		}
		seen[byte(id)] = true
		u := values(row, "[unlock type]")
		if len(u) != 2 || u[0].Type == 0 || u[1].Type != 0 || u[1].Value <= 0 {
			return nil, fmt.Errorf("invalid challenge unlock")
		}
		if u[0].Text != "level" && u[0].Text != "fame" {
			return nil, fmt.Errorf("unsupported challenge unlock %s", u[0].Text)
		}
		d := ChallengeDefinition{Index: byte(id), UnlockKind: u[0].Text, UnlockValue: uint32(u[1].Value), Kind: label(row, "[challenge type]")}
		d.Goal, e = number(row, "[challenge goal]", ^uint32(0))
		if e != nil || d.Goal == 0 {
			return nil, fmt.Errorf("invalid challenge goal")
		}
		d.Repeat, e = number(row, "[challenge repeat count]", ^uint32(0))
		if e != nil || d.Repeat == 0 || uint64(d.Goal)*uint64(d.Repeat) > uint64(^uint32(0)) {
			return nil, fmt.Errorf("invalid challenge repeat")
		}
		switch d.Kind {
		case "clear endkeeper of order", "clear higher or legion", "clear raid":
		default:
			return nil, fmt.Errorf("unsupported challenge type %s", d.Kind)
		}
		for _, v := range values(row, "[challenge condition]") {
			if v.Type != 0 || v.Value <= 0 {
				return nil, fmt.Errorf("invalid challenge content condition")
			}
			d.Conditions = append(d.Conditions, uint32(v.Value))
		}
		if d.Kind != "clear endkeeper of order" && len(d.Conditions) == 0 {
			return nil, fmt.Errorf("challenge content IDs missing")
		}
		d.UnlockRewards, e = rewards(row, "[unlock reward]")
		if e != nil || len(d.UnlockRewards) == 0 {
			return nil, fmt.Errorf("challenge unlock rewards missing")
		}
		d.ClearRewards, e = rewards(row, "[challenge reward]")
		if e != nil || len(d.ClearRewards) == 0 {
			return nil, fmt.Errorf("challenge rewards missing")
		}
		for _, v := range row {
			if v.Type == 3 {
				switch v.Text {
				case "[unlock mail]":
					d.UnlockMail = true
				case "[challenge mail]":
					d.ClearMail = true
				}
			}
		}
		d.GuideDungeon, e = number(row, "[go contents dungeon index]", ^uint32(0))
		if e != nil {
			return nil, e
		}
		if v := values(row, "[go contents town area]"); len(v) == 4 {
			for _, t := range v {
				if t.Type != 0 || t.Value < 0 {
					return nil, fmt.Errorf("invalid challenge go target")
				}
			}
			if v[2].Value <= 65535 && v[3].Value <= 65535 {
				d.GoTarget = [4]uint32{uint32(v[0].Value), uint32(v[1].Value), uint32(v[2].Value), uint32(v[3].Value)}
				d.HasGoTarget = true
			}
		}
		out = append(out, d)
	}
	return out, nil
}

func (c *Catalog) Challenge(index byte) (ChallengeDefinition, error) {
	if c != nil {
		for _, d := range c.Challenges {
			if d.Index == index {
				return d, nil
			}
		}
	}
	return ChallengeDefinition{}, fmt.Errorf("challenge index not in source")
}

func (s *ChallengeState) clone() *ChallengeState {
	if s == nil {
		return &ChallengeState{Version: 1, Rows: map[byte]ChallengeProgress{}}
	}
	n := *s
	n.Rows = map[byte]ChallengeProgress{}
	for k, v := range s.Rows {
		n.Rows[k] = v
	}
	return &n
}

func (c *Catalog) ValidateChallengeState(s *ChallengeState) error {
	if s == nil {
		return nil
	}
	if s.Version != 1 {
		return fmt.Errorf("unknown challenge storage version")
	}
	for index, p := range s.Rows {
		d, e := c.Challenge(index)
		if e != nil {
			return e
		}
		if (!s.Enrolled || !p.Unlocked) && (p.Unlocked || p.UnlockClaimed || p.Progress != 0 || p.Claims != 0) || p.Claims > d.Repeat || uint64(p.Progress) > uint64(d.Goal)*uint64(d.Repeat) || uint64(p.Claims)*uint64(d.Goal) > uint64(p.Progress) {
			return fmt.Errorf("inconsistent challenge progress")
		}
	}
	return nil
}

// Sticky eligibility, never an award or a clear proof. Call with server-owned
// level/fame inside the character transaction, after proven training graduation.
func (c *Catalog) UnlockChallenges(old *ChallengeState, level byte, fame uint32) (*ChallengeState, bool, error) {
	if e := c.ValidateChallengeState(old); e != nil {
		return nil, false, e
	}
	n := old.clone()
	changed := !n.Enrolled
	n.Enrolled = true
	for _, d := range c.Challenges {
		p := n.Rows[d.Index]
		met := d.UnlockKind == "level" && uint32(level) >= d.UnlockValue || d.UnlockKind == "fame" && fame >= d.UnlockValue
		if met && !p.Unlocked {
			p.Unlocked = true
			n.Rows[d.Index] = p
			changed = true
		}
	}
	return n, changed, nil
}

func (c *Catalog) ClaimChallenge(old *ChallengeState, index, action byte) (*ChallengeState, []Reward, error) {
	if old == nil || !old.Enrolled || action > 1 {
		return nil, nil, fmt.Errorf("challenge not enrolled/invalid action")
	}
	if e := c.ValidateChallengeState(old); e != nil {
		return nil, nil, e
	}
	d, e := c.Challenge(index)
	if e != nil {
		return nil, nil, e
	}
	n := old.clone()
	p := n.Rows[index]
	if !p.Unlocked {
		return nil, nil, fmt.Errorf("challenge is locked")
	}
	var rewards []Reward
	if action == 0 {
		if p.UnlockClaimed || !d.UnlockMail {
			return nil, nil, fmt.Errorf("unlock reward already claimed/unsupported delivery")
		}
		p.UnlockClaimed = true
		rewards = d.UnlockRewards
	} else {
		if !d.ClearMail || p.Claims >= d.Repeat || p.Progress/d.Goal <= p.Claims {
			return nil, nil, fmt.Errorf("challenge clear reward not ready")
		}
		p.Claims++
		rewards = d.ClearRewards
	}
	n.Rows[index] = p
	return n, append([]Reward(nil), rewards...), nil
}

// Resolve the completed content on the server, not from C680 parameters.
// Its durable receipt/run ID supplies deduplication in the storage adapter.
func (c *Catalog) ApplyChallengeClear(old *ChallengeState, kind string, content, dungeon uint32) (*ChallengeState, bool, error) {
	if old == nil || !old.Enrolled {
		return old, false, nil
	}
	if e := c.ValidateChallengeState(old); e != nil {
		return nil, false, e
	}
	n := old.clone()
	changed := false
	for _, d := range c.Challenges {
		p := n.Rows[d.Index]
		if !p.Unlocked || d.Kind != kind {
			continue
		}
		match := kind == "clear endkeeper of order" && dungeon != 0 && dungeon == d.GuideDungeon
		for _, id := range d.Conditions {
			match = match || content != 0 && content == id
		}
		limit := d.Goal * d.Repeat
		if match && p.Progress < limit {
			p.Progress++
			n.Rows[d.Index] = p
			changed = true
		}
	}
	return n, changed, nil
}
