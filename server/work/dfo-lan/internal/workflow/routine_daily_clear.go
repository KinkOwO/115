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

// BindSequentialClears commits task progress with the existing clear receipt.
// A replay, disconnect or later UI failure cannot add or lose a completed clear.
func (s *RoutineService) BindSequentialClears(loc *time.Location, reset int) error {
	if e := s.validate(); e != nil {
		return e
	}
	if !s.Rules.SequentialDaily || loc == nil || reset < 0 || reset > 23 {
		return fmt.Errorf("sequential daily requires source and calendar")
	}
	return s.Store.RegisterCharacterEventAccountObserver(func(role database.Character, key string, outcome json.RawMessage, p *database.AccountAdventure) (bool, error) {
		if !strings.HasPrefix(key, "clear:") {
			return false, nil
		}
		var receipt character.ClearReceipt
		if e := json.Unmarshal(outcome, &receipt); e != nil {
			return false, e
		}
		if !receipt.RecommendedDungeonClear {
			return false, nil
		}
		if receipt.Source != s.Items.Source.SaveIdentity() || receipt.Source != role.ConfigVersion || receipt.Run == "" || key != "clear:"+receipt.Run || receipt.DungeonID == 0 || receipt.CompletedAt.IsZero() {
			return false, fmt.Errorf("daily clear requires owned dated source receipt")
		}
		if uint32(receipt.CharacterLevel) < s.Rules.MinimumLevel {
			return false, nil
		}
		now := receipt.CompletedAt.In(loc)
		campaign := s.Rules.CampaignKey()
		progress, e := routineProgress(*p, campaign)
		if e != nil {
			return false, e
		}
		day := now.Add(-time.Duration(reset) * time.Hour).Format("2006-01-02")
		progress, e = s.Rules.Observe(progress, now, now, day, reset, uint32(receipt.CharacterLevel))
		if errors.Is(e, activity.ErrRoutineInactive) {
			return false, nil
		}
		if e != nil {
			return false, e
		}
		progress = s.Rules.ProperClear(progress, now)
		if e = s.Rules.ValidateProgress(progress, loc, reset); e != nil {
			return false, e
		}
		raw, e := mergeRoutineProgress(p.Data.Attendance[campaign], progress)
		if e != nil {
			return false, e
		}
		if p.Data.Attendance == nil {
			p.Data.Attendance = map[string]json.RawMessage{}
		}
		p.Data.Attendance[campaign] = raw
		return true, nil
	})
}
