package storage

import (
	"context"
	"fmt"
)

// Birth stages for a character's one-time starting route.
const (
	BirthPending  = 0 // created, has not entered its job tutorial
	BirthEntered  = 1 // inside the tutorial dungeon
	BirthReturned = 2 // tutorial finished, not yet settled in town
	BirthComplete = 3 // finished or intentionally skipped; ordinary play
)

// MigrateBirth creates the starting-route table and backfills every character
// that already exists as complete. Characters made before this build have
// already been played past their starting route, and the project's standing
// rule is that existing progress is never reset or replayed: only characters
// created from here on begin at BirthPending.
func (s *Store) MigrateBirth(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_birth(
 character_id bigint PRIMARY KEY REFERENCES characters(id),
 stage smallint NOT NULL CHECK(stage BETWEEN 0 AND 3),
 dungeon bigint NOT NULL DEFAULT 0 CHECK(dungeon>=0),
 updated_at timestamptz NOT NULL DEFAULT now());`)
	if e != nil {
		return e
	}
	// ON CONFLICT DO NOTHING keeps an already-recorded stage, so a character
	// created during this run is not re-marked complete by a later restart.
	_, e = s.DB.Exec(ctx, `INSERT INTO character_birth(character_id,stage)
 SELECT id,$1 FROM characters ON CONFLICT (character_id) DO NOTHING`, BirthComplete)
	return e
}

// BirthStage reports a character's starting-route stage. A character without
// a row is treated as complete: an absent row can only mean the character
// predates this table, and such a character must never be sent back through
// a starting route it already finished.
func (s *Store) BirthStage(ctx context.Context, account, id int64) (byte, uint32, error) {
	var stage int16
	var dungeon int64
	e := s.DB.QueryRow(ctx, `SELECT b.stage,b.dungeon FROM character_birth b
 JOIN characters c ON c.id=b.character_id
 WHERE c.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL`, account, id).Scan(&stage, &dungeon)
	if e != nil {
		var owned bool
		if q := s.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL)`, account, id).Scan(&owned); q != nil {
			return BirthComplete, 0, q
		}
		if !owned {
			return BirthComplete, 0, fmt.Errorf("starting route character is not owned")
		}
		return BirthComplete, 0, nil
	}
	return byte(stage), uint32(dungeon), nil
}

// StartBirth records a newly created character as owing its starting route.
func (s *Store) StartBirth(ctx context.Context, account, id int64) error {
	tag, e := s.DB.Exec(ctx, `INSERT INTO character_birth(character_id,stage)
 SELECT id,$3 FROM characters WHERE account_id=$1 AND id=$2
 ON CONFLICT (character_id) DO NOTHING`, account, id, BirthPending)
	if e == nil && tag.RowsAffected() != 1 {
		return fmt.Errorf("starting route already recorded")
	}
	return e
}

// AdvanceBirth moves a character's starting route forward. The transition is
// one-way and refuses to move backwards, so a reconnect or a replayed request
// can never send a character through its starting route a second time.
func (s *Store) AdvanceBirth(ctx context.Context, account, id int64, stage byte, dungeon uint32) (bool, error) {
	if stage > BirthComplete {
		return false, fmt.Errorf("invalid starting route stage")
	}
	tag, e := s.DB.Exec(ctx, `UPDATE character_birth b SET stage=$3,dungeon=$4,updated_at=now()
 FROM characters c WHERE c.id=b.character_id AND c.account_id=$1 AND c.id=$2
 AND c.deleted_at IS NULL AND b.stage<$3`, account, id, int16(stage), int64(dungeon))
	if e != nil {
		return false, e
	}
	return tag.RowsAffected() == 1, nil
}
