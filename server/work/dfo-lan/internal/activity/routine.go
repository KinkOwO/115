package activity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"
)

// RoutineRules is an execution projection of one selected PVF campaign.
// Goal keys describe protocol claim identities, never item IDs or policy data.
type RoutineRules struct {
	Kind, Table, SourceChecksum                             string
	Dates                                                   Dates
	MinimumLevel, SecondsPerProperClear, ProperDungeonCount uint32
	ClickCumulative                                         bool
	SequentialDaily                                         bool // owned paired contract, never native calendar
	Weekly                                                  []Milestone
	Daily                                                   []Milestone
	Cumulative                                              []Milestone
	OnlineDaily                                             map[string][]OnlineReward
	OnlineMaximum                                           map[string]uint32
	OnlinePeriod                                            []OnlineReward
	ItemNames                                               map[uint32]string // selected-source presentation only
}

type RoutineProgress struct {
	SequenceDay  string            `json:"sequence_day,omitempty"`
	Version      uint32            `json:"version"`
	HighWater    time.Time         `json:"high_water,omitempty"`
	Day          string            `json:"day,omitempty"`
	DailyMillis  uint64            `json:"daily_millis"`
	TotalMillis  uint64            `json:"total_millis"`
	PeriodMillis map[uint32]uint64 `json:"period_millis,omitempty"`
	Clears       map[uint32]uint32 `json:"clears,omitempty"`
	Observed     map[uint32]bool   `json:"observed,omitempty"`
	Claimed      map[string]bool   `json:"claimed,omitempty"`
}

func (c RoutineRules) CampaignKey() string {
	h := sha256.Sum256([]byte(c.Kind + "\x00" + c.Table + "\x00" + c.Dates.Start + "\x00" + c.Dates.End))
	return hex.EncodeToString(h[:])
}

func dateWindow(d Dates, loc *time.Location) (time.Time, time.Time, error) {
	a, e := time.ParseInLocation("2006-01-02 15:04:05", d.Start, loc)
	if e != nil {
		return a, a, e
	}
	b, e := time.ParseInLocation("2006-01-02 15:04:05", d.End, loc)
	if e != nil || !b.After(a) {
		return a, b, fmt.Errorf("invalid source activity calendar")
	}
	return a, b, nil
}

func (c RoutineRules) Validate() error {
	if c.Table == "" || len(c.SourceChecksum) != 64 {
		return fmt.Errorf("routine requires selected source")
	}
	if c.SequentialDaily && (c.Kind != "daily" || c.ProperDungeonCount == 0) {
		return fmt.Errorf("sequential daily requires daily clear source")
	}
	start, end, e := dateWindow(c.Dates, time.UTC)
	if e != nil {
		return e
	}
	switch c.Kind {
	case "weekly":
		if len(c.Weekly) == 0 || len(c.Weekly) > 31 {
			return fmt.Errorf("invalid weekly source")
		}
		var previous time.Time
		for i, r := range c.Weekly {
			a, b, e := dateWindow(r.Dates, time.UTC)
			if e != nil || r.Index != uint32(i+1) || r.Required == 0 || a.Before(start) || b.After(end) || (!previous.IsZero() && a.Before(previous)) {
				return fmt.Errorf("invalid weekly period")
			}
			previous = b
		}
	case "daily":
		if len(c.Daily) == 0 || len(c.Daily) > 32 {
			return fmt.Errorf("invalid daily source")
		}
		var previous time.Time
		for i, r := range c.Daily {
			if r.Index != uint32(i+1) {
				return fmt.Errorf("invalid daily sequence")
			}
			if c.SequentialDaily {
				if r.Dates != (Dates{}) {
					return fmt.Errorf("sequential daily cannot carry native calendar rows")
				}
				continue
			}
			a, b, e := dateWindow(r.Dates, time.UTC)
			if e != nil || a.Before(start) || b.After(end) || (!previous.IsZero() && a.Before(previous)) {
				return fmt.Errorf("invalid daily calendar")
			}
			previous = b
		}
	case "online":
		if len(c.OnlineDaily) != 2 || len(c.OnlinePeriod) == 0 {
			return fmt.Errorf("invalid online source")
		}
		for _, k := range []string{"weekday", "weekend"} {
			if len(c.OnlineDaily[k]) == 0 || c.OnlineMaximum[k] == 0 {
				return fmt.Errorf("invalid online daily source")
			}
			var threshold uint64
			for _, row := range c.OnlineDaily[k] {
				if row.Milliseconds == 0 {
					return fmt.Errorf("invalid online daily interval")
				}
				threshold += uint64(row.Milliseconds)
			}
			if threshold > uint64(c.OnlineMaximum[k])*1000 {
				return fmt.Errorf("online daily rewards exceed source daily cap")
			}
		}
		for _, row := range c.OnlinePeriod {
			if row.Milliseconds == 0 {
				return fmt.Errorf("invalid online period interval")
			}
		}
	default:
		return fmt.Errorf("unsupported routine family")
	}
	var last uint32
	limit := len(c.Weekly)
	if c.Kind == "daily" {
		limit = len(c.Daily)
	}
	for _, row := range c.Cumulative {
		if row.Index <= last || row.Required == 0 || row.Required > uint32(limit) {
			return fmt.Errorf("invalid routine cumulative goal")
		}
		last = row.Index
	}
	for _, r := range c.Rewards() {
		if r.Item < 2 || r.Count == 0 || r.MailTitle == "" || r.MailMessage == "" {
			return fmt.Errorf("unbound routine reward")
		}
	}
	return nil
}

func (c RoutineRules) Rewards() []Reward {
	var out []Reward
	for _, rows := range [][]Milestone{c.Weekly, c.Daily, c.Cumulative} {
		for _, r := range rows {
			out = append(out, r.Reward)
		}
	}
	for _, rows := range c.OnlineDaily {
		for _, r := range rows {
			out = append(out, r.Reward)
		}
	}
	for _, r := range c.OnlinePeriod {
		out = append(out, r.Reward)
	}
	return out
}

func (c RoutineRules) Active(now time.Time, level uint32) bool {
	a, b, e := dateWindow(c.Dates, now.Location())
	return e == nil && level >= c.MinimumLevel && !now.Before(a) && now.Before(b)
}

func weekdayKey(now time.Time) string {
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		return "weekend"
	}
	return "weekday"
}

// ValidateProgress refuses damaged or incompatible saves before arithmetic or
// grants. Unknown JSON fields are retained by the workflow; known counters must
// remain representable by their native status fields and selected source rules.
func (c RoutineRules) ValidateProgress(p RoutineProgress, loc *time.Location, resetHour int) error {
	if p.Version > 1 || loc == nil || resetHour < 0 || resetHour > 23 {
		return fmt.Errorf("unsupported routine save/calendar")
	}
	if !c.SequentialDaily && p.SequenceDay != "" {
		return fmt.Errorf("foreign sequence progress")
	}
	if p.Version == 0 {
		if !p.HighWater.IsZero() || p.Day != "" || p.SequenceDay != "" || p.DailyMillis != 0 || p.TotalMillis != 0 || len(p.PeriodMillis)+len(p.Clears)+len(p.Observed)+len(p.Claimed) != 0 {
			return fmt.Errorf("unversioned routine save contains progress")
		}
		return nil
	}
	day, e := time.ParseInLocation("2006-01-02", p.Day, loc)
	if e != nil || p.HighWater.IsZero() {
		return fmt.Errorf("routine save has no valid clock/day")
	}
	last := p.HighWater.In(loc)
	boundary := time.Date(last.Year(), last.Month(), last.Day(), resetHour, 0, 0, 0, loc)
	if last.Before(boundary) {
		boundary = boundary.AddDate(0, 0, -1)
	}
	if boundary.Format("2006-01-02") != p.Day {
		return fmt.Errorf("routine save day disagrees with saved clock")
	}
	switch c.Kind {
	case "weekly":
		if p.DailyMillis != 0 || p.TotalMillis != 0 || len(p.Clears)+len(p.Observed) != 0 {
			return fmt.Errorf("weekly save contains another family counters")
		}
		for index, millis := range p.PeriodMillis {
			if index == 0 || int(index) > len(c.Weekly) || millis > uint64(^uint32(0))*1000 {
				return fmt.Errorf("invalid weekly saved counter")
			}
		}
	case "daily":
		if p.DailyMillis != 0 || p.TotalMillis != 0 || len(p.PeriodMillis) != 0 {
			return fmt.Errorf("daily save contains another family counters")
		}
		for index, n := range p.Clears {
			if index == 0 || int(index) > len(c.Daily) || n > c.ProperDungeonCount {
				return fmt.Errorf("invalid daily saved clear counter")
			}
		}
		for index := range p.Observed {
			if index == 0 || int(index) > len(c.Daily) {
				return fmt.Errorf("invalid daily saved observation")
			}
		}
		if c.SequentialDaily {
			n := uint32(0)
			gap := false
			for _, r := range c.Daily {
				if p.Observed[r.Index] {
					if gap || p.Clears[r.Index] != c.ProperDungeonCount {
						return fmt.Errorf("invalid sequential completion")
					}
					n++
				} else {
					gap = true
					if r.Index > n+1 && p.Clears[r.Index] != 0 {
						return fmt.Errorf("future daily clear progress")
					}
				}
			}
			if n == 0 && p.SequenceDay != "" || n > 0 && p.SequenceDay == "" {
				return fmt.Errorf("missing sequential completion day")
			}
			if p.SequenceDay != "" {
				d, e := time.ParseInLocation("2006-01-02", p.SequenceDay, loc)
				if e != nil || d.After(day) {
					return fmt.Errorf("invalid completion day")
				}
			}
		}
	case "online":
		if len(p.PeriodMillis)+len(p.Clears)+len(p.Observed) != 0 {
			return fmt.Errorf("online save contains another family counters")
		}
		if p.DailyMillis > uint64(c.OnlineMaximum[weekdayKey(day)])*1000 || p.TotalMillis < p.DailyMillis {
			return fmt.Errorf("invalid online saved counter")
		}
		// Even a continuously connected account cannot exceed the campaign's
		// wall time. This loose upper bound detects overflow without inventing
		// a reward threshold or discarding a legitimate previously capped day.
		a, b, e := dateWindow(c.Dates, loc)
		if e != nil || p.TotalMillis > uint64(b.Sub(a)/time.Millisecond) {
			return fmt.Errorf("online total exceeds campaign duration")
		}
	}
	return nil
}

// Observe credits only a server-owned, contiguous connected interval. HighWater
// prevents overlapping account sessions from multiplying online time. The
// caller resets 'from' at login; an old saved timestamp never grants offline time.
func (c RoutineRules) Observe(p RoutineProgress, from, now time.Time, gameDay string, resetHour int, level uint32) (RoutineProgress, error) {
	if e := c.Validate(); e != nil {
		return p, e
	}
	if e := c.ValidateProgress(p, now.Location(), resetHour); e != nil {
		return p, e
	}
	if p.Version > 1 || now.Before(from) || gameDay == "" || resetHour < 0 || resetHour > 23 {
		return p, fmt.Errorf("invalid routine observation")
	}
	if _, e := time.Parse("2006-01-02", gameDay); e != nil {
		return p, e
	}
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), resetHour, 0, 0, 0, now.Location())
	if now.Before(dayStart) {
		dayStart = dayStart.AddDate(0, 0, -1)
	}
	if gameDay != dayStart.Format("2006-01-02") {
		return p, fmt.Errorf("routine game day disagrees with server clock")
	}
	if !c.Active(now, level) {
		return p, ErrRoutineInactive
	}
	p.Version = 1
	if p.PeriodMillis == nil {
		p.PeriodMillis = map[uint32]uint64{}
	}
	if p.Clears == nil {
		p.Clears = map[uint32]uint32{}
	}
	if p.Observed == nil {
		p.Observed = map[uint32]bool{}
	}
	if p.Claimed == nil {
		p.Claimed = map[string]bool{}
	}
	if now.Before(p.HighWater) {
		return p, fmt.Errorf("activity clock moved backwards")
	}
	if from.Before(p.HighWater) {
		from = p.HighWater
	}
	start, _, _ := dateWindow(c.Dates, now.Location())
	if from.Before(start) {
		from = start
	}
	oldDay, oldDaily := p.Day, p.DailyMillis
	if p.Day != gameDay {
		p.Day = gameDay
		p.DailyMillis = 0
		if c.SequentialDaily {
			for index := range p.Clears {
				if !p.Observed[index] {
					delete(p.Clears, index)
				}
			}
		}
	}
	if now.After(from) {
		switch c.Kind {
		case "weekly":
			for _, r := range c.Weekly {
				a, b, _ := dateWindow(r.Dates, now.Location())
				lo, hi := from, now
				if lo.Before(a) {
					lo = a
				}
				if hi.After(b) {
					hi = b
				}
				if hi.After(lo) {
					p.PeriodMillis[r.Index] += uint64(hi.Sub(lo) / time.Millisecond)
					if p.PeriodMillis[r.Index] > uint64(^uint32(0))*1000 {
						return p, fmt.Errorf("weekly time exceeds native counter")
					}
				}
			}
		case "online":
			// Split at the game's reset boundary, so a 30-second tick crossing 06:00
			// cannot credit yesterday's seconds to today's rewards.
			for cursor := from; cursor.Before(now); {
				dayStart := time.Date(cursor.Year(), cursor.Month(), cursor.Day(), resetHour, 0, 0, 0, cursor.Location())
				if cursor.Before(dayStart) {
					dayStart = dayStart.AddDate(0, 0, -1)
				}
				stop := dayStart.AddDate(0, 0, 1)
				if stop.After(now) {
					stop = now
				}
				key := dayStart.Format("2006-01-02")
				// Only the current day's partial bucket is retained, while campaign
				// total includes each connected day subject to that day's source cap.
				used := uint64(0)
				if key == p.Day {
					used = p.DailyMillis
				} else if key == oldDay {
					used = oldDaily
				}
				cap := uint64(c.OnlineMaximum[weekdayKey(dayStart)]) * 1000
				delta := uint64(stop.Sub(cursor) / time.Millisecond)
				if used >= cap {
					delta = 0
				} else if delta > cap-used {
					delta = cap - used
				}
				p.TotalMillis += delta
				if key == p.Day {
					p.DailyMillis += delta
				}
				cursor = stop
			}
		}
	}
	for _, r := range c.Daily {
		if c.SequentialDaily {
			break
		}
		a, b, _ := dateWindow(r.Dates, now.Location())
		if !now.Before(a) && now.Before(b) {
			p.Observed[r.Index] = true
		}
	}
	p.HighWater = now
	return p, nil
}

var ErrRoutineInactive = fmt.Errorf("routine is outside source calendar or level limit")

func (c RoutineRules) ProperClear(p RoutineProgress, now time.Time) RoutineProgress {
	switch c.Kind {
	case "weekly":
		if p.PeriodMillis == nil {
			p.PeriodMillis = map[uint32]uint64{}
		}
		for _, r := range c.Weekly {
			a, b, e := dateWindow(r.Dates, now.Location())
			if e == nil && !now.Before(a) && now.Before(b) {
				p.PeriodMillis[r.Index] += uint64(c.SecondsPerProperClear) * 1000
			}
		}
	case "daily":
		if c.SequentialDaily {
			if p.SequenceDay == p.Day {
				return p
			}
			if p.Clears == nil {
				p.Clears = map[uint32]uint32{}
			}
			if p.Observed == nil {
				p.Observed = map[uint32]bool{}
			}
			for _, r := range c.Daily {
				if p.Observed[r.Index] {
					continue
				}
				if p.Clears[r.Index] < c.ProperDungeonCount {
					p.Clears[r.Index]++
				}
				if p.Clears[r.Index] == c.ProperDungeonCount {
					p.Observed[r.Index] = true
					p.SequenceDay = p.Day
				}
				break
			}
			return p
		}
		if p.Clears == nil {
			p.Clears = map[uint32]uint32{}
		}
		for _, r := range c.Daily {
			a, b, e := dateWindow(r.Dates, now.Location())
			if e == nil && !now.Before(a) && now.Before(b) && p.Clears[r.Index] < c.ProperDungeonCount {
				p.Clears[r.Index]++
			}
		}
	}
	return p
}

func rowKey(kind string, index uint32) string {
	return kind + ":" + strconv.FormatUint(uint64(index), 10)
}
func (c RoutineRules) DailyEligible(p RoutineProgress, index uint32) bool {
	return p.Observed[index] && p.Clears[index] >= c.ProperDungeonCount
}

// Claim checks source goals and mutates only the provided transaction copy.
func (c RoutineRules) Claim(p RoutineProgress, now time.Time, kind string, index uint32) (RoutineProgress, Reward, bool, error) {
	key := rowKey(kind, index)
	var reward Reward
	if e := c.Validate(); e != nil {
		return p, reward, false, e
	}
	if p.Version != 1 || p.HighWater.IsZero() {
		return p, reward, false, fmt.Errorf("claim requires observed versioned progress")
	}
	start, end, e := dateWindow(c.Dates, now.Location())
	if e != nil || now.Before(start) || !now.Before(end) {
		return p, reward, false, ErrRoutineInactive
	}
	eligible := false
	found := false
	switch c.Kind {
	case "weekly":
		if kind == "period" {
			for _, r := range c.Weekly {
				if r.Index == index {
					reward = r.Reward
					found = true
					eligible = p.PeriodMillis[index] >= uint64(r.Required)*1000
				}
			}
		}
		if kind == "cumulative" {
			n := uint32(0)
			for _, r := range c.Weekly {
				if p.Claimed[rowKey("period", r.Index)] {
					n++
				}
			}
			for _, r := range c.Cumulative {
				if r.Index == index {
					reward = r.Reward
					found = true
					eligible = n >= r.Required
				}
			}
		}
	case "daily":
		if kind == "day" {
			for _, r := range c.Daily {
				if r.Index == index {
					reward = r.Reward
					found = true
					eligible = c.DailyEligible(p, index)
				}
			}
		}
		if kind == "cumulative" {
			n := uint32(0)
			for _, r := range c.Daily {
				if p.Claimed[rowKey("day", r.Index)] {
					n++
				}
			}
			for _, r := range c.Cumulative {
				if r.Index == index {
					reward = r.Reward
					found = true
					eligible = n >= r.Required
				}
			}
		}
	case "online":
		if kind == "day" {
			key = p.Day + ":" + key
			day, e := time.ParseInLocation("2006-01-02", p.Day, now.Location())
			if e != nil {
				return p, reward, false, e
			}
			rows := c.OnlineDaily[weekdayKey(day)]
			if index > 0 && int(index) <= len(rows) {
				r := rows[index-1]
				reward = r.Reward
				found = true
				var threshold uint64
				for _, row := range rows[:index] {
					threshold += uint64(row.Milliseconds)
				}
				eligible = p.DailyMillis >= threshold
			}
		}
		if kind == "period" {
			var threshold uint64
			for i, r := range c.OnlinePeriod {
				threshold += uint64(r.Milliseconds)
				if uint32(i+1) == index {
					reward = r.Reward
					found = true
					eligible = p.TotalMillis >= threshold
				}
			}
		}
	}
	if !found {
		return p, reward, false, fmt.Errorf("unknown source claim")
	}
	if p.Claimed[key] {
		return p, reward, false, nil
	}
	if !eligible {
		return p, reward, false, fmt.Errorf("activity goal not complete")
	}
	if p.Claimed == nil {
		p.Claimed = map[string]bool{}
	}
	p.Claimed[key] = true
	return p, reward, true, nil
}
