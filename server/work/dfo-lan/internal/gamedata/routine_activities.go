package gamedata

import (
	"dfolan/internal/activity"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

// RoutineWeekly prepares the verified US dated weekly grammar independently
// of daily/online frontends, which may belong to a different client generation.
func (s *Source) RoutineWeekly(index catalog.ItemIndex) (*activity.RoutineRules, error) {
	if s == nil || s.archive == nil || index.Source.Checksum != s.Snapshot().Checksum {
		return nil, fmt.Errorf("weekly source/index mismatch")
	}
	tokens, e := s.archive.Tokens(activity.WeeklyPath)
	if e != nil {
		return nil, e
	}
	w, e := activity.ParseWeekly(tokens)
	if e != nil {
		return nil, e
	}
	out := []activity.RoutineRules{{Kind: "weekly", Table: w.Table, Dates: w.Dates, MinimumLevel: w.MinimumLevel, SecondsPerProperClear: w.SecondsPerProperClear, ClickCumulative: w.ClickCumulative, Weekly: w.Periods, Cumulative: w.Cumulative}}
	if e = s.bindRoutineActivities(out, index); e != nil {
		return nil, e
	}
	return &out[0], nil
}

// RoutineActivities binds all content and mail text to the same selected PVF.
// Preparing these domain services does not enable a protocol without its own
// current-client acceptance evidence.
func (s *Source) RoutineActivities(index catalog.ItemIndex) ([]activity.RoutineRules, error) {
	if s == nil || s.archive == nil || index.Source.Checksum != s.Snapshot().Checksum {
		return nil, fmt.Errorf("routine source/index mismatch")
	}
	wt, e := s.archive.Tokens(activity.WeeklyPath)
	if e != nil {
		return nil, e
	}
	weekly, e := activity.ParseWeekly(wt)
	if e != nil {
		return nil, e
	}
	dt, e := s.archive.Tokens(activity.DailyNativePath)
	if e != nil {
		return nil, e
	}
	daily, e := activity.ParseDailyNative(dt)
	if e != nil {
		return nil, e
	}
	if daily.MissionType != 0 || daily.TodayBonus != 0 || daily.ProperDungeonCounts[0] != daily.ProperDungeonCounts[1] {
		return nil, fmt.Errorf("unverified daily mission/bonus branch")
	}
	onlineRules, e := s.routineOnline(index, false)
	if e != nil {
		return nil, e
	}
	out := []activity.RoutineRules{
		{Kind: "weekly", Table: weekly.Table, Dates: weekly.Dates, MinimumLevel: weekly.MinimumLevel, SecondsPerProperClear: weekly.SecondsPerProperClear, ClickCumulative: weekly.ClickCumulative, Weekly: weekly.Periods, Cumulative: weekly.Cumulative},
		{Kind: "daily", Table: daily.Table, Dates: daily.Dates, MinimumLevel: daily.MinimumLevel, ProperDungeonCount: daily.ProperDungeonCounts[0], Daily: daily.Days, Cumulative: daily.Cumulative},
		*onlineRules,
	}
	if e = s.bindRoutineActivities(out, index); e != nil {
		return nil, e
	}
	return out, nil
}

func (s *Source) bindRoutineActivities(out []activity.RoutineRules, index catalog.ItemIndex) error {
	texts := map[string]string{}
	wanted := map[string]bool{}
	for _, c := range out {
		for _, r := range c.Rewards() {
			wanted[r.MailTitle] = true
			wanted[r.MailMessage] = true
		}
	}
	tables, e := s.archive.StringTables()
	if e != nil {
		return e
	}
	seen := map[string]bool{}
	for _, path := range tables {
		if seen[path] {
			continue
		}
		seen[path] = true
		text, e := s.archive.ReadText(path)
		if e != nil {
			return e
		}
		for _, line := range strings.Split(text, "\n") {
			key, value, ok := strings.Cut(strings.TrimRight(line, "\r"), ">")
			key = strings.TrimPrefix(strings.TrimSpace(key), "\ufeff")
			if !ok || !wanted[key] {
				continue
			}
			if old, ok := texts[key]; ok && old != value {
				return fmt.Errorf("ambiguous routine mail key %s", key)
			}
			texts[key] = value
		}
	}
	names := map[uint32]string{}
	bind := func(r *activity.Reward) error {
		item, ok := index.Items[r.Item]
		if !ok || item.Kind != "stackable" {
			return fmt.Errorf("routine material %d missing", r.Item)
		}
		if _, ok := s.archive.FindFile(item.Path); !ok {
			return fmt.Errorf("routine material source missing")
		}
		if _, exists := names[r.Item]; !exists {
			name, found, err := s.archive.LocalizedNameByPath(item.Path)
			if err != nil || !found || name == "" {
				return fmt.Errorf("routine item %d name missing: %v", r.Item, err)
			}
			names[r.Item] = name
		}
		r.MailTitle, r.MailMessage = texts[r.MailTitle], texts[r.MailMessage]
		if r.MailTitle == "" || r.MailMessage == "" {
			return fmt.Errorf("routine mail text missing")
		}
		return nil
	}
	for i := range out {
		c := &out[i]
		c.SourceChecksum = s.Snapshot().Checksum
		c.ItemNames = names
		for _, rows := range [][]activity.Milestone{c.Weekly, c.Daily, c.Cumulative} {
			for j := range rows {
				if e = bind(&rows[j].Reward); e != nil {
					return e
				}
			}
		}
		for _, rows := range c.OnlineDaily {
			for j := range rows {
				if e = bind(&rows[j].Reward); e != nil {
					return e
				}
			}
		}
		for j := range c.OnlinePeriod {
			if e = bind(&c.OnlinePeriod[j].Reward); e != nil {
				return e
			}
		}
		if e = c.Validate(); e != nil {
			return e
		}
	}
	return nil
}

// RoutineOnline is an explicitly paired local controller, never a HoneyTime
// native packet projection. Legacy native-only branch flags remain unmodified.
func (s *Source) RoutineOnline(index catalog.ItemIndex) (*activity.RoutineRules, error) {
	return s.routineOnline(index, true)
}
func labelOnlineExtension(t []pvf.Token) (string, error) {
	for i, c := range t {
		if c.Type == 3 && c.Text == "[local extension controller]" && i+1 < len(t) && t[i+1].Type == 6 {
			return t[i+1].Text, nil
		}
	}
	return "", fmt.Errorf("missing extension contract")
}
func (s *Source) routineOnline(index catalog.ItemIndex, extension bool) (*activity.RoutineRules, error) {
	if s == nil || s.archive == nil || index.Source.Checksum != s.Snapshot().Checksum {
		return nil, fmt.Errorf("online source/index mismatch")
	}
	ot, e := s.archive.Tokens(activity.OnlinePath)
	if e != nil {
		return nil, e
	}
	if extension {
		v, e := labelOnlineExtension(ot)
		if e != nil || v != "online-v1" {
			return nil, fmt.Errorf("source is not an owned online extension")
		}
	}
	online, e := activity.ParseOnline(ot)
	if e != nil {
		return nil, e
	}
	if online.PCRoom || online.DailyAutoget || online.DailyLoop != 1 || online.ExtendedDailyLoop != 0 || len(online.ExtendedRewards) != 0 || len(online.SpecifiedRewards) != 0 {
		return nil, fmt.Errorf("unsupported online campaign branch")
	}
	// The local material adaptation supplies its own mail metadata in its source,
	// since native HoneyTime rewards do not carry per-row mail keys.
	find := func(tag string) (string, error) {
		for i, t := range ot {
			if t.Type == 3 && t.Text == tag && i+1 < len(ot) && ot[i+1].Type == 6 {
				return ot[i+1].Text, nil
			}
		}
		return "", fmt.Errorf("missing local source mail tag %s", tag)
	}
	title, e := find("[local reward mail title]")
	if e != nil {
		return nil, e
	}
	message, e := find("[local reward mail message]")
	if e != nil {
		return nil, e
	}
	for k, rows := range online.Daily {
		for i := range rows {
			rows[i].Reward.MailTitle = title
			rows[i].Reward.MailMessage = message
		}
		online.Daily[k] = rows
	}
	for i := range online.Period {
		online.Period[i].Reward.MailTitle = title
		online.Period[i].Reward.MailMessage = message
	}
	out := []activity.RoutineRules{{Kind: "online", Table: online.Table, Dates: online.Dates, OnlineDaily: online.Daily, OnlineMaximum: online.DailyMaximumSeconds, OnlinePeriod: online.Period}}
	if e = s.bindRoutineActivities(out, index); e != nil {
		return nil, e
	}
	return &out[0], nil
}
