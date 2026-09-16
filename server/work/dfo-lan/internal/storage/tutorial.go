package storage

import (
	"context"
	"fmt"
)

func (s *Store) MigrateTutorial(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_tutorial_flags(
 character_id bigint NOT NULL REFERENCES characters(id),
 flag integer NOT NULL CHECK(flag BETWEEN 0 AND 100),
 completed boolean NOT NULL, updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,flag));`)
	return e
}
func (s *Store) SaveTutorialFlag(ctx context.Context, account, id int64, index byte, completed bool) error {
	if index >= 101 {
		return fmt.Errorf("tutorial index out of range")
	}
	tag, e := s.DB.Exec(ctx, `INSERT INTO character_tutorial_flags(character_id,flag,completed)
 SELECT id,$3,$4 FROM characters WHERE account_id=$1 AND id=$2
 ON CONFLICT(character_id,flag) DO UPDATE SET completed=EXCLUDED.completed,updated_at=now()`, account, id, index, completed)
	if e == nil && tag.RowsAffected() != 1 {
		return fmt.Errorf("tutorial character is not owned")
	}
	return e
}
func (s *Store) TutorialFlags(ctx context.Context, account, id int64) ([]byte, error) {
	var owned bool
	if e := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM characters WHERE account_id=$1 AND id=$2)`, account, id).Scan(&owned); e != nil {
		return nil, e
	}
	if !owned {
		return nil, fmt.Errorf("tutorial character is not owned")
	}
	rows, e := s.DB.Query(ctx, `SELECT flag FROM character_tutorial_flags WHERE character_id=$1 AND completed ORDER BY flag`, id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := make([]byte, 0)
	for rows.Next() {
		var flag byte
		if e = rows.Scan(&flag); e != nil {
			return nil, e
		}
		out = append(out, flag)
	}
	return out, rows.Err()
}
