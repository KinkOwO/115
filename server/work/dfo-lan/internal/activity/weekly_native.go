package activity

import (
	"fmt"
	"time"
)

// WeeklyNativeView describes the fields consumed by the US actor, without
// duplicating source goals. Cumulative eligibility is computed by the client
// from ClaimedPeriods and [accu reward info], not carried in another mask.
type WeeklyNativeView struct {
	CurrentPeriod                                      byte
	Seconds                                            uint32
	EligiblePeriods, ClaimedPeriods, ClaimedCumulative uint32
}

func (c RoutineRules) WeeklyNativeView(p RoutineProgress, now time.Time, resetHour int) (WeeklyNativeView, error) {
	var out WeeklyNativeView
	if err := c.Validate(); err != nil {
		return out, err
	}
	if c.Kind != "weekly" {
		return out, fmt.Errorf("native weekly view requires weekly source")
	}
	if err := c.ValidateProgress(p, now.Location(), resetHour); err != nil {
		return out, err
	}
	for _, row := range c.Weekly {
		start, end, err := dateWindow(row.Dates, now.Location())
		if err != nil {
			return out, err
		}
		if !now.Before(start) && now.Before(end) {
			out.CurrentPeriod = byte(row.Index)
			out.Seconds = uint32(p.PeriodMillis[row.Index] / 1000)
		}
		if p.PeriodMillis[row.Index] >= uint64(row.Required)*1000 {
			out.EligiblePeriods |= uint32(1) << row.Index
		}
		if p.Claimed[rowKey("period", row.Index)] {
			out.ClaimedPeriods |= uint32(1) << row.Index
		}
	}
	for _, row := range c.Cumulative {
		// The parser stores the source count as the native claim identity.
		if row.Index != row.Required || row.Index > 31 {
			return out, fmt.Errorf("weekly cumulative identity is not native")
		}
		if p.Claimed[rowKey("cumulative", row.Index)] {
			out.ClaimedCumulative |= uint32(1) << row.Index
		}
	}
	if out.CurrentPeriod == 0 {
		return out, ErrRoutineInactive
	}
	return out, nil
}
