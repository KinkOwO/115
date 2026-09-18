package character

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/progression"
	"dfolan/internal/storage"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

type ProgressionService struct {
	Odyssey     *catalog.OdysseyGrowth
	Store       *storage.Store
	Catalog     catalog.Progression
	Professions catalog.Characters
	Rules       progression.Rules
}

func (s *ProgressionService) Monster(ctx context.Context, role storage.Character, run *dungeon.Session, entity uint16) (storage.Character, bool, error) {
	if run == nil || !run.Loaded || !run.Dead[entity] || role.ConfigVersion != s.Catalog.Source.Checksum || s.Professions.Source.Checksum != s.Catalog.Source.Checksum {
		return role, false, fmt.Errorf("experience requires owned confirmed source monster")
	}
	b, e := hex.DecodeString(run.RunID)
	if e != nil || len(b) != 16 {
		return role, false, fmt.Errorf("invalid experience run")
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
	return s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.Checksum, key, s.Rules.Model, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		var state State
		if e := json.Unmarshal(current.State, &state); e != nil {
			return nil, nil, e
		}
		gain, e := progression.MonsterGain(s.Catalog, s.Rules, run.Definition, monster, state.Level, 0)
		if e != nil {
			return nil, nil, e
		}
		updated, result, e := s.ApplyGain(current, gain)
		if e != nil {
			return nil, nil, e
		}
		proof, e := json.Marshal(map[string]any{"gain": gain, "level": result.Level, "experience": result.Experience, "skill_point_gain": result.SkillPointGain, "monster": monster.Template, "source_map": run.Room.Map, "reference_sha256": s.Rules.ReferenceSHA256})
		return updated.State, proof, e
	})
}

func ExperiencePayload(role storage.Character) ([]byte, error) {
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return nil, e
	}
	return protocol.ExperienceState(protocol.ExperienceUpdate{Level: state.Level, Total: state.Experience, SP: state.SkillPoints, TP: state.TechniquePoints, CurrencySlot2: state.CurrencySlot2})
}
