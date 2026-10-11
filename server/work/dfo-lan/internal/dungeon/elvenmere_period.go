package dungeon

import "time"

// Weekly progress/rewards reset Tuesday 09:00 UTC, independently of seasons.
// Source: https://www.dfoneople.com/news/updates/2979/Dungeons/Elvenmere
// This server-owned limit does not use a new packet field or client clock.
func ElvenmereWeekStart(now time.Time) int64 {
	utc := now.UTC()
	day := time.Date(utc.Year(), utc.Month(), utc.Day(), 9, 0, 0, 0, time.UTC)
	delta := (int(day.Weekday()) - int(time.Tuesday) + 7) % 7
	start := day.AddDate(0, 0, -delta)
	if utc.Before(start) {
		start = start.AddDate(0, 0, -7)
	}
	return start.Unix()
}

// Migration namespace preserving the existing ongoing season, not an invented
// upstream season number or date. It changes only through an audited reset.
const ElvenmereLegacySeason = "legacy-preserved"
