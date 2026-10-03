package character

import (
	"context"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

// SaveComboSkillInfo records the CMD500 combo-extension arrangement.
//
// The client pushes ENUM_CMDPACKET_COMBO_SKILL_INFO every time the skill window
// closes and after every edit of the learn page's combo cells, and it expects
// nothing back: the nine retained bodies of 2026-09-26/27 all went out with no
// response and no retry. Until this handler existed the body was dropped as an
// unimplemented sample, so the arrangement only ever lived in the client's
// in-process character cache and was gone the moment the client rebuilt the
// character from NOTI19 - which is exactly the reported "the ASDFGH row I set
// up in the learn page comes back empty after I re-select the character".
func (s *Service) SaveComboSkillInfo(ctx context.Context, role Character, key string, req protocol.ComboSkillInfo) (Character, bool, error) {
	if s.Store == nil || role.ID == 0 || role.AccountID == 0 || role.Profession != 9 {
		return role, false, fmt.Errorf("combo skill info requires an owned selected character")
	}
	raw := req.Raw
	if len(raw) == 0 {
		var err error
		raw, err = protocol.EncodeComboSkillInfo(req)
		if err != nil {
			return role, false, err
		}
	}
	if _, err := protocol.DecodeComboSkillInfo(raw); err != nil {
		return role, false, err
	}
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "combo-skill-info-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
		if current.Profession != 9 {
			return nil, nil, fmt.Errorf("combo skill info requires dark knight")
		}
		var state State
		if err := json.Unmarshal(current.State, &state); err != nil {
			return nil, nil, err
		}
		state.ComboSkillInfo = append([]byte(nil), raw...)
		next, err := mergeSkillState(current.State, state)
		if err != nil {
			return nil, nil, err
		}
		return next, json.RawMessage(`{}`), nil
	})
}

// ClearComboSkillInfo is CMD502
// ENUM_CMDPACKET_COMBO_SKILL_EXTENSION_QUICK_SLOT_RESET: the parameterless
// frame the client sends from the skill window to drop every combo-extension
// assignment. It writes an empty body, which entry then skips.
func (s *Service) ClearComboSkillInfo(ctx context.Context, role Character, key string) (Character, bool, error) {
	if s.Store == nil || role.ID == 0 || role.AccountID == 0 || role.Profession != 9 {
		return role, false, fmt.Errorf("combo skill info requires an owned selected character")
	}
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, key, "combo-skill-info-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
		if current.Profession != 9 {
			return nil, nil, fmt.Errorf("combo skill info requires dark knight")
		}
		var state State
		if err := json.Unmarshal(current.State, &state); err != nil {
			return nil, nil, err
		}
		state.ComboSkillInfo = nil
		next, err := mergeSkillState(current.State, state)
		if err != nil {
			return nil, nil, err
		}
		return next, json.RawMessage(`{}`), nil
	})
}

// EntryComboSkillInfo returns the stored CMD500 body. The caller must convert
// it with ComboSkillInfoNotify before sending NOTI433. Missing or undecodable
// snapshots are skipped so older saves can still enter the world.
func (s *Service) EntryComboSkillInfo(role Character) ([]byte, error) {
	if role.Profession != 9 || len(role.State) == 0 {
		return nil, nil
	}
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, err
	}
	if len(state.ComboSkillInfo) == 0 {
		return nil, nil
	}
	if _, err := protocol.DecodeComboSkillInfo(state.ComboSkillInfo); err != nil {
		// A body that no longer decodes (older/corrupt save) must not take the
		// whole entry down; drop it and let the client re-push.
		return nil, nil
	}
	return append([]byte(nil), state.ComboSkillInfo...), nil
}

// ComboSkillInfoNotify converts stored C2S cells to the independent NOTI433
// layout. Never replay a CMD500 body verbatim as a notification.
func (s *Service) ComboSkillInfoNotify(role Character) ([]byte, error) {
	raw, err := s.EntryComboSkillInfo(role)
	if err != nil || len(raw) == 0 {
		return nil, err
	}
	info, err := protocol.DecodeComboSkillInfo(raw)
	if err != nil {
		return nil, err
	}
	return protocol.EncodeComboSkillInfoNotify(info)
}
