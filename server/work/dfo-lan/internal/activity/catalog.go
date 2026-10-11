// Package activity reads attendance and online-event content from the selected
// PVF. Loading content does not advertise an event or enable an unverified codec.
package activity

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
	"time"
)

const (
	WeeklyPath = "event/periodconnectionkeepingevent.evt"
	OnlinePath = "live/event/chn/routine_event/honeytime/honeytime.evt"
	DailyPath  = "live/event/chn/2026/0922_nationalday_attendance/dailymissionevent.evt"
	LoginPath  = "live/event/kor/2026/0326_attendancedailyevent/attendancedailyevent.evt"
)

type TokenSource interface {
	Tokens(string) ([]pvf.Token, error)
}

// Dates retain the original script calendar. A local operating calendar must be
// supplied separately; loading never silently repairs stale source dates.
type Dates struct{ Start, End string }
type Reward struct {
	Item, Count            uint32
	MailTitle, MailMessage string
}
type Milestone struct {
	Index, Required uint32
	Reward          Reward
	Dates           Dates // present in the 115US weekly layout
}
type Weekly struct {
	Dates                               Dates
	Table                               string
	MinimumLevel, SecondsPerProperClear uint32
	ClickCumulative                     bool
	Periods, Cumulative                 []Milestone
}
type OnlineReward struct {
	Index, Milliseconds uint32
	Reward              Reward
	Options             []bool // preserve both native daily flags; their execution is separate
}
type Online struct {
	Dates                                           Dates
	Table, UI                                       string
	PCRoom, DailyAutoget                            bool
	DailyLoop, ExtendedDailyLoop                    uint32
	Daily                                           map[string][]OnlineReward
	DailyMaximumSeconds                             map[string]uint32
	Period                                          []OnlineReward
	DailyUIMinutes, PeriodUIHours                   []uint32
	InnerRewardBan                                  []pvf.Token
	WeeklyLoopIncrease, SpecifiedWeeklyLoopIncrease []uint32
	ExtendedRewards, SpecifiedRewards               []pvf.Token
}
type Daily struct {
	Dates                                 Dates
	Table, Mode, UI, UIDateStart          string
	MinimumLevel, TodayBonus, MissionType uint32
	ProperDungeonCounts                   []uint32
	UseCumulative                         bool
	Days, Cumulative                      []Milestone
}
type Catalog struct {
	Weekly Weekly
	Online Online
	Daily  Daily
	Login  Login
}

type LoginDay struct {
	Day     uint32
	Rewards []Reward
	Special bool
}
type Login struct {
	SourceChecksum                    string
	MailTextTitle, MailTextMessage    string
	CalendarUI                        string
	Dates                             Dates
	Table, UI, MailTitle, MailMessage string
	MinimumLevel                      uint32
	Accumulate, FirstLoginPopup       bool
	Days                              []LoginDay
}

func Load(src TokenSource) (*Catalog, error) {
	c := new(Catalog)
	for _, entry := range []struct {
		path  string
		parse func([]pvf.Token) error
	}{
		{WeeklyPath, func(t []pvf.Token) (e error) { c.Weekly, e = ParseWeekly(t); return }},
		{OnlinePath, func(t []pvf.Token) (e error) { c.Online, e = ParseOnline(t); return }},
		{DailyPath, func(t []pvf.Token) (e error) { c.Daily, e = ParseDaily(t); return }},
		{LoginPath, func(t []pvf.Token) (e error) { c.Login, e = ParseLogin(t); return }},
	} {
		t, e := src.Tokens(entry.path)
		if e == nil {
			e = entry.parse(t)
		}
		if e != nil {
			return nil, fmt.Errorf("activity %s: %w", entry.path, e)
		}
	}
	return c, nil
}

func value(t []pvf.Token, tag string) ([]pvf.Token, error) {
	var out []pvf.Token
	found := false
	for i, cell := range t {
		if cell.Type != 3 || cell.Text != tag {
			continue
		}
		if found {
			return nil, fmt.Errorf("duplicate %s", tag)
		}
		found = true
		j := i + 1
		for j < len(t) && t[j].Type != 3 {
			j++
		}
		out = t[i+1 : j]
	}
	if !found {
		return nil, fmt.Errorf("missing %s", tag)
	}
	return out, nil
}
func section(t []pvf.Token, tag string) ([][]pvf.Token, error) {
	close := "[/" + strings.TrimPrefix(tag, "[")
	start := -1
	var out [][]pvf.Token
	for i, cell := range t {
		if cell.Type != 3 {
			continue
		}
		if cell.Text == tag {
			if start >= 0 {
				return nil, fmt.Errorf("nested %s", tag)
			}
			start = i + 1
		}
		if cell.Text == close {
			if start < 0 {
				return nil, fmt.Errorf("orphan %s", close)
			}
			out = append(out, t[start:i])
			start = -1
		}
	}
	if start >= 0 {
		return nil, fmt.Errorf("unterminated %s", tag)
	}
	return out, nil
}
func one(t []pvf.Token, tag string) ([]pvf.Token, error) {
	rows, e := section(t, tag)
	if e != nil {
		return nil, e
	}
	if len(rows) != 1 {
		return nil, fmt.Errorf("expected one %s, got %d", tag, len(rows))
	}
	return rows[0], nil
}
func num(t pvf.Token) (uint32, error) {
	if t.Type != 0 || t.Value < 0 {
		return 0, fmt.Errorf("expected nonnegative integer")
	}
	return uint32(t.Value), nil
}
func text(t pvf.Token) (string, error) {
	if t.Type != 6 {
		return "", fmt.Errorf("expected source string")
	}
	return t.Text, nil
}
func mailKey(t pvf.Token) (string, error) {
	if t.Type == 8 && t.Reference != "" {
		return t.Reference, nil
	}
	return text(t)
}
func number(t []pvf.Token, tag string) (uint32, error) {
	v, e := value(t, tag)
	if e != nil {
		return 0, e
	}
	if len(v) != 1 {
		return 0, fmt.Errorf("invalid %s width", tag)
	}
	n, e := num(v[0])
	if e != nil {
		return 0, fmt.Errorf("%s: %w", tag, e)
	}
	return n, nil
}
func label(t []pvf.Token, tag string) (string, error) {
	v, e := value(t, tag)
	if e != nil {
		return "", e
	}
	if len(v) != 1 {
		return "", fmt.Errorf("invalid %s width", tag)
	}
	return text(v[0])
}
func flag(t pvf.Token) (bool, error) {
	if t.Type == 0 && (t.Value == 0 || t.Value == 1) {
		return t.Value == 1, nil
	}
	if t.Type == 6 && (t.Text == "true" || t.Text == "false") {
		return t.Text == "true", nil
	}
	return false, fmt.Errorf("invalid boolean")
}
func boolean(t []pvf.Token, tag string) (bool, error) {
	v, e := value(t, tag)
	if e != nil {
		return false, e
	}
	if len(v) != 1 {
		return false, fmt.Errorf("invalid %s width", tag)
	}
	return flag(v[0])
}
func numbers(t []pvf.Token) ([]uint32, error) {
	r := make([]uint32, len(t))
	for i, v := range t {
		n, e := num(v)
		if e != nil {
			return nil, e
		}
		r[i] = n
	}
	return r, nil
}
func dates(v []pvf.Token) (Dates, error) {
	if len(v) != 2 {
		return Dates{}, fmt.Errorf("date range requires two strings")
	}
	a, e := text(v[0])
	if e != nil {
		return Dates{}, e
	}
	b, e := text(v[1])
	if e != nil {
		return Dates{}, e
	}
	x, e := time.Parse("2006-01-02 15:04:05", a)
	if e != nil {
		return Dates{}, e
	}
	y, e := time.Parse("2006-01-02 15:04:05", b)
	if e != nil {
		return Dates{}, e
	}
	if !y.After(x) {
		return Dates{}, fmt.Errorf("event end must follow start")
	}
	return Dates{a, b}, nil
}
func reward(v []pvf.Token, mail bool) (Reward, error) {
	want := 2
	if mail {
		want = 4
	}
	if len(v) != want {
		return Reward{}, fmt.Errorf("invalid reward width")
	}
	i, e := num(v[0])
	if e != nil {
		return Reward{}, e
	}
	n, e := num(v[1])
	if e != nil {
		return Reward{}, e
	}
	if i < 2 || n == 0 {
		return Reward{}, fmt.Errorf("invalid reward item/count")
	}
	r := Reward{Item: i, Count: n}
	if mail {
		r.MailTitle, e = mailKey(v[2])
		if e != nil {
			return Reward{}, e
		}
		r.MailMessage, e = mailKey(v[3])
		if e != nil {
			return Reward{}, e
		}
	}
	return r, nil
}
func milestones(t []pvf.Token, tag string, timed bool) ([]Milestone, error) {
	v, e := one(t, tag)
	if e != nil {
		return nil, e
	}
	w := 5
	if timed {
		w = 6
	}
	if len(v) == 0 || len(v)%w != 0 {
		return nil, fmt.Errorf("invalid %s rows", tag)
	}
	r := make([]Milestone, 0, len(v)/w)
	var last uint32
	for i := 0; i < len(v); i += w {
		index, e := num(v[i])
		if e != nil || index <= last {
			return nil, fmt.Errorf("unordered %s milestone", tag)
		}
		last = index
		m := Milestone{Index: index, Required: index}
		off := i + 1
		if timed {
			m.Required, e = num(v[off])
			if e != nil || m.Required == 0 {
				return nil, fmt.Errorf("invalid period seconds")
			}
			off++
		}
		m.Reward, e = reward(v[off:i+w], true)
		if e != nil {
			return nil, e
		}
		r = append(r, m)
	}
	return r, nil
}

func ParseWeekly(t []pvf.Token) (c Weekly, e error) {
	for _, token := range t {
		if token.Type == 3 && (token.Text == "[accu period reward info]" || token.Text == "[bonus period reward info]") {
			return c, fmt.Errorf("weekly source branch requires its own execution: %s", token.Text)
		}
	}
	if c.Table, e = label(t, "[db table name]"); e != nil {
		return
	}
	v, e := one(t, "[event period]")
	if e != nil {
		return c, e
	}
	if c.Dates, e = dates(v); e != nil {
		return
	}
	if c.Periods, e = weeklyPeriods(t); e != nil {
		return
	}
	if c.Cumulative, e = weeklyCumulative(t); e != nil {
		return
	}
	if c.MinimumLevel, e = number(t, "[level limit]"); e != nil {
		return
	}
	if c.SecondsPerProperClear, e = number(t, "[add sec on proper dungeon clear]"); e != nil {
		return
	}
	c.ClickCumulative, e = boolean(t, "[click button on accu reward]")
	return
}

// The US reader consumes two dates before the seconds/reward/mail cells.
// Current CN omits these per-period dates. Both retain source mail keys.
func weeklyPeriods(t []pvf.Token) ([]Milestone, error) {
	v, e := one(t, "[keeping connection info as period]")
	if e != nil {
		return nil, e
	}
	if len(v) < 2 || v[1].Type != 6 {
		return milestones(t, "[keeping connection info as period]", true)
	}
	if len(v) == 0 || len(v)%8 != 0 {
		return nil, fmt.Errorf("invalid US weekly periods")
	}
	var r []Milestone
	var last uint32
	for i := 0; i < len(v); i += 8 {
		index, e := num(v[i])
		if e != nil || index <= last {
			return nil, fmt.Errorf("unordered weekly period")
		}
		last = index
		d, e := dates(v[i+1 : i+3])
		if e != nil {
			return nil, e
		}
		seconds, e := num(v[i+3])
		if e != nil || seconds == 0 {
			return nil, fmt.Errorf("invalid weekly seconds")
		}
		reward, e := reward(v[i+4:i+8], true)
		if e != nil {
			return nil, e
		}
		r = append(r, Milestone{Index: index, Required: seconds, Reward: reward, Dates: d})
	}
	return r, nil
}
func weeklyCumulative(t []pvf.Token) ([]Milestone, error) {
	return milestones(t, "[accu reward info]", false)
}

func control(t []pvf.Token, n int) (string, Dates, string, error) {
	v, e := one(t, "[control panel]")
	if e != nil {
		return "", Dates{}, "", e
	}
	if len(v) != n {
		return "", Dates{}, "", fmt.Errorf("invalid control panel width")
	}
	table, e := text(v[0])
	if e != nil {
		return "", Dates{}, "", e
	}
	d, e := dates(v[1:3])
	if e != nil {
		return "", d, "", e
	}
	mode := ""
	if n == 4 {
		mode, e = text(v[3])
	}
	return table, d, mode, e
}
func onlineRewards(v []pvf.Token, daily bool) ([]OnlineReward, error) {
	w := 5
	if daily {
		w = 6
	}
	if len(v) == 0 || len(v)%w != 0 {
		return nil, fmt.Errorf("invalid online reward rows")
	}
	r := make([]OnlineReward, 0, len(v)/w)
	for i := 0; i < len(v); i += w {
		var o OnlineReward
		var e error
		if o.Index, e = num(v[i]); e != nil || o.Index == 0 {
			return nil, fmt.Errorf("invalid online reward index")
		}
		if o.Reward, e = reward(v[i+1:i+3], false); e != nil {
			return nil, e
		}
		if o.Milliseconds, e = num(v[i+3]); e != nil || o.Milliseconds == 0 {
			return nil, fmt.Errorf("invalid online interval")
		}
		for j := i + 4; j < i+w; j++ {
			b, e := flag(v[j])
			if e != nil {
				return nil, e
			}
			o.Options = append(o.Options, b)
		}
		r = append(r, o)
	}
	return r, nil
}
func ParseOnline(t []pvf.Token) (c Online, e error) {
	if c.Table, c.Dates, _, e = control(t, 3); e != nil {
		return
	}
	if c.UI, e = label(t, "[ui file]"); e != nil {
		return
	}
	if c.PCRoom, e = boolean(t, "[pc room event]"); e != nil {
		return
	}
	if c.DailyAutoget, e = boolean(t, "[daily reward autoget]"); e != nil {
		return
	}
	if c.DailyLoop, e = number(t, "[daily reward loop]"); e != nil {
		return
	}
	if c.ExtendedDailyLoop, e = number(t, "[extend daily reward loop]"); e != nil {
		return
	}
	c.Daily = map[string][]OnlineReward{}
	c.DailyMaximumSeconds = map[string]uint32{}
	rows, e := section(t, "[daily reward info]")
	if e != nil {
		return c, e
	}
	if len(rows) == 0 {
		return c, fmt.Errorf("no daily online rewards")
	}
	for _, row := range rows {
		kind, e := label(row, "[week type]")
		if e != nil {
			return c, e
		}
		if kind != "weekday" && kind != "weekend" {
			return c, fmt.Errorf("unsupported week type %q", kind)
		}
		if _, ok := c.Daily[kind]; ok {
			return c, fmt.Errorf("duplicate week type %q", kind)
		}
		v, e := one(row, "[reward items]")
		if e != nil {
			return c, e
		}
		r, e := onlineRewards(v, true)
		if e != nil {
			return c, e
		}
		c.Daily[kind] = r
	}
	v, e := one(t, "[daily Accumulated Time]")
	if e != nil {
		return c, e
	}
	if len(v)%2 != 0 {
		return c, fmt.Errorf("invalid daily maxima")
	}
	for i := 0; i < len(v); i += 2 {
		k, e := text(v[i])
		if e != nil {
			return c, e
		}
		n, e := num(v[i+1])
		if e != nil || n == 0 {
			return c, fmt.Errorf("invalid daily maximum")
		}
		if _, ok := c.DailyMaximumSeconds[k]; ok {
			return c, fmt.Errorf("duplicate daily maximum")
		}
		if _, ok := c.Daily[k]; !ok {
			return c, fmt.Errorf("maximum without daily rewards")
		}
		c.DailyMaximumSeconds[k] = n
	}
	if len(c.DailyMaximumSeconds) != len(c.Daily) {
		return c, fmt.Errorf("missing daily maximum")
	}
	v, e = one(t, "[period reward item]")
	if e != nil {
		return c, e
	}
	if c.Period, e = onlineRewards(v, false); e != nil {
		return
	}
	ui, e := one(t, "[reward ui time info]")
	if e != nil {
		return c, e
	}
	for _, f := range []struct {
		tag  string
		dest *[]uint32
	}{{"[daily]", &c.DailyUIMinutes}, {"[period]", &c.PeriodUIHours}} {
		part, e := one(ui, f.tag)
		if e != nil {
			return c, e
		}
		v, e := one(part, "[time]")
		if e != nil {
			return c, e
		}
		n, e := numbers(v)
		if e != nil {
			return c, e
		}
		*f.dest = n
	}
	for _, r := range c.Daily {
		if len(r) != len(c.DailyUIMinutes) {
			return c, fmt.Errorf("daily UI/reward count mismatch")
		}
	}
	if len(c.Period) != len(c.PeriodUIHours) {
		return c, fmt.Errorf("period UI/reward count mismatch")
	}
	if c.InnerRewardBan, e = one(t, "[inner reward ban]"); e != nil {
		return
	}
	if c.ExtendedRewards, e = one(t, "[extend daily reward items]"); e != nil {
		return
	}
	if c.SpecifiedRewards, e = one(t, "[specified period reward item]"); e != nil {
		return
	}
	for _, f := range []struct {
		tag  string
		dest *[]uint32
	}{{"[period reward weekly loop increase value]", &c.WeeklyLoopIncrease}, {"[specified period reward weekly loop increase value]", &c.SpecifiedWeeklyLoopIncrease}} {
		v, e := value(t, f.tag)
		if e != nil {
			return c, e
		}
		n, e := numbers(v)
		if e != nil {
			return c, e
		}
		*f.dest = n
	}
	return
}

func ParseDaily(t []pvf.Token) (c Daily, e error) {
	if c.Table, c.Dates, c.Mode, e = control(t, 4); e != nil {
		return
	}
	if c.UI, e = label(t, "[ui file]"); e != nil {
		return
	}
	if c.UIDateStart, e = label(t, "[ui date start]"); e != nil {
		return
	}
	if _, e = time.Parse("2006-01-02 15:04:05", c.UIDateStart); e != nil {
		return
	}
	if c.MinimumLevel, e = number(t, "[level limit]"); e != nil {
		return
	}
	if c.TodayBonus, e = number(t, "[today bonus]"); e != nil {
		return
	}
	if c.MissionType, e = number(t, "[mission type]"); e != nil {
		return
	}
	if c.UseCumulative, e = boolean(t, "[use accumulated reward]"); e != nil {
		return
	}
	v, e := value(t, "[proper dungeon count]")
	if e != nil {
		return c, e
	}
	if c.ProperDungeonCounts, e = numbers(v); e != nil {
		return
	}
	if len(c.ProperDungeonCounts) != 2 || c.ProperDungeonCounts[0] == 0 || c.ProperDungeonCounts[1] == 0 {
		return c, fmt.Errorf("invalid proper dungeon counts")
	}
	if c.Days, e = milestones(t, "[daily reward]", false); e != nil {
		return
	}
	c.Cumulative, e = milestones(t, "[accumulated reward]", false)
	return
}

// RewardItems is a validation projection, not a second gameplay table.
func (c *Catalog) RewardItems() map[uint32]bool {
	r := map[uint32]bool{}
	for _, rows := range [][]Milestone{c.Weekly.Periods, c.Weekly.Cumulative, c.Daily.Days, c.Daily.Cumulative} {
		for _, v := range rows {
			r[v.Reward.Item] = true
		}
	}
	for _, rows := range c.Online.Daily {
		for _, v := range rows {
			r[v.Reward.Item] = true
		}
	}
	for _, v := range c.Online.Period {
		r[v.Reward.Item] = true
	}
	for _, d := range c.Login.Days {
		for _, v := range d.Rewards {
			r[v.Item] = true
		}
	}
	return r
}

func ParseLogin(t []pvf.Token) (c Login, e error) {
	v, e := one(t, "[event period]")
	if e != nil {
		return c, e
	}
	if c.Dates, e = dates(v); e != nil {
		return
	}
	if c.Table, e = label(t, "[db table name]"); e != nil {
		return
	}
	if c.UI, e = label(t, "[ui path]"); e != nil {
		return
	}
	if c.MailTitle, e = label(t, "[reward mail title]"); e != nil {
		return
	}
	if c.MailMessage, e = label(t, "[reward mail message]"); e != nil {
		return
	}
	if c.MinimumLevel, e = number(t, "[level limit]"); e != nil {
		return
	}
	if c.Accumulate, e = boolean(t, "[allow accumulate attendance]"); e != nil {
		return
	}
	if c.FirstLoginPopup, e = boolean(t, "[first login open popup]"); e != nil {
		return
	}
	v, e = one(t, "[reward infos]")
	if e != nil {
		return c, e
	}
	rows, e := section(v, "[reward info]")
	if e != nil {
		return c, e
	}
	if len(rows) == 0 {
		return c, fmt.Errorf("empty login rewards")
	}
	for i, row := range rows {
		var d LoginDay
		if d.Day, e = number(row, "[day]"); e != nil {
			return c, e
		}
		if d.Day != uint32(i) {
			return c, fmt.Errorf("login day sequence must begin at zero")
		}
		cells, e := one(row, "[reward items]")
		if e != nil {
			return c, e
		}
		if len(cells) == 0 || len(cells)%2 != 0 {
			return c, fmt.Errorf("invalid login reward pairs")
		}
		for j := 0; j < len(cells); j += 2 {
			r, e := reward(cells[j:j+2], false)
			if e != nil {
				return c, e
			}
			r.MailTitle = c.MailTitle
			r.MailMessage = c.MailMessage
			d.Rewards = append(d.Rewards, r)
		}
		for _, cell := range row {
			if cell.Type == 3 && cell.Text == "[special reward]" {
				d.Special = true
			}
		}
		c.Days = append(c.Days, d)
	}
	return
}
