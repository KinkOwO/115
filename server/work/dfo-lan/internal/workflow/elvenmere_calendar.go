package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

type ElvenmerePeriod struct {
	WeekStart int64  `json:"week_start"`
	SeasonID  string `json:"season_id"`
	At        int64  `json:"at"`
}

type ElvenmereSeasonReset struct {
	ID       string `json:"id"`
	Previous string `json:"previous"`
	At       int64  `json:"at"`
	Reason   string `json:"reason"`
}

func (s *LootService) elvenmereTime() time.Time {
	if s.ElvenmereNow != nil {
		return s.ElvenmereNow().UTC()
	}
	return time.Now().UTC()
}

func elvenmereSeasons(ctx context.Context, tx *database.Tx, now time.Time) ([]ElvenmereSeasonReset, error) {
	rows, err := tx.CharacterEventHistory(ctx, "elvenmere-season-reset-v1")
	if err != nil {
		return nil, err
	}
	var resets []ElvenmereSeasonReset
	for _, row := range rows {
		var r ElvenmereSeasonReset
		if err := json.Unmarshal(row.Outcome, &r); err != nil {
			return nil, err
		}
		id, err := hex.DecodeString(r.ID)
		if err != nil || len(id) != 16 || row.Key != "elvenmere-season-reset:"+r.ID || r.Previous == "" || r.At <= 0 || r.At > now.Unix() || r.Reason == "" {
			return nil, fmt.Errorf("invalid elvenmere season reset or clock rollback")
		}
		resets = append(resets, r)
	}
	sort.Slice(resets, func(i, j int) bool { return resets[i].At < resets[j].At })
	previous := dungeon.ElvenmereLegacySeason
	var at int64
	for _, r := range resets {
		if r.Previous != previous || r.At <= at {
			return nil, fmt.Errorf("elvenmere season reset chain is inconsistent")
		}
		previous, at = r.ID, r.At
	}
	return resets, nil
}

func elvenmereProgressLocked(ctx context.Context, tx *database.Tx, current database.Character, now time.Time) (dungeon.ElvenmereProgress, bool, error) {
	old, exists, err := dungeon.ReadElvenmereProgress(current.State)
	if err != nil {
		return old, exists, err
	}
	resets, err := elvenmereSeasons(ctx, tx, now)
	if err != nil {
		return old, exists, err
	}
	p := dungeon.ElvenmereProgress{Version: 2, WeekStart: dungeon.ElvenmereWeekStart(now), SeasonID: dungeon.ElvenmereLegacySeason}
	knownSeasons := map[string]bool{dungeon.ElvenmereLegacySeason: true}
	for _, r := range resets {
		knownSeasons[r.ID] = true
		p.SeasonID = r.ID
	}
	if old.Version == 2 && (old.WeekStart > p.WeekStart || !knownSeasons[old.SeasonID]) {
		return p, exists, fmt.Errorf("elvenmere projection has a future or unknown period")
	}
	put := func(claims *map[byte]dungeon.ElvenmereClaim, floor byte, claim dungeon.ElvenmereClaim) {
		if *claims == nil {
			*claims = map[byte]dungeon.ElvenmereClaim{}
		}
		if _, held := (*claims)[floor]; !held {
			(*claims)[floor] = claim
		}
	}
	// Preserved projection fields must have period evidence. A timestamp-less
	// old hold is not silently reassigned to every subsequent week.
	for floor, claim := range old.Weekly {
		if claim.ClaimedAt <= 0 {
			return p, exists, fmt.Errorf("elvenmere legacy weekly claim lacks time evidence")
		}
		if claim.ClaimedAt > now.Unix() {
			return p, exists, fmt.Errorf("elvenmere clock moved before a prior claim")
		}
		if dungeon.ElvenmereWeekStart(time.Unix(claim.ClaimedAt, 0)) == p.WeekStart {
			put(&p.Weekly, floor, claim)
			p.MaxCleared = max(p.MaxCleared, floor)
		}
	}
	oldSeason := old.SeasonID
	if old.Version == 1 {
		oldSeason = dungeon.ElvenmereLegacySeason
	}
	if oldSeason == p.SeasonID {
		for floor, claim := range old.Season {
			if claim.ClaimedAt <= 0 || claim.ClaimedAt > now.Unix() {
				return p, exists, fmt.Errorf("elvenmere season claim lacks valid time evidence")
			}
			put(&p.Season, floor, claim)
		}
	}
	if old.Version == 2 && old.WeekStart == p.WeekStart {
		p.MaxCleared, p.StartFloor = old.MaxCleared, old.StartFloor
	}
	var firstAt int64
	for _, model := range []string{"elvenmere-floor-v1", "elvenmere-floor-v2", "elvenmere-floor-v3"} {
		rows, err := tx.CharacterEventHistory(ctx, model)
		if err != nil {
			return p, exists, err
		}
		for _, row := range rows {
			var receipt ElvenmereFloorReceipt
			if err := json.Unmarshal(row.Outcome, &receipt); err != nil {
				return p, exists, err
			}
			g := receipt.Grant
			run, err := hex.DecodeString(g.Run)
			prefix := "elvenmere-floor"
			at, season := row.CreatedAt, dungeon.ElvenmereLegacySeason
			for _, r := range resets {
				if at >= r.At {
					season = r.ID
				}
			}
			week := dungeon.ElvenmereWeekStart(time.Unix(at, 0))
			if model == "elvenmere-floor-v2" {
				prefix += "-v2"
			}
			key := fmt.Sprintf("%s:%s:%d", prefix, g.Run, g.Floor)
			if model == "elvenmere-floor-v3" {
				at, week, season = receipt.Period.At, receipt.Period.WeekStart, receipt.Period.SeasonID
				key = fmt.Sprintf("elvenmere-floor-v3:%d:%s:%d", week, g.Run, g.Floor)
			}
			if err != nil || len(run) != 16 || g.Floor == 0 || g.Floor > 100 || g.StartFloor > g.Floor || g.ExpectedWeekStart != 0 && g.ExpectedWeekStart != week || row.Key != key || (g.WeeklyItem == 0) != (g.WeeklyCount == 0) || (g.SeasonItem == 0) != (g.SeasonCount == 0) || at <= 0 || at > now.Unix() || week != dungeon.ElvenmereWeekStart(time.Unix(at, 0)) || week > p.WeekStart || !knownSeasons[season] {
				return p, exists, fmt.Errorf("invalid elvenmere floor receipt or clock rollback")
			}
			claim := dungeon.ElvenmereClaim{EventKey: row.Key, ClaimedAt: at}
			if week == p.WeekStart {
				p.MaxCleared = max(p.MaxCleared, g.Floor)
				if p.StartFloor == 0 || firstAt == 0 || at < firstAt || at == firstAt && g.Floor < p.StartFloor {
					p.StartFloor, firstAt = g.Floor, at
				}
				if g.WeeklyItem != 0 {
					put(&p.Weekly, g.Floor, claim)
				}
			}
			if season == p.SeasonID && g.SeasonItem != 0 {
				put(&p.Season, g.Floor, claim)
			}
		}
	}
	if p.MaxCleared > 0 && p.StartFloor == 0 {
		for floor := range p.Weekly {
			if p.StartFloor == 0 || floor < p.StartFloor {
				p.StartFloor = floor
			}
		}
	}
	return p, exists, nil
}

// Reset requires an explicit administrative operation and reason. No gameplay
// opcode calls it and no guessed monthly/yearly season schedule triggers it.
// The account-wide receipt chain prevents stale characters or replayed requests
// from reactivating an old season. Weekly progress is preserved.
func (s *LootService) ResetElvenmereSeason(ctx context.Context, role database.Character, operation, reason string) (database.Character, error) {
	id, err := hex.DecodeString(operation)
	if s == nil || s.Store == nil || s.Loot == nil || err != nil || len(id) != 16 || reason == "" || len(reason) > 500 {
		return role, fmt.Errorf("elvenmere reset needs a valid operation and reason")
	}
	now := s.elvenmereTime()
	already := errors.New("elvenmere account season operation already applied")
	var unchanged database.Character
	saved, _, err := s.Store.CommitCharacterEventTx(ctx, role.AccountID, role.ID, s.Loot.Catalog.Source.SaveIdentity(), "elvenmere-season-reset:"+operation, "elvenmere-season-reset-v1", func(tx *database.Tx, current database.Character) (json.RawMessage, json.RawMessage, error) {
		now = s.elvenmereTime()
		p, _, err := elvenmereProgressLocked(ctx, tx, current, now)
		if err != nil {
			return nil, nil, err
		}
		resets, err := elvenmereSeasons(ctx, tx, now)
		if err != nil {
			return nil, nil, err
		}
		for _, reset := range resets {
			if reset.ID == operation {
				if reset.Reason != reason {
					return nil, nil, fmt.Errorf("elvenmere season operation reason mismatch")
				}
				unchanged = current
				return nil, nil, already
			}
		}
		if len(resets) > 0 && now.Unix() <= resets[len(resets)-1].At {
			return nil, nil, fmt.Errorf("elvenmere reset must advance its audit time")
		}
		proof, err := json.Marshal(ElvenmereSeasonReset{ID: operation, Previous: p.SeasonID, At: now.Unix(), Reason: reason})
		if err != nil {
			return nil, nil, err
		}
		p.SeasonID, p.Season = operation, nil
		state, err := dungeon.WriteElvenmereProgress(current.State, p)
		return state, proof, err
	})
	if errors.Is(err, already) {
		saved, err = unchanged, nil
	} else if err == nil {
		proof, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, "elvenmere-season-reset:"+operation)
		var reset ElvenmereSeasonReset
		if e != nil {
			return role, e
		}
		if e = json.Unmarshal(proof, &reset); e != nil || reset.ID != operation || reset.Reason != reason {
			return role, fmt.Errorf("elvenmere season operation receipt mismatch")
		}
	}
	if err != nil {
		return role, err
	}
	return s.RestoreElvenmereProgress(ctx, saved)
}
