package workflow

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/database"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

func (s *AttendanceService) RestoreMissingProgress(ctx context.Context, role database.Character, now time.Time, gameDay string) error {
	if err := s.validate(); err != nil {
		return err
	}
	campaign := s.Rules.CampaignKey()
	return s.Store.RecoverMissingAttendance(ctx, role.AccountID, role.ID, campaign, "attendance-", func(rows []database.AttendanceEvidence) (json.RawMessage, error) {
		p := activity.LoginProgress{Claimed: map[uint32]bool{}}
		days := map[string]bool{}
		loginPrefix, claimPrefix := "attendance-login:"+campaign+":", "attendance-claim:"+campaign+":"
		for _, row := range rows {
			if strings.HasPrefix(row.Key, loginPrefix) {
				day := strings.TrimPrefix(row.Key, loginPrefix)
				parsed, err := time.ParseInLocation("2006-01-02", day, now.Location())
				if err != nil || day > gameDay {
					continue
				}
				start, _, err := dateWindowRecovery(s.Rules.Dates, now.Location())
				if err != nil {
					return nil, err
				}
				if parsed.Add(24 * time.Hour).Before(start) {
					continue
				}
				days[day] = true
				if day > p.LastDay {
					p.LastDay = day
				}
			}
			if strings.HasPrefix(row.Key, claimPrefix) {
				var proof struct {
					MailID int64 `json:"mail_id"`
				}
				if err := json.Unmarshal(row.Outcome, &proof); err != nil {
					return nil, err
				}
				if proof.MailID <= 0 {
					continue
				}
				index, err := strconv.ParseUint(strings.TrimPrefix(row.Key, claimPrefix), 10, 32)
				if err != nil || index >= uint64(len(s.Rules.Days)) {
					return nil, fmt.Errorf("invalid attendance claim evidence")
				}
				p.Claimed[uint32(index)] = true
				if uint32(index)+1 > p.Visits {
					p.Visits = uint32(index) + 1
				}
			}
		}
		if len(days) == 0 && len(p.Claimed) == 0 {
			return nil, nil
		}
		if uint32(len(days)) > p.Visits {
			p.Visits = uint32(len(days))
		}
		if p.Visits > uint32(len(s.Rules.Days)) {
			p.Visits = uint32(len(s.Rules.Days))
		}
		// A claim proves an unlocked day; without a dated observation the safe
		// last-login boundary is today, rather than inventing an extra visit.
		if p.LastDay == "" {
			p.LastDay = gameDay
		}
		p.Version = 1
		if _, _, err := s.Rules.Observe(p, now, gameDay, s.Rules.MinimumLevel); err != nil {
			return nil, err
		}
		return json.Marshal(p)
	})
}

func dateWindowRecovery(d activity.Dates, loc *time.Location) (time.Time, time.Time, error) {
	start, err := time.ParseInLocation("2006-01-02 15:04:05", d.Start, loc)
	if err != nil {
		return start, time.Time{}, err
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", d.End, loc)
	return start, end, err
}

func (s *RoutineService) RestoreMissingWeeklyProgress(ctx context.Context, role database.Character, now time.Time, resetHour int) error {
	if err := s.validate(); err != nil {
		return err
	}
	if s.Rules.Kind != "weekly" {
		return fmt.Errorf("weekly recovery requires weekly rules")
	}
	campaign := s.Rules.CampaignKey()
	prefix := "routine:" + campaign + ":"
	return s.Store.RecoverMissingAttendance(ctx, role.AccountID, role.ID, campaign, prefix, func(rows []database.AttendanceEvidence) (json.RawMessage, error) {
		p := activity.RoutineProgress{Version: 1, HighWater: now, Day: now.Add(-time.Duration(resetHour) * time.Hour).Format("2006-01-02"), PeriodMillis: map[uint32]uint64{}, Claimed: map[string]bool{}}
		for _, row := range rows {
			var proof struct {
				MailID int64 `json:"mail_id"`
			}
			if err := json.Unmarshal(row.Outcome, &proof); err != nil {
				return nil, err
			}
			if proof.MailID <= 0 {
				continue
			}
			parts := strings.Split(row.Key, ":")
			if len(parts) < 4 {
				continue
			}
			kind := parts[len(parts)-2]
			if kind != "period" && kind != "cumulative" {
				continue
			}
			index64, err := strconv.ParseUint(parts[len(parts)-1], 10, 32)
			if err != nil {
				return nil, err
			}
			index := uint32(index64)
			sourceRows := s.Rules.Weekly
			if kind == "cumulative" {
				sourceRows = s.Rules.Cumulative
			}
			found := false
			for _, r := range sourceRows {
				if r.Index == index {
					found = true
					p.Claimed[kind+":"+strconv.FormatUint(index64, 10)] = true
					// A paid period receipt proves its source threshold was met.
					if kind == "period" {
						p.PeriodMillis[index] = uint64(r.Required) * 1000
					}
				}
			}
			if !found {
				return nil, fmt.Errorf("weekly receipt is absent from selected rules")
			}
		}
		if len(p.Claimed) == 0 {
			return nil, nil
		}
		if err := s.Rules.ValidateProgress(p, now.Location(), resetHour); err != nil {
			return nil, err
		}
		return json.Marshal(p)
	})
}
