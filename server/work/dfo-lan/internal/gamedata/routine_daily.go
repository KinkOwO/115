package gamedata

import (
	"dfolan/internal/activity"
	"dfolan/internal/catalog"
	"fmt"
)

// RoutineDaily binds the owned sequential adaptation. CN TodayBonus and the
// native US dated-calendar grammar are never silently given this meaning.
func (s *Source) RoutineDaily(index catalog.ItemIndex) (*activity.RoutineRules, error) {
	if s == nil || s.archive == nil || index.Source.Checksum != s.Snapshot().Checksum {
		return nil, fmt.Errorf("daily source/index mismatch")
	}
	t, e := s.archive.Tokens(activity.DailyPath)
	if e != nil {
		return nil, e
	}
	contract, e := labelOnlineExtension(t)
	if e != nil || contract != "daily-sequence-v1" {
		return nil, fmt.Errorf("source is not an owned daily extension")
	}
	d, e := activity.ParseDaily(t)
	if e != nil {
		return nil, e
	}
	if d.Mode != "integrate" || d.TodayBonus != 0 || d.MissionType != 0 || len(d.ProperDungeonCounts) != 2 || d.ProperDungeonCounts[0] != d.ProperDungeonCounts[1] || !d.UseCumulative {
		return nil, fmt.Errorf("unsupported daily extension branch")
	}
	out := []activity.RoutineRules{{Kind: "daily", Table: d.Table, Dates: d.Dates, MinimumLevel: d.MinimumLevel, ProperDungeonCount: d.ProperDungeonCounts[0], SequentialDaily: true, Daily: d.Days, Cumulative: d.Cumulative}}
	if e = s.bindRoutineActivities(out, index); e != nil {
		return nil, e
	}
	return &out[0], nil
}
