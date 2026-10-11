package gamedata_test

import (
	"dfolan/internal/activity"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"encoding/json"
	"os"
	"testing"
)

// Explicit read-only archive: never discover/open the player's database.
func TestAttendanceActivitiesNativeSource(t *testing.T) {
	path := os.Getenv("US115_TEST_ACTIVITY_PVF")
	if path == "" {
		t.Skip("set US115_TEST_ACTIVITY_PVF to the explicit source candidate")
	}
	s, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: path, MaxBytes: gamedata.DefaultMaxBytes})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	index, err := s.ItemIndex("")
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.AttendanceActivities(index)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Login.Days) != 8 || len(c.Weekly.Periods) != 5 || len(c.Daily.Days) != 10 || len(c.Online.Period) != 4 {
		t.Fatal("unexpected native activity content")
	}
	login, err := s.AttendanceLogin(index)
	if err != nil {
		t.Fatal(err)
	}
	if login.SourceChecksum != s.Snapshot().Checksum || login.MailTextTitle == "" || login.MailTextMessage == "" {
		t.Fatal("login source/text binding missing")
	}
	index.Source.Checksum = "different"
	if _, err = s.AttendanceLogin(index); err == nil {
		t.Fatal("mixed sources accepted")
	}
	t.Logf("source=%s rewards=%d login=%d weekly=%d daily=%d online=%d", s.Snapshot().Checksum, len(c.RewardItems()), len(c.Login.Days), len(c.Weekly.Periods), len(c.Daily.Days), len(c.Online.Period))
	if adapted := os.Getenv("US115_TEST_ACTIVITY_WEEKLY_ADAPTED"); adapted != "" {
		raw, e := os.ReadFile(adapted)
		if e != nil {
			t.Fatal(e)
		}
		var cells []pvf.Token
		if e = json.Unmarshal(raw, &cells); e != nil {
			t.Fatal(e)
		}
		w, e := activity.ParseWeekly(cells)
		if e != nil {
			t.Fatal(e)
		}
		if len(w.Periods) != len(c.Weekly.Periods) || len(w.Cumulative) != len(c.Weekly.Cumulative) {
			t.Fatal("weekly adaptation lost rows")
		}
		for i, row := range w.Periods {
			src := c.Weekly.Periods[i]
			if row.Required != src.Required || row.Reward.Item != src.Reward.Item || row.Reward.Count != src.Reward.Count || row.Dates.Start == "" {
				t.Fatal("weekly adaptation changed gameplay content")
			}
		}
		for i, row := range w.Cumulative {
			src := c.Weekly.Cumulative[i]
			if row.Required != src.Required || row.Reward.Item != src.Reward.Item || row.Reward.Count != src.Reward.Count {
				t.Fatal("weekly adaptation changed cumulative reward")
			}
		}
	}
}
