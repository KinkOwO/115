package character

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/reward"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type ProgressionService struct {
	JournalRoutes     *catalog.OdysseyJournalRoutes
	Odyssey           *catalog.OdysseyGrowth
	Chapters          *catalog.OdysseyChapters
	CompletionRewards *catalog.OdysseyCompletionRewards
	CompletionAwarder *inventory.Awarder
	Store             ProgressionStore
	Catalog           catalog.Progression
	Professions       catalog.Characters
	Rules             GrowthRules
	// Rewards is the optional event-triggered reward notifier. Domains only
	// notify after a committed success; nil disables the feature.
	Rewards reward.Notifier
}

func (s *ProgressionService) Monster(ctx context.Context, role Character, run *dungeon.Session, entity uint16) (Character, bool, error) {
	if run == nil || !run.Loaded || !run.Dead[entity] || role.ConfigVersion != s.Catalog.Source.SaveIdentity() || s.Professions.Source.Checksum != s.Catalog.Source.Checksum {
		return role, false, fmt.Errorf("experience requires owned confirmed source monster")
	}
	b, e := hex.DecodeString(run.RunID)
	if e != nil || len(b) != 16 {
		return role, false, fmt.Errorf("invalid experience run")
	}
	difficulty, e := growthDifficultyIndex(run.Difficulty)
	if e != nil {
		return role, false, e
	}
	var monster protocol.DungeonMonster
	found := false
	for _, m := range run.Monsters {
		if m.Entity == entity {
			monster = m
			found = true
			break
		}
	}
	if !found {
		return role, false, fmt.Errorf("experience target absent from current room")
	}
	if monster.NonCombat {
		return role, false, nil
	}
	key := fmt.Sprintf("monster:%s:%d:%d", run.RunID, run.Room.Map, entity)
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.SaveIdentity(), key, s.Rules.Model, func(current Character) (json.RawMessage, json.RawMessage, error) {
		var state State
		if e := json.Unmarshal(current.State, &state); e != nil {
			return nil, nil, e
		}
		gain, e := GrowthMonsterGain(s.Catalog, s.Rules, run.Definition, monster, state.Level, difficulty)
		if e != nil {
			return nil, nil, e
		}
		if s.Store != nil {
			if hasGrowth, _ := s.Store.HasGrowthPremium(ctx, role.AccountID, time.Now()); hasGrowth {
				gain = gain + gain*20/100
			}
		}
		updated, result, e := s.ApplyGain(current, gain)
		if e != nil {
			return nil, nil, e
		}
		proof, e := json.Marshal(map[string]any{"gain": gain, "level": result.Level, "experience": result.Experience, "skill_point_gain": result.SkillPointGain, "monster": monster.Template, "source_map": run.Room.Map, "reference_sha256": s.Rules.ReferenceSHA256})
		return updated.State, proof, e
	})
	if e != nil {
		return saved, applied, e
	}
	if applied {
		s.notifyLevelUp(ctx, role, saved)
	}
	return saved, applied, nil
}

// notifyLevelUp is best-effort: a reward failure never affects the committed
// domain result. before/after are the pre-commit and committed characters.
func (s *ProgressionService) notifyLevelUp(ctx context.Context, before, after Character) {
	if s.Rewards == nil {
		return
	}
	level := stateLevel(after.State)
	if level <= stateLevel(before.State) {
		return
	}
	s.Rewards.LevelUp(ctx, reward.Recipient{AccountID: after.AccountID, CharacterID: after.ID, Name: after.Name, Level: level, ConfigVersion: after.ConfigVersion})
}

// stateLevel reads the committed level from a saved character state. An
// absent or malformed state yields 0, which suppresses the notification.
func stateLevel(raw json.RawMessage) byte {
	var state State
	if e := json.Unmarshal(raw, &state); e != nil {
		return 0
	}
	return state.Level
}

func ExperiencePayload(role Character) ([]byte, error) {
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, e
	}
	return protocol.ExperienceState(protocol.ExperienceUpdate{Level: state.Level, Total: state.Experience, SP: state.SkillPoints, TP: state.TechniquePoints, CurrencySlot2: state.CurrencySlot2})
}
