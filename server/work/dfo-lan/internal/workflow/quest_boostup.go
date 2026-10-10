package workflow

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/database"
	"dfolan/internal/quest"
	"encoding/json"
	"fmt"
)

// ReconcileBoostMainline is retried after the final training claim and before
// entry snapshots for older graduated saves. SkipAllNow supplies the same native
// source exclusions, profession gates, atomic settlement and one-time key items
// as the manual mainline button. It never reruns the training rewards.
func (s *QuestService) ReconcileBoostMainline(ctx context.Context, role database.Character, content *boostup.Catalog) (quest.SkipResult, error) {
	out := quest.SkipResult{Role: role}
	if content == nil {
		return out, nil
	}
	state, err := boostup.ReadState(role.State)
	if err != nil {
		return out, err
	}
	if !state.Activated || !state.Training.Finished {
		return out, nil
	}
	var base struct {
		Level byte `json:"level"`
	}
	if err = json.Unmarshal(role.State, &base); err != nil {
		return out, err
	}
	if content.GoalLevel == 0 || base.Level < content.GoalLevel {
		return out, fmt.Errorf("boost graduation level differs from source goal")
	}
	out, err = s.SkipMainlineNow(ctx, role, "boostup-graduation", "source training completed", 0)
	if err == nil {
		out.Role.WireID = role.WireID
	}
	return out, err
}
