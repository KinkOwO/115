package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

func (s *Service) SaveSkillPreset(ctx context.Context, role Character, key string, preset protocol.SkillPreset) (Character, bool, error) {
	if s.Store == nil || role.ID == 0 || role.AccountID == 0 {
		return role, false, fmt.Errorf("skill preset requires an owned selected character")
	}
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "skill-preset-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
		var state State
		if err := json.Unmarshal(current.State, &state); err != nil {
			return nil, nil, err
		}
		state.SkillPreset = append([]byte(nil), preset.Config[:]...)
		next, err := mergeSkillState(current.State, state)
		if err != nil {
			return nil, nil, err
		}
		return next, json.RawMessage(`{}`), nil
	})
}

func (s *Service) SkillPresetInfo(role Character) ([]byte, error) {
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, err
	}
	if len(state.SkillPreset) == 0 {
		return nil, nil
	}
	if len(state.SkillPreset) != protocol.SkillPresetConfigBytes {
		return nil, fmt.Errorf("invalid saved skill preset size")
	}
	var preset protocol.SkillPreset
	copy(preset.Config[:], state.SkillPreset)
	return protocol.SkillPresetInfo(preset), nil
}
