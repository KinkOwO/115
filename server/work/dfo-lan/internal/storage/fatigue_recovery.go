package storage

import (
	"context"
	"dfolan/internal/character"
	"encoding/json"
	"fmt"
	"time"
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
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return role, fp, e
	}
	defer tx.Rollback(ctx)
	e = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if e != nil {
		return role, fp, e
	}
	if role.ConfigVersion != version {
		return role, fp, fmt.Errorf("recovery source mismatch")
	}
	e = tx.QueryRow(ctx, `INSERT INTO character_fatigue(character_id,day,daily_limit) VALUES($1,$2::date,$3)
 ON CONFLICT(character_id) DO UPDATE SET
 used=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used END,
 used_max=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used_max END,
 daily_limit=CASE WHEN EXCLUDED.day>=character_fatigue.day THEN EXCLUDED.daily_limit ELSE character_fatigue.daily_limit END,
 day=GREATEST(EXCLUDED.day,character_fatigue.day),updated_at=now()
 RETURNING day::text,used,daily_limit,used_max`, id, r.Day, r.Limit).Scan(&fp.Day, &fp.Used, &fp.Limit, &fp.UsedMax)
	if e != nil {
		return role, fp, e
	}
	if fp.Day != r.Day {
		return role, fp, fmt.Errorf("recovery clock moved backwards")
	}
	if fp.Used == 0 {
		return role, fp, fmt.Errorf("fatigue already full")
	}
	var used int64
	var last *time.Time
	e = tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE day=$3::date),max(used_at) FROM character_fatigue_recovery WHERE character_id=$1 AND template=$2`, id, r.Template, r.Day).Scan(&used, &last)
	if e != nil {
		return role, fp, e
	}
	if used >= int64(r.DailyUses) {
		return role, fp, fmt.Errorf("fatigue potion daily quota exhausted")
	}
	if last != nil && r.Now.Sub(*last) < r.Cooldown {
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
	_, e = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state)
	if e != nil {
		return role, fp, e
	}
	_, e = tx.Exec(ctx, `UPDATE character_fatigue SET used=$2,updated_at=now() WHERE character_id=$1`, id, fp.Used)
	if e != nil {
		return role, fp, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO character_fatigue_recovery(character_id,template,day,ordinal,restored,used_at) VALUES($1,$2,$3::date,$4,$5,$6)`, id, r.Template, r.Day, used+1, restored, r.Now)
	if e != nil {
		return role, fp, e
	}
	if e = tx.Commit(ctx); e != nil {
		return role, fp, e
	}
	role.State = state
	return role, fp, nil
}
