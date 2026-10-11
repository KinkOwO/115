package main

import (
	"dfolan/internal/character"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The launcher supplies an absolute path because its historical server cwd is
// outside this module. Direct module invocations retain the existing default.
func attendanceCalendarPath(fatigueRules string) string {
	if path := strings.TrimSpace(os.Getenv("DFO_ATTENDANCE_CALENDAR")); path != "" {
		return path
	}
	if override := os.Getenv("DFO_FATIGUE_RULES"); override != "" {
		fatigueRules = override
	}
	if fatigueRules == "" {
		fatigueRules = "configs/fatigue-probe.json"
	}
	return filepath.Join(filepath.Dir(fatigueRules), "attendance-calendar.json")
}

// The attendance calendar is independent of the existing fatigue reset policy.
func loadAttendanceClock(path string) (*character.FatigueService, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p struct {
		Timezone  string `json:"timezone"`
		ResetHour int    `json:"reset_hour"`
		Source    string `json:"source"`
	}
	if err = json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	if p.Timezone == "" || p.ResetHour < 0 || p.ResetHour > 23 || p.Source == "" {
		return nil, fmt.Errorf("invalid attendance calendar")
	}
	loc, err := time.LoadLocation(p.Timezone)
	if err != nil {
		return nil, err
	}
	return &character.FatigueService{Location: loc, Rules: character.FatigueRules{Timezone: p.Timezone, ResetHour: p.ResetHour, Source: p.Source}}, nil
}

func (w *worldSession) attendanceClock() *character.FatigueService {
	if w.attendanceCalendar != nil {
		return w.attendanceCalendar
	}
	return w.fatigue // Retains the explicit calendar in isolated protocol fixtures.
}
