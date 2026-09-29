package character

import (
	"context"
	"dfolan/internal/adventure"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"dfolan/internal/progression"
	"dfolan/internal/storage"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

type ClearReceipt struct {
	progression.ClearGain
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
}

func (s *ProgressionService) Clear(ctx context.Context, role storage.Character, run *dungeon.Session, rank byte, now time.Time) (storage.Character, ClearReceipt, bool, error) {
	var receipt ClearReceipt
	fail := func(e error) (storage.Character, ClearReceipt, bool, error) { return role, receipt, false, e }
	if run == nil || !run.Completed() || run.StartedAt.IsZero() || now.Before(run.StartedAt) {
		return fail(fmt.Errorf("clear reward requires completed owned run"))
	}
	b, e := hex.DecodeString(run.RunID)
	if e != nil || len(b) != 16 {
		return fail(fmt.Errorf("invalid clear run identity"))
	}
	key := "clear:" + run.RunID
	var monsterTotal uint64
	e = s.Store.DB.QueryRow(ctx, `SELECT COALESCE(SUM((e.outcome->>'gain')::bigint),0)::bigint FROM character_events e JOIN characters c ON c.id=e.character_id WHERE c.account_id=$1 AND c.id=$2 AND e.event_key LIKE $3`, role.AccountID, role.ID, "monster:"+run.RunID+":%").Scan(&monsterTotal)
	if e != nil {
		return fail(e)
	}
	if monsterTotal > math.MaxUint32 {
		return fail(fmt.Errorf("result monster experience out of range"))
	}
	// The room ledger records each first loaded map once. In the local free
	// fatigue policy, these receipts still give the pet a progression measure.
	var chargedFatigue, loadedRooms int64
	e = s.Store.DB.QueryRow(ctx, `SELECT COALESCE(SUM(f.cost),0)::bigint,COUNT(*)::bigint FROM character_fatigue_rooms f JOIN characters c ON c.id=f.character_id WHERE c.account_id=$1 AND c.id=$2 AND f.run_id=$3`, role.AccountID, role.ID, run.RunID).Scan(&chargedFatigue, &loadedRooms)
	if e != nil {
		return fail(e)
	}
	creatureGain, e := dungeonCreatureExperience(chargedFatigue, loadedRooms, run.Definition.NoFatigue)
	if e != nil {
		return fail(fmt.Errorf("creature experience gain out of range"))
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.Checksum, key, s.Rules.Model, func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		var before State
		if e := json.Unmarshal(current.State, &before); e != nil {
			return nil, nil, e
		}
		recommended, e := adventure.RecommendedDungeonClear(run.Definition.ID, before.Level)
		if e != nil {
			return nil, nil, e
		}
		gain, e := progression.DungeonClear(s.Catalog, run.Definition, 0, rank)
		if e != nil {
			return nil, nil, e
		}
		if s.Store != nil {
			if hasGrowth, _ := s.Store.HasActivePremium(ctx, role.AccountID, storage.PremiumGrowth, now); hasGrowth {
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
		outcome, e := json.Marshal(ClearReceipt{ClearGain: gain, Source: s.Catalog.Source.Checksum, Run: run.RunID, Elapsed: uint32(elapsed), BestElapsed: best, NewRecord: improved, AllClear: all, MonsterExperience: uint32(monsterTotal), CreatureExperienceGained: creatureAwarded, SeasonExperienceGained: seasonAwarded,
			RecommendedDungeonClear: recommended, DungeonID: run.Definition.ID, CharacterLevel: before.Level})
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
	if receipt.Source != s.Catalog.Source.Checksum || receipt.Run != run.RunID || receipt.Rank != rank {
		return fail(fmt.Errorf("clear retry conflicts with saved result"))
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
