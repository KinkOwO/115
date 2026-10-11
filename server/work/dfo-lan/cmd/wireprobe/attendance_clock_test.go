package main

import (
	"dfolan/internal/character"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAttendanceCalendarLoadsFromInstalledRulesWithServerWorkingDirectory(t *testing.T) {
	t.Setenv("DFO_ATTENDANCE_CALENDAR", "")
	root := t.TempDir()
	configDir := filepath.Join(root, "server", "work", "dfo-lan", "configs")
	require.NoError(t, os.MkdirAll(configDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(configDir, "attendance-calendar.json"), []byte(`{"timezone":"Asia/Hong_Kong","reset_hour":6,"source":"installed attendance calendar"}`), 0600))
	t.Chdir(filepath.Join(root, "server"))
	t.Setenv("DFO_FATIGUE_RULES", "")
	clock, err := loadAttendanceClock(attendanceCalendarPath(filepath.Join(configDir, "fatigue-probe.json")))
	require.NoError(t, err)
	require.Equal(t, "Asia/Hong_Kong", clock.Rules.Timezone)
	require.Equal(t, 6, clock.Rules.ResetHour)
	t.Setenv("DFO_FATIGUE_RULES", filepath.Join(configDir, "custom-fatigue.json"))
	clock, err = loadAttendanceClock(attendanceCalendarPath("configs/fatigue-probe.json"))
	require.NoError(t, err)
	require.Equal(t, 6, clock.Rules.ResetHour)
}

func TestAttendanceCalendarPreservesHongKongSixAMWithoutChangingFatigue(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Hong_Kong")
	require.NoError(t, err)
	fatigue := &character.FatigueService{Location: time.UTC, Rules: character.FatigueRules{Timezone: "UTC", ResetHour: 0}}
	calendar := &character.FatigueService{Location: loc, Rules: character.FatigueRules{Timezone: "Asia/Hong_Kong", ResetHour: 6}}
	w := &worldSession{fatigue: fatigue, attendanceCalendar: calendar}
	boundary := time.Date(2026, 10, 10, 6, 0, 0, 0, loc)
	require.Equal(t, "2026-10-09", w.attendanceClock().Day(boundary.Add(-time.Second)))
	require.Equal(t, "2026-10-10", w.attendanceClock().Day(boundary))
	require.Equal(t, "2026-10-09", fatigue.Day(boundary))
	require.Equal(t, 0, fatigue.Rules.ResetHour)
}
