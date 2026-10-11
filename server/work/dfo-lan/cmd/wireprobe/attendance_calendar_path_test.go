package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAttendanceCalendarExplicitPathRetainsReleaseClock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "签到配置.json")
	if err := os.WriteFile(path, []byte(`{"timezone":"Asia/Hong_Kong","reset_hour":6,"source":"release-calendar"}`), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DFO_ATTENDANCE_CALENDAR", path)
	t.Setenv("DFO_FATIGUE_RULES", "stale-other-installation.json")
	clock, err := loadAttendanceClock(attendanceCalendarPath("ignored/fatigue.json"))
	if err != nil || clock.Location.String() != "Asia/Hong_Kong" || clock.Rules.ResetHour != 6 {
		t.Fatalf("release calendar was not loaded: %+v %v", clock, err)
	}
	t.Setenv("DFO_ATTENDANCE_CALENDAR", "")
	t.Setenv("DFO_FATIGUE_RULES", "")
	if got := attendanceCalendarPath(""); got != filepath.Join("configs", "attendance-calendar.json") {
		t.Fatalf("direct module default changed: %s", got)
	}
}
