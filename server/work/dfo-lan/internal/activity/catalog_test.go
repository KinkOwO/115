package activity

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"testing"
)

func fixture(t *testing.T, name string) []pvf.Token {
	t.Helper()
	raw, e := os.ReadFile("testdata/" + name + ".json")
	if e != nil {
		t.Fatal(e)
	}
	var cells []pvf.Token
	if e = json.Unmarshal(raw, &cells); e != nil {
		t.Fatal(e)
	}
	return cells
}
func TestCurrentCNRules(t *testing.T) {
	l, err := ParseLogin(fixture(t, "login"))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Days) != 8 || len(l.Days[0].Rewards) != 2 || l.Days[0].Rewards[1].Count != 11 || !l.Days[6].Special {
		t.Fatalf("login: %+v", l)
	}
	w, e := ParseWeekly(fixture(t, "weekly"))
	if e != nil {
		t.Fatal(e)
	}
	if len(w.Periods) != 5 || w.Periods[0].Required != 3600 || w.SecondsPerProperClear != 60 || w.Cumulative[1].Reward.Count != 3 {
		t.Fatalf("weekly: %+v", w)
	}
	o, e := ParseOnline(fixture(t, "online"))
	if e != nil {
		t.Fatal(e)
	}
	if o.Daily["weekday"][3].Reward.Item == o.Daily["weekend"][3].Reward.Item || o.DailyMaximumSeconds["weekend"] != 7200 || o.Period[3].Milliseconds != 28800000 || o.PeriodUIHours[3] != 32 {
		t.Fatalf("online: %+v", o)
	}
	d, e := ParseDaily(fixture(t, "daily"))
	if e != nil {
		t.Fatal(e)
	}
	if len(d.Days) != 10 || d.Cumulative[2].Required != 7 || d.TodayBonus != 2 || d.ProperDungeonCounts[0] != 4 {
		t.Fatalf("daily: %+v", d)
	}
	// Source anomalies remain visible; a parser must not silently invent a date.
	if d.UIDateStart != "2025-09-30 06:00:00" {
		t.Fatal("stale source calendar was silently rewritten")
	}
}

func TestCurrentUSWeeklyFormat(t *testing.T) {
	c, err := ParseWeekly(fixture(t, "weekly_us"))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Periods) != 11 || c.Periods[0].Dates.Start != "2099-01-17 09:00:00" || c.Periods[0].Required != 7200 || len(c.Cumulative) != 3 {
		t.Fatalf("US weekly: %+v", c)
	}
	if c.Cumulative[0].Reward.Item != 590005968 || c.Cumulative[0].Required != 2 {
		t.Fatal("US cumulative rule changed")
	}
}
func TestRulesFollowChangedSource(t *testing.T) {
	w := fixture(t, "weekly")
	for i, v := range w {
		if v.Type == 3 && v.Text == "[keeping connection info as period]" {
			w[i+2].Value = 120
			w[i+4].Value = 9
			break
		}
	}
	c, e := ParseWeekly(w)
	if e != nil {
		t.Fatal(e)
	}
	if c.Periods[0].Required != 120 || c.Periods[0].Reward.Count != 9 {
		t.Fatal("weekly ignored source changes")
	}
	o := fixture(t, "online")
	for i, v := range o {
		if v.Type == 3 && v.Text == "[daily Accumulated Time]" {
			o[i+4].Value = 9000
			break
		}
	}
	r, e := ParseOnline(o)
	if e != nil {
		t.Fatal(e)
	}
	if r.DailyMaximumSeconds["weekend"] != 9000 {
		t.Fatal("online ignored source cap")
	}
	d := fixture(t, "daily")
	for i, v := range d {
		if v.Type == 3 && v.Text == "[proper dungeon count]" {
			d[i+1].Value = 6
			break
		}
	}
	a, e := ParseDaily(d)
	if e != nil {
		t.Fatal(e)
	}
	if a.ProperDungeonCounts[0] != 6 {
		t.Fatal("daily ignored source condition")
	}
}
func TestMalformedRulesFailClosed(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  func([]pvf.Token) []pvf.Token
		parse func([]pvf.Token) error
	}{
		{"weekly", func(c []pvf.Token) []pvf.Token {
			for i, v := range c {
				if v.Type == 3 && v.Text == "[keeping connection info as period]" {
					c[i+4].Value = 0
					break
				}
			}
			return c
		}, func(c []pvf.Token) error { _, e := ParseWeekly(c); return e }},
		{"online", func(c []pvf.Token) []pvf.Token {
			for i, v := range c {
				if v.Type == 3 && v.Text == "[week type]" {
					c[i+1].Text = "unknown"
					break
				}
			}
			return c
		}, func(c []pvf.Token) error { _, e := ParseOnline(c); return e }},
		{"daily", func(c []pvf.Token) []pvf.Token {
			for i, v := range c {
				if v.Type == 3 && v.Text == "[daily reward]" {
					return append(c[:i+3], c[i+4:]...)
				}
			}
			return c
		}, func(c []pvf.Token) error { _, e := ParseDaily(c); return e }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if e := tc.parse(tc.edit(fixture(t, tc.name))); e == nil {
				t.Fatal("accepted invalid source")
			}
		})
	}
}

type memorySource map[string][]pvf.Token

func (s memorySource) Tokens(p string) ([]pvf.Token, error) {
	if c, ok := s[p]; ok {
		return c, nil
	}
	return nil, os.ErrNotExist
}
func TestLoadRequiresAllSourceFiles(t *testing.T) {
	s := memorySource{WeeklyPath: fixture(t, "weekly"), OnlinePath: fixture(t, "online"), DailyPath: fixture(t, "daily"), LoginPath: fixture(t, "login")}
	c, e := Load(s)
	if e != nil {
		t.Fatal(e)
	}
	if len(c.RewardItems()) != 29 {
		t.Fatalf("reward roots=%d", len(c.RewardItems()))
	}
	delete(s, DailyPath)
	if _, e = Load(s); e == nil {
		t.Fatal("missing daily source accepted")
	}
}
