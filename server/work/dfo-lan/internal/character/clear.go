package character

import (
	"context"
	"dfolan/internal/adventure"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

type ClearReceipt struct {
	GrowthClearGain
	Source, Run              string
	Elapsed                  uint32
	BestElapsed              uint32
	NewRecord                bool
	AllClear                 bool
	MonsterExperience        uint32
	CreatureExperienceGained uint32
	SeasonExperienceGained   uint32 `json:"season_experience_gained,omitempty"`
	RecommendedDungeonClear  bool   `json:"recommended_dungeon_clear,omitempty"`
	DungeonID                uint32 `json:"dungeon_id,omitempty"`
	CharacterLevel           byte   `json:"character_level,omitempty"`
	TowerRewards             []inventory.AwardReceipt
}

func (s *ProgressionService) Clear(ctx context.Context, role Character, run *dungeon.Session, rank byte, now time.Time) (Character, ClearReceipt, bool, error) {
	return s.ClearWithTowerRewards(ctx, role, run, rank, now, nil)
}

// ClearWithTowerRewards commits sourced tower items in the same idempotent
// clear event as experience and the dungeon record.
func (s *ProgressionService) ClearWithTowerRewards(ctx context.Context, role Character, run *dungeon.Session, rank byte, now time.Time, awarder *inventory.Awarder) (Character, ClearReceipt, bool, error) {
	var receipt ClearReceipt
	fail := func(e error) (Character, ClearReceipt, bool, error) { return role, receipt, false, e }
	if run == nil || !run.Completed() || run.StartedAt.IsZero() || now.Before(run.StartedAt) {
		return fail(fmt.Errorf("clear reward requires completed owned run"))
	}
	b, e := hex.DecodeString(run.RunID)
	if e != nil || len(b) != 16 {
		return fail(fmt.Errorf("invalid clear run identity"))
	}
	difficulty, e := growthDifficultyIndex(run.Difficulty)
	if e != nil {
		return fail(e)
	}
	key := "clear:" + run.RunID
	monsterTotal, e := s.Store.RunMonsterExperience(ctx, role.AccountID, role.ID, run.RunID)
	if e != nil {
		return fail(e)
	}
	if monsterTotal > math.MaxUint32 {
		return fail(fmt.Errorf("result monster experience out of range"))
	}
	// The room ledger records each first loaded map once. In the local free
	// fatigue policy, these receipts still give the pet a progression measure.
	chargedFatigue, loadedRooms, e := s.Store.RunFatigueLedger(ctx, role.AccountID, role.ID, run.RunID)
	if e != nil {
		return fail(e)
	}
	creatureGain, e := dungeonCreatureExperience(chargedFatigue, loadedRooms, run.Definition.NoFatigue)
	if e != nil {
		return fail(fmt.Errorf("creature experience gain out of range"))
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.SaveIdentity(), key, s.Rules.Model, func(current Character) (json.RawMessage, json.RawMessage, error) {
		var before State
		if e := json.Unmarshal(current.State, &before); e != nil {
			return nil, nil, e
		}
		recommended, e := adventure.RecommendedDungeonClear(run.Definition.ID, before.Level)
		if e != nil {
			return nil, nil, e
		}
		gain, e := GrowthDungeonClear(s.Catalog, run.Definition, difficulty, rank)
		if e != nil {
			return nil, nil, e
		}
		if s.Store != nil {
			if hasGrowth, _ := s.Store.HasGrowthPremium(ctx, role.AccountID, now); hasGrowth {
				gain.Base = gain.Base + gain.Base*20/100
				gain.Score = gain.Score + gain.Score*20/100
			}
		}
		next, _, e := s.ApplyGain(current, uint64(gain.Base)+uint64(gain.Score))
		if e != nil {
			return nil, nil, e
		}
		elapsed := now.Sub(run.StartedAt).Milliseconds()
		if elapsed > math.MaxUint32 {
			return nil, nil, fmt.Errorf("clear duration out of range")
		}
		state, best, improved, e := saveClearRecord(next.State, run.Definition.ID, uint32(elapsed))
		if e != nil {
			return nil, nil, e
		}
		next.State = state
		var seasonAwarded uint32
		next.State, seasonAwarded, e = awardSeasonClear(next.State, run.Definition.ID, run.Difficulty, now)
		if e != nil {
			return nil, nil, e
		}
		var creatureAwarded uint32
		next.State, creatureAwarded, e = inventory.AwardEquippedCreatureExperience(next.State, creatureGain)
		if e != nil {
			return nil, nil, e
		}
		var towerRewards []inventory.AwardReceipt
		if tower := run.Definition.Tower; tower != nil {
			for _, item := range tower.Items {
				if awarder == nil || item.Template == 0 || item.Amount == 0 {
					return nil, nil, fmt.Errorf("tower item reward has no verified grant source")
				}
				updated, granted, grantErr := awarder.Grant(next.State, item.Template, item.Amount)
				if grantErr != nil {
					return nil, nil, grantErr
				}
				next.State = updated
				towerRewards = append(towerRewards, granted)
			}
		}
		all := true
		for _, room := range run.Maze.Rooms {
			monsters, visited := run.Visited[room.Map]
			if !visited {
				all = false
			}
			for _, m := range monsters {
				if !m.NonCombat && !run.Dead[m.Entity] {
					all = false
				}
			}
		}
		outcome, e := json.Marshal(ClearReceipt{GrowthClearGain: gain, Source: s.Catalog.Source.SaveIdentity(), Run: run.RunID, Elapsed: uint32(elapsed), BestElapsed: best, NewRecord: improved, AllClear: all, MonsterExperience: uint32(monsterTotal), CreatureExperienceGained: creatureAwarded, SeasonExperienceGained: seasonAwarded,
			RecommendedDungeonClear: recommended, DungeonID: run.Definition.ID, CharacterLevel: before.Level, TowerRewards: towerRewards})
		return next.State, outcome, e
	})
	if e != nil {
		return fail(e)
	}
	raw, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(raw, &receipt); e != nil {
		return fail(e)
	}
	if receipt.Source != s.Catalog.Source.SaveIdentity() || receipt.Run != run.RunID || receipt.Rank != rank {
		return fail(fmt.Errorf("clear retry conflicts with saved result"))
	}
	if applied {
		s.notifyLevelUp(ctx, role, saved)
	}
	return saved, receipt, applied, nil
}

// A free-fatigue server still awards one pet experience per first-loaded room.
// Explicit no-fatigue dungeons remain exempt, and paid policies use the
// actual charged amount when it is positive.
func dungeonCreatureExperience(chargedFatigue, loadedRooms int64, exempt bool) (uint32, error) {
	if chargedFatigue < 0 || loadedRooms < 0 || chargedFatigue > math.MaxUint32 || loadedRooms > math.MaxUint32 {
		return 0, fmt.Errorf("invalid creature experience source")
	}
	if exempt {
		return 0, nil
	}
	if chargedFatigue > 0 {
		return uint32(chargedFatigue), nil
	}
	return uint32(loadedRooms), nil
}

// The supported dungeon flow currently admits Normal solo difficulty only.
// Keep the record in the same character transaction as the clear reward.
func saveClearRecord(raw json.RawMessage, dungeonID, elapsed uint32) (json.RawMessage, uint32, bool, error) {
	if elapsed == 0 || dungeonID == 0 {
		return nil, 0, false, fmt.Errorf("invalid clear record")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, 0, false, err
	}
	records := map[string]uint32{}
	if b := fields["dungeon_best_times"]; len(b) > 0 {
		if err := json.Unmarshal(b, &records); err != nil {
			return nil, 0, false, err
		}
	}
	if records == nil {
		records = map[string]uint32{}
	}
	key := fmt.Sprintf("%d:normal:solo", dungeonID)
	best := records[key]
	improved := best == 0 || elapsed < best
	if improved {
		best = elapsed
		records[key] = best
	}
	fields["dungeon_best_times"], _ = json.Marshal(records)
	out, err := json.Marshal(fields)
	return out, best, improved, err
}
