package database

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database/sqlcgen"
	"errors"
	"fmt"
)

type FatigueState = character.FatigueState

func (s *Store) MigrateFatigue(ctx context.Context) error {
	return s.execMigration(ctx, "0021_fatigue.sql")
}

var ErrFatigueExhausted = errors.New("fatigue exhausted")

// The fatigue row serializes concurrent charges and rollover. The room ledger
// makes retries idempotent even after an uncertain commit or response failure.
func (s *Store) ConsumeRoomFatigue(ctx context.Context, account, id int64, day string, limit uint16, run string, room uint32, cost uint16) (FatigueState, bool, error) {
	var out FatigueState
	if limit == 0 || len(run) != 32 || room == 0 {
		return out, false, fmt.Errorf("invalid fatigue charge")
	}
	tx, e := s.engine.begin(ctx)
	if e != nil {
		return out, false, e
	}
	defer tx.rollback(ctx)
	q := tx.queries()
	row, e := q.LoadFatigue(ctx, sqlcgen.LoadFatigueParams{AccountID: account, CharacterID: id, Day: day, DailyLimit: int32(limit)})
	out = FatigueState{Day: row.Day, Used: uint16(row.Used), Limit: uint16(row.DailyLimit), UsedMax: uint16(row.UsedMax)}
	if e != nil {
		return out, false, e
	}
	exists, e := q.FatigueRoomRecorded(ctx, sqlcgen.FatigueRoomRecordedParams{CharacterID: id, RunID: run, MapID: int64(room)})
	if e != nil {
		return out, false, e
	}
	if exists {
		return out, false, tx.commit(ctx)
	}
	if cost > 0 && out.Used >= out.Limit {
		// Once a run paid for entry, reaching zero must not strand its next
		// room during the loading handshake. A free/exempt receipt does not
		// authorize a new paid run. The locked character row serializes this
		// check with concurrent charges; retries still use the room ledger.
		paidRun, e := q.RunPaidFatigue(ctx, sqlcgen.RunPaidFatigueParams{CharacterID: id, RunID: run})
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
	e = q.RecordFatigueRoom(ctx, sqlcgen.RecordFatigueRoomParams{CharacterID: id, RunID: run, MapID: int64(room), Day: out.Day, Cost: int32(cost)})
	if e != nil {
		return out, false, e
	}
	e = q.SaveFatigueCharge(ctx, sqlcgen.SaveFatigueChargeParams{CharacterID: id, Used: int32(out.Used), UsedMax: int32(out.UsedMax)})
	if e != nil {
		return out, false, e
	}
	return out, true, tx.commit(ctx)
}

// RunPaidFatigue 报告某个 run 是否已经付过进本消耗（存在 cost>0 的房间记录）。
//
// 「进本只收一次」（源 [use fatigue only start dungeon]）靠它实现：第一次进本记
// 官方值，之后同一 run 的换房记 0。零消耗（exempt / 本地策略 0 点）不产生付费记录，
// 因此不会把一次免费进入误判成「已付费」。
func (s *Store) RunPaidFatigue(ctx context.Context, id int64, run string) (bool, error) {
	return s.queries.RunPaidFatigue(ctx, sqlcgen.RunPaidFatigueParams{CharacterID: id, RunID: run})
}

// Ownership and rollover are one atomic statement. Reconnecting on the same
// day preserves consumption. A backwards clock never grants a fresh quota.
func (s *Store) LoadFatigue(ctx context.Context, account, id int64, day string, limit uint16) (FatigueState, error) {
	var out FatigueState
	if limit == 0 {
		return out, errors.New("zero fatigue limit")
	}
	row, e := s.queries.LoadFatigue(ctx, sqlcgen.LoadFatigueParams{AccountID: account, CharacterID: id, Day: day, DailyLimit: int32(limit)})
	out = FatigueState{Day: row.Day, Used: uint16(row.Used), Limit: uint16(row.DailyLimit), UsedMax: uint16(row.UsedMax)}
	return out, storageError(e)
}
