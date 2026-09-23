package character

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

func OdysseyRole(role storage.Character) bool {
	// Graduation wins over every launcher tier: a graduated character is a
	// regular character no matter what DFO_ODYSSEY_MODE says.
	if OdysseyGraduated(role) {
		return false
	}
	if mode := os.Getenv("DFO_ODYSSEY_MODE"); mode != "" {
		return mode == "1"
	}
	return CreatedAsOdyssey(role)
}

// CreatedAsOdyssey reports whether the character itself was created as an Arad
// Odyssey user (creation option[10] = 2), ignoring the DFO_ODYSSEY_MODE debug
// override. Town entry gating uses this: the client decides with the same
// per-character flag (XORSTR "[is arad odyssey user]"), so a launcher-forced
// mode must not make the server apply a different level gate than the one the
// client names in its own refusal message (DSTR 535).
func CreatedAsOdyssey(role storage.Character) bool {
	r, e := protocol.DecodeCreateRequest(role.Request)
	return e == nil && len(r.Options) == 12 && r.Options[10] == 2
}

func (s *ProgressionService) ApplyOdysseyTarget(role storage.Character, target byte) (storage.Character, error) {
	if s.Odyssey == nil || !OdysseyRole(role) || role.ConfigVersion != s.Odyssey.Source || target < 2 || target > 115 || target > s.Rules.LevelCap || int(target)-2 >= len(s.Catalog.Thresholds) {
		return role, fmt.Errorf("invalid Odyssey target/source/role")
	}
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return role, e
	}
	if state.Level >= target {
		return role, nil
	}
	threshold := s.Catalog.Thresholds[int(target)-2]
	if state.Experience >= threshold {
		return role, fmt.Errorf("inconsistent Odyssey experience ledger")
	}
	next, result, e := s.ApplyGain(role, threshold-state.Experience)
	if e != nil {
		return role, e
	}
	if result.Level != target {
		return role, fmt.Errorf("Odyssey target mismatch")
	}
	return next, nil
}

// OdysseyExpandEquipMask maps an Odyssey dungeon clear to the extended equipment
// slot it unlocks, using the same bits as the quest [slot expansion] rewards.
//
// Odyssey characters have no quest/season system, so clearing these runs is how
// they earn the slots (rules from the user, recorded in
// analysis/tasks/next50-odyssey-expanded-equip-slot.md §1):
//
//	安徒恩讨伐战 anton.dgn   100004950 -> support,     equipment slot 22
//	使徒卢克     luke.dgn    100004953 -> magic stone, equipment slot 23
//	盖波加       gaebolg.dgn 100004969 -> earring,     equipment slot 25
func OdysseyExpandEquipMask(dungeonID uint32) (byte, bool) {
	switch dungeonID {
	case 100004950:
		return inventory.ExpandSupport, true
	case 100004953:
		return inventory.ExpandMagicStone, true
	case 100004969:
		return inventory.ExpandEarring, true
	}
	return 0, false
}

// Completion owns this transaction, before notifying the client that the exit
// portal is available. Ordinary card-result XP remains a separate receipt.
func (s *ProgressionService) OdysseyClear(ctx context.Context, role storage.Character, run *dungeon.Session) (storage.Character, bool, error) {
	if s.Odyssey == nil || run == nil || !run.Definition.Odyssey || !run.Loaded || !run.Completed() || !OdysseyRole(role) {
		return role, false, fmt.Errorf("Odyssey growth requires owned completed run")
	}
	id, e := hex.DecodeString(run.RunID)
	if e != nil || len(id) != 16 {
		return role, false, fmt.Errorf("invalid Odyssey run")
	}
	target := s.Odyssey.ClearLevels[run.Definition.ID]
	if target == 0 {
		return role, false, fmt.Errorf("missing source clear target")
	}
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Odyssey.Source, "odyssey-growth:"+run.RunID, "odyssey-source-growth-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		next, e := s.ApplyOdysseyTarget(current, target)
		if e != nil {
			return nil, nil, e
		}
		next.State, e = s.saveOdysseyCompletion(next, run.Definition.ID)
		if e != nil {
			return nil, nil, e
		}
		// The clear may also unlock an extended equipment slot. It lands in the
		// same character transaction, so a failed commit rolls both back and a
		// repeated clear stays idempotent (the mask is only OR-ed in).
		if mask, ok := OdysseyExpandEquipMask(run.Definition.ID); ok {
			next.State, e = inventory.UnlockEquipSlots(next.State, mask)
			if e != nil {
				return nil, nil, e
			}
		}
		proof, e := json.Marshal(map[string]any{"dungeon": run.Definition.ID, "target_level": target, "source": s.Odyssey.Source})
		return next.State, proof, e
	})
}

func (s *ProgressionService) odysseyRecordedTarget(role storage.Character) (byte, error) {
	if s.Odyssey == nil || !OdysseyRole(role) || role.ConfigVersion != s.Odyssey.Source {
		return 0, nil
	}
	var saved struct {
		Level byte              `json:"level"`
		Times map[string]uint32 `json:"dungeon_best_times"`
	}
	if e := json.Unmarshal(role.State, &saved); e != nil {
		return 0, e
	}
	target := saved.Level
	for id, level := range s.Odyssey.ClearLevels {
		if saved.Times[fmt.Sprintf("%d:normal:solo", id)] > 0 && level > target {
			target = level
		}
	}
	if target == saved.Level {
		return 0, nil
	}
	return target, nil
}

// Existing clear receipts wrote these server-owned best-time records even
// before Odyssey growth existed. Do not grant catch-up from client claims.
func (s *ProgressionService) OdysseyCatchup(ctx context.Context, role storage.Character) (storage.Character, bool, error) {
	target, e := s.odysseyRecordedTarget(role)
	if e != nil || target == 0 {
		return role, false, e
	}
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Odyssey.Source, fmt.Sprintf("odyssey-clear-catchup-v1:%d", target), "odyssey-source-growth-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		actual, e := s.odysseyRecordedTarget(current)
		if e != nil {
			return nil, nil, e
		}
		if actual != 0 && actual != target {
			return nil, nil, fmt.Errorf("catch-up target changed")
		}
		next := current
		if actual != 0 {
			next, e = s.ApplyOdysseyTarget(current, actual)
			if e != nil {
				return nil, nil, e
			}
		}
		proof, e := json.Marshal(map[string]any{"target_level": target, "evidence": "persisted dungeon_best_times"})
		return next.State, proof, e
	})
}
