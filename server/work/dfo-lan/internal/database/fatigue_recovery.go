package database

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"fmt"
)

type FatigueRecovery = character.FatigueRecovery

// Character then fatigue locks serialize inventory changes with room charges.
// The callback is pure: item decrement and fatigue credit share one commit.
func (s *Store) RecoverFatigue(ctx context.Context, account, id int64, version string, r FatigueRecovery, consume func(Character) (json.RawMessage, error)) (Character, FatigueState, error) {
	var role Character
	var fp FatigueState
	if r.Limit == 0 || r.Amount == 0 || r.Template == 0 || r.DailyUses == 0 || r.Cooldown < 0 || r.Now.IsZero() || consume == nil {
		return role, fp, fmt.Errorf("invalid fatigue recovery")
	}
	tx, e := s.db.Begin(ctx)
	if e != nil {
		return role, fp, e
	}
	defer tx.Rollback(ctx)
	role, e = lockCharacter(ctx, tx, account, id)
	if e != nil {
		return role, fp, e
	}
	if role.ConfigVersion != version {
		return role, fp, fmt.Errorf("recovery source mismatch")
	}
	q := s.queries.WithTx(tx)
	row, e := q.LoadLockedCharacterFatigue(ctx, sqlcgen.LoadLockedCharacterFatigueParams{CharacterID: id, Day: r.Day, DailyLimit: int32(r.Limit)})
	fp = FatigueState{Day: row.Day, Used: uint16(row.Used), Limit: uint16(row.DailyLimit), UsedMax: uint16(row.UsedMax)}
	if e != nil {
		return role, fp, e
	}
	if fp.Day != r.Day {
		return role, fp, fmt.Errorf("recovery clock moved backwards")
	}
	if fp.Used == 0 {
		return role, fp, fmt.Errorf("fatigue already full")
	}
	usage, e := q.FatigueRecoveryUsage(ctx, sqlcgen.FatigueRecoveryUsageParams{CharacterID: id, Template: int64(r.Template), Day: r.Day})
	if e != nil {
		return role, fp, e
	}
	if usage.DailyUses >= int64(r.DailyUses) {
		return role, fp, fmt.Errorf("fatigue potion daily quota exhausted")
	}
	if usage.HasLastUsed && r.Now.Sub(usage.LastUsed) < r.Cooldown {
		return role, fp, fmt.Errorf("fatigue potion cooldown")
	}
	state, e := consume(role)
	if e != nil {
		return role, fp, e
	}
	if !json.Valid(state) {
		return role, fp, fmt.Errorf("invalid recovery inventory")
	}
	restored := min(fp.Used, r.Amount)
	fp.Used -= restored
	e = q.UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state})
	if e != nil {
		return role, fp, e
	}
	e = q.SaveFatigueRecovery(ctx, sqlcgen.SaveFatigueRecoveryParams{CharacterID: id, Used: int32(fp.Used)})
	if e != nil {
		return role, fp, e
	}
	e = q.RecordFatigueRecovery(ctx, sqlcgen.RecordFatigueRecoveryParams{CharacterID: id, Template: int64(r.Template), Day: r.Day, Ordinal: usage.DailyUses + 1, Restored: int32(restored), UsedAt: r.Now})
	if e != nil {
		return role, fp, e
	}
	if e = tx.Commit(ctx); e != nil {
		return role, fp, e
	}
	role.State = state
	return role, fp, nil
}
