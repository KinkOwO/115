package storage

import (
	"context"
	"dfolan/internal/character"
	"errors"
	"fmt"
)

type FatigueState = character.FatigueState

func (s *Store) MigrateFatigue(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_fatigue (
 character_id bigint PRIMARY KEY REFERENCES characters(id),
 day date NOT NULL, used integer NOT NULL DEFAULT 0 CHECK(used BETWEEN 0 AND 65535),
 daily_limit integer NOT NULL CHECK(daily_limit BETWEEN 1 AND 65535),
 used_max integer NOT NULL DEFAULT 0 CHECK(used_max BETWEEN 0 AND 65535),
 updated_at timestamptz NOT NULL DEFAULT now());
 CREATE TABLE IF NOT EXISTS character_fatigue_rooms (
 character_id bigint NOT NULL REFERENCES characters(id), run_id text NOT NULL,
 map_id bigint NOT NULL CHECK(map_id > 0), day date NOT NULL,
 cost integer NOT NULL CHECK(cost BETWEEN 0 AND 65535),
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,run_id,map_id));
 CREATE TABLE IF NOT EXISTS character_fatigue_recovery (
 character_id bigint NOT NULL REFERENCES characters(id), template bigint NOT NULL,
 day date NOT NULL, ordinal integer NOT NULL, restored integer NOT NULL,
 used_at timestamptz NOT NULL,
 PRIMARY KEY(character_id,template,day,ordinal));`)
	return e
}

var ErrFatigueExhausted = errors.New("fatigue exhausted")

// The fatigue row serializes concurrent charges and rollover. The room ledger
// makes retries idempotent even after an uncertain commit or response failure.
func (s *Store) ConsumeRoomFatigue(ctx context.Context, account, id int64, day string, limit uint16, run string, room uint32, cost uint16) (FatigueState, bool, error) {
	var out FatigueState
	if limit == 0 || len(run) != 32 || room == 0 {
		return out, false, fmt.Errorf("invalid fatigue charge")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return out, false, e
	}
	defer tx.Rollback(ctx)
	e = tx.QueryRow(ctx, `INSERT INTO character_fatigue(character_id,day,daily_limit)
 SELECT id,$3::date,$4 FROM characters WHERE account_id=$1 AND id=$2
 ON CONFLICT(character_id) DO UPDATE SET
 used=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used END,
 used_max=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used_max END,
 daily_limit=CASE WHEN EXCLUDED.day>=character_fatigue.day THEN EXCLUDED.daily_limit ELSE character_fatigue.daily_limit END,
 day=GREATEST(EXCLUDED.day,character_fatigue.day),updated_at=now()
 RETURNING day::text,used,daily_limit,used_max`, account, id, day, limit).Scan(&out.Day, &out.Used, &out.Limit, &out.UsedMax)
	if e != nil {
		return out, false, e
	}
	var exists bool
	e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM character_fatigue_rooms WHERE character_id=$1 AND run_id=$2 AND map_id=$3)`, id, run, room).Scan(&exists)
	if e != nil {
		return out, false, e
	}
	if exists {
		return out, false, tx.Commit(ctx)
	}
	if cost > 0 && out.Used >= out.Limit {
		// Once a run paid for entry, reaching zero must not strand its next
		// room during the loading handshake. A free/exempt receipt does not
		// authorize a new paid run. The locked character row serializes this
		// check with concurrent charges; retries still use the room ledger.
		var paidRun bool
		e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM character_fatigue_rooms WHERE character_id=$1 AND run_id=$2 AND cost>0)`, id, run).Scan(&paidRun)
		if e != nil {
			return out, false, e
		}
		if !paidRun {
			return out, false, ErrFatigueExhausted
		}
		cost = 0
	}
	// A room is affordable whenever some fatigue remains. Clamp at zero;
	// configurable costs never wrap the unsigned wire counters.
	if remaining := int(out.Limit) - int(out.Used); int(cost) > remaining && cost > 0 {
		cost = uint16(remaining)
	}
	out.Used += cost
	if out.Used > out.UsedMax {
		out.UsedMax = out.Used
	}
	_, e = tx.Exec(ctx, `INSERT INTO character_fatigue_rooms(character_id,run_id,map_id,day,cost) VALUES($1,$2,$3,$4::date,$5)`, id, run, room, out.Day, cost)
	if e != nil {
		return out, false, e
	}
	_, e = tx.Exec(ctx, `UPDATE character_fatigue SET used=$2,used_max=$3,updated_at=now() WHERE character_id=$1`, id, out.Used, out.UsedMax)
	if e != nil {
		return out, false, e
	}
	return out, true, tx.Commit(ctx)
}

// RunPaidFatigue 报告某个 run 是否已经付过进本消耗（存在 cost>0 的房间记录）。
//
// 「进本只收一次」（源 [use fatigue only start dungeon]）靠它实现：第一次进本记
// 官方值，之后同一 run 的换房记 0。零消耗（exempt / 本地策略 0 点）不产生付费记录，
// 因此不会把一次免费进入误判成「已付费」。
func (s *Store) RunPaidFatigue(ctx context.Context, id int64, run string) (bool, error) {
	var paid bool
	if e := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM character_fatigue_rooms WHERE character_id=$1 AND run_id=$2 AND cost>0)`, id, run).Scan(&paid); e != nil {
		return false, e
	}
	return paid, nil
}

// Ownership and rollover are one atomic statement. Reconnecting on the same
// day preserves consumption. A backwards clock never grants a fresh quota.
func (s *Store) LoadFatigue(ctx context.Context, account, id int64, day string, limit uint16) (FatigueState, error) {
	var out FatigueState
	if limit == 0 {
		return out, errors.New("zero fatigue limit")
	}
	e := s.DB.QueryRow(ctx, `INSERT INTO character_fatigue(character_id,day,daily_limit)
 SELECT id,$3::date,$4 FROM characters WHERE account_id=$1 AND id=$2
 ON CONFLICT(character_id) DO UPDATE SET
 used=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used END,
 used_max=CASE WHEN EXCLUDED.day>character_fatigue.day THEN 0 ELSE character_fatigue.used_max END,
 daily_limit=CASE WHEN EXCLUDED.day>=character_fatigue.day THEN EXCLUDED.daily_limit ELSE character_fatigue.daily_limit END,
 day=GREATEST(EXCLUDED.day,character_fatigue.day),updated_at=now()
 RETURNING day::text,used,daily_limit,used_max`, account, id, day, limit).Scan(&out.Day, &out.Used, &out.Limit, &out.UsedMax)
	return out, e
}
