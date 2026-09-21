package storage

import (
	"context"
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
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_skill_locks (
 character_id bigint NOT NULL REFERENCES characters(id),
 skill_id integer NOT NULL CHECK(skill_id BETWEEN 1 AND 1023),
 PRIMARY KEY(character_id, skill_id));`)
	return e
}

func (s *Store) SkillLocks(ctx context.Context, id int64) ([]uint16, error) {
	rows, e := s.DB.Query(ctx, `SELECT skill_id FROM character_skill_locks WHERE character_id=$1 ORDER BY skill_id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []uint16
	for rows.Next() {
		var v int
		if e = rows.Scan(&v); e != nil {
			return nil, e
		}
		out = append(out, uint16(v))
	}
	return out, rows.Err()
}

// CommitSkillLocks merges one incremental frame into the stored set and
// replaces the set atomically under the owning character's row lock. The
// receipt key makes a replayed frame a no-op instead of applying the same
// delta twice.
func (s *Store) CommitSkillLocks(ctx context.Context, account, id int64, key, model string, apply func(current []uint16) ([]uint16, error)) ([]uint16, bool, error) {
	if key == "" || len(key) > 200 || model == "" || len(model) > 100 || apply == nil {
		return nil, false, fmt.Errorf("invalid skill lock event")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return nil, false, e
	}
	defer tx.Rollback(ctx)
	var version string
	e = tx.QueryRow(ctx, `SELECT config_version FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&version)
	if e != nil {
		return nil, false, e
	}
	var prior string
	e = tx.QueryRow(ctx, `SELECT model FROM character_events WHERE character_id=$1 AND event_key=$2`, id, key).Scan(&prior)
	if e == nil {
		if prior != model {
			return nil, false, fmt.Errorf("skill lock event model mismatch")
		}
		locks, e := skillLocksTx(ctx, tx, id)
		if e != nil {
			return nil, false, e
		}
		return locks, false, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return nil, false, e
	}
	current, e := skillLocksTx(ctx, tx, id)
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
	if _, e = tx.Exec(ctx, `DELETE FROM character_skill_locks WHERE character_id=$1`, id); e != nil {
		return nil, false, e
	}
	for _, v := range next {
		if _, e = tx.Exec(ctx, `INSERT INTO character_skill_locks(character_id,skill_id) VALUES($1,$2)`, id, int(v)); e != nil {
			return nil, false, e
		}
	}
	outcome, e := json.Marshal(map[string]any{"locks": next, "count": len(next)})
	if e != nil {
		return nil, false, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,$4,$5)`, id, key, version, model, outcome); e != nil {
		return nil, false, e
	}
	if e = tx.Commit(ctx); e != nil {
		return nil, false, e
	}
	return next, true, nil
}

func skillLocksTx(ctx context.Context, tx pgx.Tx, id int64) ([]uint16, error) {
	rows, e := tx.Query(ctx, `SELECT skill_id FROM character_skill_locks WHERE character_id=$1 ORDER BY skill_id`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	var out []uint16
	for rows.Next() {
		var v int
		if e = rows.Scan(&v); e != nil {
			return nil, e
		}
		out = append(out, uint16(v))
	}
	return out, rows.Err()
}
