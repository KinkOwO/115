package workflow

import (
	"dfolan/internal/activity"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// BindCommittedClears is called once at startup for the selected weekly source.
// Bonus progress and the clear receipt commit or roll back together; a later
// status packet or connection loss cannot lose or duplicate the source bonus.
func (s *RoutineService) BindCommittedClears(location *time.Location, resetHour int) error {
	if err := s.validate(); err != nil {
		return err
	}
	if s.Rules.Kind != "weekly" || location == nil || resetHour < 0 || resetHour > 23 {
		return fmt.Errorf("weekly clear observer requires selected source and calendar")
	}
	return s.Store.RegisterCharacterEventAccountObserver(func(role database.Character, key string, outcome json.RawMessage, profile *database.AccountAdventure) (bool, error) {
		if !strings.HasPrefix(key, "clear:") {
			return false, nil
		}
		var receipt character.ClearReceipt
		if err := json.Unmarshal(outcome, &receipt); err != nil {
			return false, err
		}
		if !receipt.RecommendedDungeonClear {
			return false, nil
		}
		if receipt.Source != s.Items.Source.SaveIdentity() || receipt.Source != role.ConfigVersion || receipt.Run == "" || key != "clear:"+receipt.Run || receipt.DungeonID == 0 || receipt.CompletedAt.IsZero() {
			return false, fmt.Errorf("weekly clear requires owned, dated source receipt")
		}
		if uint32(receipt.CharacterLevel) < s.Rules.MinimumLevel {
			return false, nil
		}
		now := receipt.CompletedAt.In(location)
		campaign := s.Rules.CampaignKey()
		progress, err := routineProgress(*profile, campaign)
		if err != nil {
			return false, err
		}
		// Validate activity dates/level without advancing the connected-time high
		// water mark: the gateway's next tick must still credit the pending interval.
		probe := activity.RoutineProgress{}
		day := now.Add(-time.Duration(resetHour) * time.Hour).Format("2006-01-02")
		_, err = s.Rules.Observe(probe, now, now, day, resetHour, uint32(receipt.CharacterLevel))
		if errors.Is(err, activity.ErrRoutineInactive) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if err = s.Rules.ValidateProgress(progress, location, resetHour); err != nil {
			return false, err
		}
		if progress.HighWater.IsZero() {
			progress, err = s.Rules.Observe(progress, now, now, day, resetHour, uint32(receipt.CharacterLevel))
			if err != nil {
				return false, err
			}
		}
		progress = s.Rules.ProperClear(progress, now)
		if err = s.Rules.ValidateProgress(progress, location, resetHour); err != nil {
			return false, err
		}
		raw, err := mergeRoutineProgress(profile.Data.Attendance[campaign], progress)
		if err != nil {
			return false, err
		}
		if profile.Data.Attendance == nil {
			profile.Data.Attendance = map[string]json.RawMessage{}
		}
		profile.Data.Attendance[campaign] = raw
		return true, nil
	})
}
