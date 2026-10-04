package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// Locked skills are character scoped: the client restores them from the
// character option block (NOTI2827) rather than from the account block, so the
// set is stored per character even though the frame arrives on the account's
// connection.
//
// Only the skill id set is durable. The two compact, ascending pages the client
// uses are re-derived from that set on every frame, which is what makes the
// client's incremental frames work: equal sets always produce equal layouts.
const maxLockedSkills = 128

func (s *Store) MigrateSkillLocks(ctx context.Context) error {
	return s.execMigration(ctx, "0008_skill_locks.sql")
}

func (s *Store) SkillLocks(ctx context.Context, id int64) ([]uint16, error) {
	return skillLocksQuery(ctx, s.queries, id)
}

// CommitSkillLocks merges one incremental frame into the stored set and
// replaces the set atomically under the owning character's row lock. The
// receipt key makes a replayed frame a no-op instead of applying the same
// delta twice.
func (s *Store) CommitSkillLocks(ctx context.Context, account, id int64, key, model string, apply func(current []uint16) ([]uint16, error)) ([]uint16, bool, error) {
	if key == "" || len(key) > 200 || model == "" || len(model) > 100 || apply == nil {
		return nil, false, fmt.Errorf("invalid skill lock event")
	}
	tx, e := s.engine.begin(ctx)
	if e != nil {
		return nil, false, e
	}
	defer tx.rollback(ctx)
	queries := tx.queries()
	version, e := queries.LockCharacterVersion(ctx, sqlcgen.LockCharacterVersionParams{AccountID: account, CharacterID: id})
	if e != nil {
		return nil, false, e
	}
	prior, e := queries.CharacterEventModel(ctx, sqlcgen.CharacterEventModelParams{CharacterID: id, EventKey: key})
	if e == nil {
		if prior != model {
			return nil, false, fmt.Errorf("skill lock event model mismatch")
		}
		locks, e := skillLocksQuery(ctx, queries, id)
		if e != nil {
			return nil, false, e
		}
		return locks, false, tx.commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return nil, false, e
	}
	current, e := skillLocksQuery(ctx, queries, id)
	if e != nil {
		return nil, false, e
	}
	next, e := apply(current)
	if e != nil {
		return nil, false, e
	}
	if len(next) > maxLockedSkills {
		return nil, false, fmt.Errorf("too many locked skills")
	}
	seen := map[uint16]bool{}
	for _, v := range next {
		if v == 0 || v > 1023 || seen[v] {
			return nil, false, fmt.Errorf("invalid locked skill")
		}
		seen[v] = true
	}
	if e = queries.ClearSkillLocks(ctx, id); e != nil {
		return nil, false, e
	}
	for _, v := range next {
		if e = queries.AddSkillLock(ctx, sqlcgen.AddSkillLockParams{CharacterID: id, SkillID: int32(v)}); e != nil {
			return nil, false, e
		}
	}
	outcome, e := json.Marshal(map[string]any{"locks": next, "count": len(next)})
	if e != nil {
		return nil, false, e
	}
	if e = queries.RecordCharacterEvent(ctx, sqlcgen.RecordCharacterEventParams{
		CharacterID: id, EventKey: key, ConfigVersion: version, Model: model, Outcome: outcome,
	}); e != nil {
		return nil, false, e
	}
	if e = tx.commit(ctx); e != nil {
		return nil, false, e
	}
	return next, true, nil
}

func skillLocksQuery(ctx context.Context, queries querySet, id int64) ([]uint16, error) {
	ids, e := queries.SkillLocks(ctx, id)
	if e != nil {
		return nil, e
	}
	var out []uint16
	for _, v := range ids {
		out = append(out, uint16(v))
	}
	return out, nil
}
