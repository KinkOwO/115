package activity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// LoginProgress is account state, separate from any character's inventory.
// Campaign identity uses the source table and calendar, not the whole PVF hash:
// importing an unrelated item must not unlock already claimed rewards again.
type LoginProgress struct {
	Version uint32          `json:"version"`
	LastDay string          `json:"last_day,omitempty"`
	Visits  uint32          `json:"visits"`
	Claimed map[uint32]bool `json:"claimed,omitempty"`
}

func (c Login) CampaignKey() string {
	h := sha256.Sum256([]byte(c.Table + "\x00" + c.Dates.Start + "\x00" + c.Dates.End))
	return hex.EncodeToString(h[:])
}

func (c Login) validateExecution() error {
	// This executor covers the cumulative login campaign in the selected source.
	// The non-cumulative branch needs separate native semantics, not a fallback.
	if !c.Accumulate || c.Table == "" || len(c.Days) == 0 || len(c.Days) > 28 {
		return fmt.Errorf("unsupported or incomplete cumulative attendance source")
	}
	for i, day := range c.Days {
		if day.Day != uint32(i) || len(day.Rewards) == 0 {
			return fmt.Errorf("invalid attendance day %d", i)
		}
		for _, reward := range day.Rewards {
			if reward.Item < 2 || reward.Count == 0 {
				return fmt.Errorf("invalid attendance reward on day %d", i)
			}
		}
	}
	return nil
}

func (c Login) validateProgress(p LoginProgress) error {
	if p.Version > 1 || p.Visits > uint32(len(c.Days)) ||
		(p.Version == 0 && (p.Visits != 0 || p.LastDay != "" || len(p.Claimed) != 0)) ||
		(p.Version == 1 && (p.Visits == 0 || p.LastDay == "")) {
		return fmt.Errorf("unsupported attendance save")
	}
	if p.LastDay != "" {
		if _, err := time.Parse("2006-01-02", p.LastDay); err != nil {
			return err
		}
	}
	for day, claimed := range p.Claimed {
		if day >= p.Visits || !claimed {
			return fmt.Errorf("invalid attendance claim save")
		}
	}
	return nil
}

// Observe counts a server-observed login once per game day. Missing days are
// never fabricated. The caller supplies the authoritative game day, allowing
// the gateway's calendar to stay responsible for timezone/reset conventions.
func (c Login) Observe(p LoginProgress, now time.Time, gameDay string, level uint32) (LoginProgress, bool, error) {
	if err := c.validateExecution(); err != nil {
		return p, false, err
	}
	start, err := time.ParseInLocation("2006-01-02 15:04:05", c.Dates.Start, now.Location())
	if err != nil {
		return p, false, err
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", c.Dates.End, now.Location())
	if err != nil || !end.After(start) {
		return p, false, fmt.Errorf("invalid attendance calendar")
	}
	if _, err = time.Parse("2006-01-02", gameDay); err != nil {
		return p, false, err
	}
	if err = c.validateProgress(p); err != nil {
		return p, false, err
	}
	if p.LastDay != "" {
		if gameDay < p.LastDay {
			return p, false, fmt.Errorf("attendance clock moved backwards")
		}
	}
	next := p
	next.Claimed = make(map[uint32]bool, len(p.Claimed))
	for day, claimed := range p.Claimed {
		next.Claimed[day] = claimed
	}
	if now.Before(start) || !now.Before(end) || level < c.MinimumLevel {
		return next, false, nil
	}
	next.Version = 1
	if next.LastDay != gameDay {
		next.LastDay = gameDay
		if next.Visits < uint32(len(c.Days)) {
			next.Visits++
		}
	}
	return next, true, nil
}

// Claim returns the source-defined rewards only once. It does not advance login
// progress; the caller must first reconcile a server-observed visit.
func (c Login) Claim(p LoginProgress, day uint32) (LoginProgress, []Reward, bool, error) {
	if err := c.validateExecution(); err != nil {
		return p, nil, false, err
	}
	if err := c.validateProgress(p); err != nil {
		return p, nil, false, err
	}
	if p.Version != 1 || day >= uint32(len(c.Days)) || day >= p.Visits {
		return p, nil, false, fmt.Errorf("attendance day is not unlocked")
	}
	if p.Claimed[day] {
		return p, nil, false, nil
	}
	next := p
	next.Claimed = make(map[uint32]bool, len(p.Claimed)+1)
	for k, v := range p.Claimed {
		next.Claimed[k] = v
	}
	next.Claimed[day] = true
	return next, append([]Reward(nil), c.Days[day].Rewards...), true, nil
}

// DayStates uses the native client's 28-byte array. Native getters
// 143869970/143869990 test 2=claimed and 1=claimable; the Oct-02 capture confirms
// the first-day 1->2 transition and its persistence across later logins.
func (c Login) DayStates(p LoginProgress) ([28]byte, error) {
	var states [28]byte
	if err := c.validateExecution(); err != nil {
		return states, err
	}
	if err := c.validateProgress(p); err != nil {
		return states, err
	}
	for day := uint32(0); day < p.Visits; day++ {
		states[day] = 1
		if p.Claimed[day] {
			states[day] = 2
		}
	}
	return states, nil
}
