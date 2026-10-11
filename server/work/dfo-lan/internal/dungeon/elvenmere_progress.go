package dungeon

import (
	"encoding/json"
	"fmt"
	"time"
)

// Progress is projected from account receipts. Version 1 holds remain readable
// for migration; version 2 separates the US weekly period and ongoing season.
type ElvenmereClaim struct {
	EventKey  string `json:"event_key"`
	ClaimedAt int64  `json:"claimed_at,omitempty"`
}
type ElvenmereProgress struct {
	WeekStart  int64                   `json:"week_start,omitempty"`
	SeasonID   string                  `json:"season_id,omitempty"`
	StartFloor byte                    `json:"start_floor,omitempty"`
	Version    int                     `json:"version"`
	MaxCleared byte                    `json:"max_cleared"`
	Weekly     map[byte]ElvenmereClaim `json:"weekly,omitempty"`
	Season     map[byte]ElvenmereClaim `json:"season,omitempty"`
}

func ReadElvenmereProgress(raw json.RawMessage) (ElvenmereProgress, bool, error) {
	p := ElvenmereProgress{Version: 1}
	if len(raw) == 0 {
		return p, false, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return p, false, fmt.Errorf("invalid elvenmere character state")
	}
	value, ok := fields["elvenmere_progress"]
	if !ok {
		return p, false, nil
	}
	p = ElvenmereProgress{}
	if err := json.Unmarshal(value, &p); err != nil {
		return p, true, err
	}
	return p, true, validateElvenmereProgress(p)
}

func validateElvenmereProgress(p ElvenmereProgress) error {
	if (p.Version != 1 && p.Version != 2) || p.MaxCleared > 100 {
		return fmt.Errorf("unsupported elvenmere progress")
	}
	if p.Version == 2 && (p.WeekStart <= 0 || ElvenmereWeekStart(time.Unix(p.WeekStart, 0)) != p.WeekStart || p.SeasonID == "" || len(p.SeasonID) > 40 || p.StartFloor > p.MaxCleared || (p.MaxCleared == 0) != (p.StartFloor == 0)) {
		return fmt.Errorf("invalid elvenmere period projection")
	}
	for i, claims := range []map[byte]ElvenmereClaim{p.Weekly, p.Season} {
		for floor, claim := range claims {
			if floor == 0 || floor > 100 || (p.Version == 1 || i == 0) && floor > p.MaxCleared || claim.EventKey == "" || len(claim.EventKey) > 200 || claim.ClaimedAt < 0 {
				return fmt.Errorf("invalid elvenmere claim hold")
			}
			if p.Version == 2 && (claim.ClaimedAt <= 0 || i == 0 && ElvenmereWeekStart(time.Unix(claim.ClaimedAt, 0)) != p.WeekStart) {
				return fmt.Errorf("invalid elvenmere claim period")
			}
		}
	}
	return nil
}

func WriteElvenmereProgress(raw json.RawMessage, p ElvenmereProgress) (json.RawMessage, error) {
	if err := validateElvenmereProgress(p); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("invalid elvenmere character state")
	}
	// Preserve unknown fields in both the character and the version-1 ledger.
	var ledger map[string]json.RawMessage
	if old := fields["elvenmere_progress"]; len(old) > 0 {
		if err := json.Unmarshal(old, &ledger); err != nil || ledger == nil {
			return nil, fmt.Errorf("invalid elvenmere progress object")
		}
	}
	if ledger == nil {
		ledger = map[string]json.RawMessage{}
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	var known map[string]json.RawMessage
	if err = json.Unmarshal(encoded, &known); err != nil {
		return nil, err
	}
	for _, key := range []string{"version", "max_cleared", "weekly", "season", "week_start", "season_id", "start_floor"} {
		delete(ledger, key)
	}
	for key, value := range known {
		ledger[key] = value
	}
	fields["elvenmere_progress"], err = json.Marshal(ledger)
	if err != nil {
		return nil, err
	}
	return json.Marshal(fields)
}

func (p ElvenmereProgress) RewardFlags() (weekly, season [100]byte) {
	for floor := range p.Weekly {
		weekly[floor-1] = 1
	}
	for floor := range p.Season {
		season[floor-1] = 1
	}
	return
}

// Consume only after the reward transaction and response preparation succeed.
// The next boss report requires a fresh owned load; an old Completed flag or
// completionTarget cannot authorize another floor.
func (s *Session) AdvanceElvenmereFloor(floor, next byte) {
	s.ElvenmereLastFloor = floor
	s.Extra = uint16(next)
	s.completed = false
	s.completionTarget = 0
	s.Loaded = false
}
