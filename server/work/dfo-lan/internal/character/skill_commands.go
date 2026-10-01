package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

// SaveSkillCommands replaces the selected character's CMD331 snapshot. It
// updates the existing JSON state under the character row lock, so old saves
// and unrelated state fields remain intact.
func (s *Service) SaveSkillCommands(ctx context.Context, role Character, key string, req protocol.SkillCommands) (Character, bool, error) {
	if s.Store == nil || role.ID == 0 || role.AccountID == 0 {
		return role, false, fmt.Errorf("skill commands require an owned selected character")
	}
	if _, err := protocol.DecodeSkillCommands(req.Raw); err != nil {
		return role, false, err
	}
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "skill-commands-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
		var state State
		if err := json.Unmarshal(current.State, &state); err != nil {
			return nil, nil, err
		}
		state.SkillCommands = append([]byte(nil), req.Raw...)
		next, err := mergeSkillState(current.State, state)
		if err != nil {
			return nil, nil, err
		}
		return next, json.RawMessage(`{}`), nil
	})
}
