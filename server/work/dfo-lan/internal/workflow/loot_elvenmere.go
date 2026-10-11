package workflow

import (
	"bytes"
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// ElvenmereFloorGrant contains the gateway's already resolved floor rewards.
// It does not introduce a second reward table or choose a weekly reset policy.
type ElvenmereFloorGrant struct {
	Run                     string
	Floor                   byte
	StartFloor              byte
	ExpectedWeekStart       int64
	WeeklyItem, WeeklyCount uint32
	SeasonItem, SeasonCount uint32
	Experience              uint64
}

type ElvenmereFloorReceipt struct {
	Grant  ElvenmereFloorGrant `json:"grant"`
	Period ElvenmerePeriod     `json:"period,omitempty"`
}

// CommitElvenmereFloor computes from the locked current character, atomically
// commits all configured rewards and records one replayable floor receipt.
// Failed item grants or growth leave both inventory and receipt unchanged.
func (s *LootService) CommitElvenmereFloor(ctx context.Context, role database.Character, g ElvenmereFloorGrant, growth *character.ProgressionService) (database.Character, ElvenmereFloorReceipt, error) {
	var receipt ElvenmereFloorReceipt
	run, err := hex.DecodeString(g.Run)
	if s == nil || s.Store == nil || s.Loot == nil || err != nil || len(run) != 16 || g.Floor < 1 || g.Floor > 100 ||
		(g.WeeklyItem == 0) != (g.WeeklyCount == 0) || (g.SeasonItem == 0) != (g.SeasonCount == 0) || g.Experience > 0 && growth == nil {
		return role, receipt, fmt.Errorf("elvenmere floor grant unavailable or invalid")
	}
	now := s.elvenmereTime()
	week := dungeon.ElvenmereWeekStart(now)
	if g.ExpectedWeekStart != 0 && g.ExpectedWeekStart != week {
		return role, receipt, fmt.Errorf("elvenmere run belongs to an expired week")
	}
	key := fmt.Sprintf("elvenmere-floor-v3:%d:%s:%d", week, g.Run, g.Floor)
	noGrant := errors.New("elvenmere floor already held")
	var unchanged database.Character
	saved, applied, err := s.Store.CommitCharacterEventTx(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), key, "elvenmere-floor-v3", func(tx *database.Tx, current database.Character) (json.RawMessage, json.RawMessage, error) {
		lockedNow := s.elvenmereTime()
		if lockedNow.Before(now) || dungeon.ElvenmereWeekStart(lockedNow) != week {
			return nil, nil, fmt.Errorf("elvenmere period changed while waiting for storage")
		}
		now = lockedNow
		before, _, e := dungeon.ReadElvenmereProgress(current.State)
		if e != nil {
			return nil, nil, e
		}
		progress, exists, e := elvenmereProgressLocked(ctx, tx, current, now)
		if e != nil {
			return nil, nil, e
		}

		if progress.MaxCleared == 0 && g.StartFloor != 0 && g.Floor != g.StartFloor {
			return nil, nil, fmt.Errorf("elvenmere first clear is outside selected start")
		}
		if progress.MaxCleared != 0 && g.Floor > progress.MaxCleared+1 {
			return nil, nil, fmt.Errorf("elvenmere skipped an uncleared floor")
		}
		if progress.MaxCleared != 0 && (g.Floor < progress.StartFloor || g.StartFloor != 0 && g.StartFloor != progress.StartFloor) {
			return nil, nil, fmt.Errorf("elvenmere run is outside the account's bound start")
		}
		actual := g
		if _, held := progress.Weekly[g.Floor]; held {
			actual.WeeklyItem, actual.WeeklyCount = 0, 0
		}
		if _, held := progress.Season[g.Floor]; held {
			actual.SeasonItem, actual.SeasonCount = 0, 0
		}
		if progress.MaxCleared >= g.Floor {
			actual.Experience = 0
		}
		beforeJSON, _ := json.Marshal(before)
		mergedJSON, _ := json.Marshal(progress)
		if exists && bytes.Equal(beforeJSON, mergedJSON) && actual.WeeklyItem == 0 && actual.SeasonItem == 0 && actual.Experience == 0 && progress.MaxCleared >= g.Floor {
			unchanged = current
			receipt = ElvenmereFloorReceipt{Grant: actual, Period: ElvenmerePeriod{WeekStart: week, SeasonID: progress.SeasonID, At: now.Unix()}}
			return nil, nil, noGrant
		}
		awarder := &inventory.Awarder{Catalog: s.Loot.Catalog, Rules: s.Loot.BagRules, Equipment: s.Loot.Equipment}
		for _, item := range []struct{ template, count uint32 }{{actual.WeeklyItem, actual.WeeklyCount}, {actual.SeasonItem, actual.SeasonCount}} {
			if item.template == 0 {
				continue
			}
			state, _, e := awarder.Grant(current.State, item.template, item.count)
			if e != nil {
				return nil, nil, e
			}
			current.State = state
		}
		if actual.Experience > 0 {
			var e error
			current, _, e = growth.ApplyGain(current, actual.Experience)
			if e != nil {
				return nil, nil, e
			}
		}
		if progress.StartFloor == 0 {
			progress.StartFloor = g.Floor
		}
		progress.MaxCleared = max(progress.MaxCleared, g.Floor)
		claim := dungeon.ElvenmereClaim{EventKey: key, ClaimedAt: now.Unix()}
		if actual.WeeklyItem != 0 {
			if progress.Weekly == nil {
				progress.Weekly = map[byte]dungeon.ElvenmereClaim{}
			}
			progress.Weekly[g.Floor] = claim
		}
		if actual.SeasonItem != 0 {
			if progress.Season == nil {
				progress.Season = map[byte]dungeon.ElvenmereClaim{}
			}
			progress.Season[g.Floor] = claim
		}
		commitNow := s.elvenmereTime()
		if commitNow.Before(now) || dungeon.ElvenmereWeekStart(commitNow) != week {
			return nil, nil, fmt.Errorf("elvenmere period changed during settlement")
		}
		current.State, e = dungeon.WriteElvenmereProgress(current.State, progress)
		if e != nil {
			return nil, nil, e
		}
		proof, e := json.Marshal(ElvenmereFloorReceipt{Grant: actual, Period: ElvenmerePeriod{WeekStart: week, SeasonID: progress.SeasonID, At: now.Unix()}})
		return current.State, proof, e
	})
	if errors.Is(err, noGrant) {
		unchanged.WireID = role.WireID
		return unchanged, receipt, nil
	}
	if err != nil {
		return role, receipt, err
	}
	proof, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err != nil {
		return role, receipt, err
	}
	if err = json.Unmarshal(proof, &receipt); err != nil {
		return role, receipt, fmt.Errorf("elvenmere floor receipt invalid: %w", err)
	}
	if receipt.Grant.Run != g.Run || receipt.Grant.Floor != g.Floor {
		return role, receipt, fmt.Errorf("elvenmere floor receipt identity mismatch")
	}
	if !applied {
		// An old response can be retried after another account character cleared
		// more floors. Refresh its projection instead of restoring stale flags.
		saved, err = s.RestoreElvenmereProgress(ctx, saved)
		if err != nil {
			return role, receipt, err
		}
	}
	saved.WireID = role.WireID
	return saved, receipt, nil
}

// Entry projects current account receipts without granting or writing an event.
// The sentinel rolls back the transaction after its owned, locked read; a fixed
// migration receipt must not make later entries miss another character's clears.
func (s *LootService) RestoreElvenmereProgress(ctx context.Context, role database.Character) (database.Character, error) {
	if s == nil || s.Store == nil || s.Loot == nil {
		return role, fmt.Errorf("elvenmere restore storage unavailable")
	}
	now := s.elvenmereTime()
	readDone := errors.New("elvenmere account projection read")
	var projected database.Character
	_, _, err := s.Store.CommitCharacterEventTx(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), "elvenmere-progress:read-v3", "elvenmere-progress-read-v3", func(tx *database.Tx, current database.Character) (json.RawMessage, json.RawMessage, error) {
		now = s.elvenmereTime()
		p, _, e := elvenmereProgressLocked(ctx, tx, current, now)
		if e != nil {
			return nil, nil, e
		}
		current.State, e = dungeon.WriteElvenmereProgress(current.State, p)
		if e != nil {
			return nil, nil, e
		}
		projected = current
		return nil, nil, readDone
	})
	if !errors.Is(err, readDone) {
		if err == nil {
			err = fmt.Errorf("elvenmere read key unexpectedly persisted")
		}
		return role, err
	}
	projected.WireID = role.WireID
	return projected, nil
}
