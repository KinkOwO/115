package boostup

import "fmt"

// Persist together with the owned item mutations; never advance this state
// merely because an ACK was written. Claimed survives all character reloads.
type Training struct {
	Step     byte          `json:"step"`
	Phase    byte          `json:"phase"` // 0 guide, 1 claimable, 2 claimed/mission pending
	Finished bool          `json:"finished"`
	Claimed  map[byte]bool `json:"claimed"`
}

func (c *Catalog) Begin() (Training, error) {
	if c == nil || len(c.Steps) == 0 {
		return Training{}, fmt.Errorf("missing boost steps")
	}
	s := Training{Step: 1, Claimed: map[byte]bool{}}
	if c.Steps[0].Guide == "none" {
		s.Phase = 1
	}
	return s, nil
}
func (c *Catalog) current(s Training) (Step, error) {
	if c == nil || s.Finished || s.Step == 0 || int(s.Step) > len(c.Steps) || s.Phase > 2 {
		return Step{}, fmt.Errorf("invalid active boost step")
	}
	return c.Steps[int(s.Step)-1], nil
}

// This is UI acknowledgement only. Dungeon and equipment proofs are not
// granted by the generic C681 query.
func (c *Catalog) GuideViewed(s Training, step byte) (Training, error) {
	row, e := c.current(s)
	if e != nil {
		return s, e
	}
	if step != s.Step || row.Guide != "normal" {
		return s, fmt.Errorf("query cannot finish this guide")
	}
	if s.Phase == 0 {
		s.Phase = 1
	}
	return s, nil
}

// expected is selected from THIS source step's profession/variant route by
// the caller. The actual cleared instance must be independently verified.
func (c *Catalog) GuideDungeonCleared(s Training, expected, actual uint32) (Training, error) {
	row, e := c.current(s)
	if e != nil {
		return s, e
	}
	if row.Guide != "dungeon" || expected == 0 || actual != expected {
		return s, fmt.Errorf("wrong boost guide instance")
	}
	valid := expected == row.Dungeon
	v := values(row.GuideCells, "[specific dungeon index]")
	for i := 0; i+3 < len(v); i += 4 {
		if v[i+3].Type == 0 && v[i+3].Value > 0 && uint32(v[i+3].Value) == expected {
			valid = true
		}
	}
	if !valid {
		return s, fmt.Errorf("guide route not in current source")
	}
	if s.Phase == 0 {
		s.Phase = 1
	}
	return s, nil
}
func (c *Catalog) next(s Training) Training {
	s.Step++
	s.Phase = 0
	if int(s.Step) > len(c.Steps) {
		s.Finished = true
	} else if c.Steps[int(s.Step)-1].Guide == "none" {
		s.Phase = 1
	}
	return s
}

// Returns a candidate, not a committed state. The caller MUST atomically
// insert these exact rewards and persist candidate, or keep the old state.
func (c *Catalog) Claim(s Training, step byte, buffer bool) (Training, []Reward, error) {
	if s.Claimed[step] {
		return s, nil, nil
	}
	row, e := c.current(s)
	if e != nil {
		return s, nil, e
	}
	if step != s.Step || s.Phase != 1 {
		return s, nil, fmt.Errorf("boost reward not ready")
	}
	items := row.Rewards
	if buffer && len(row.BufferRewards) > 0 {
		items = row.BufferRewards
	}
	s.Claimed = cloneClaims(s.Claimed)
	s.Claimed[step] = true
	s.Phase = 2
	if row.Mission == "none" {
		s = c.next(s)
	}
	return s, append([]Reward(nil), items...), nil
}
func cloneClaims(m map[byte]bool) map[byte]bool {
	n := make(map[byte]bool, len(m)+1)
	for k, v := range m {
		n[k] = v
	}
	return n
}

// Only a server-side source-condition evaluator may call this, after it
// checked durable inventory/VP/enchantment/journal state. No wire flag is a
// mission proof. Matching the type prevents a wrong hook advancing a step.
func (c *Catalog) MissionCompleted(s Training, mission string) (Training, error) {
	row, e := c.current(s)
	if e != nil {
		return s, e
	}
	if s.Phase != 2 || !s.Claimed[s.Step] || row.Mission == "none" || row.Mission != mission {
		return s, fmt.Errorf("mission completion outside claimed step")
	}
	return c.next(s), nil
}
