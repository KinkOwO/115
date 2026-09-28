package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var ErrOathOptionRevisionConflict = errors.New("oath option revision conflict")

type OathOptionState struct {
	CharacterID     int64
	CoreInstanceKey string
	SelectedOption  int
	Revision        int64
}

// MigrateOathOptions creates the per-core option ledger. It is additive and
// safe for existing character saves; no JSON state is rewritten.
func (s *Store) MigrateOathOptions(ctx context.Context) error {
	_, err := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_oath_options (
 character_id bigint NOT NULL REFERENCES characters(id),
 core_instance_key text NOT NULL CHECK(length(core_instance_key)>0),
 selected_option integer NOT NULL CHECK(selected_option>=0),
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id,core_instance_key));
 CREATE INDEX IF NOT EXISTS character_oath_options_updated_idx
 ON character_oath_options(character_id,updated_at);`)
	return err
}

func (s *Store) OathOption(ctx context.Context, characterID int64, coreInstanceKey string) (OathOptionState, bool, error) {
	if characterID <= 0 || coreInstanceKey == "" || len(coreInstanceKey) > 256 {
		return OathOptionState{}, false, errors.New("invalid oath option key")
	}
	var out OathOptionState
	err := s.DB.QueryRow(ctx, `SELECT character_id,core_instance_key,selected_option,revision
 FROM character_oath_options WHERE character_id=$1 AND core_instance_key=$2`, characterID, coreInstanceKey).
		Scan(&out.CharacterID, &out.CoreInstanceKey, &out.SelectedOption, &out.Revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return OathOptionState{}, false, nil
	}
	if err != nil {
		return OathOptionState{}, false, err
	}
	return out, true, nil
}

// SaveOathOption performs an insert at expectedRevision=0 or an optimistic
// update at the exact current revision. The row lock covers the decision, so
// stale concurrent writes cannot overwrite newer state.
func (s *Store) SaveOathOption(ctx context.Context, characterID int64, coreInstanceKey string, selectedOption int, expectedRevision int64) (OathOptionState, error) {
	if characterID <= 0 || coreInstanceKey == "" || len(coreInstanceKey) > 256 || selectedOption < 0 || expectedRevision < 0 {
		return OathOptionState{}, errors.New("invalid oath option write")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return OathOptionState{}, err
	}
	defer tx.Rollback(ctx)
	var lockedCharacter int64
	if err = tx.QueryRow(ctx, `SELECT id FROM characters WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, characterID).Scan(&lockedCharacter); err != nil {
		return OathOptionState{}, err
	}
	var current int64
	err = tx.QueryRow(ctx, `SELECT revision FROM character_oath_options
 WHERE character_id=$1 AND core_instance_key=$2 FOR UPDATE`, characterID, coreInstanceKey).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) {
		if expectedRevision != 0 {
			return OathOptionState{}, fmt.Errorf("%w: expected %d, row absent", ErrOathOptionRevisionConflict, expectedRevision)
		}
		current = 1
		_, err = tx.Exec(ctx, `INSERT INTO character_oath_options(character_id,core_instance_key,selected_option,revision,updated_at)
 VALUES($1,$2,$3,$4,now())`, characterID, coreInstanceKey, selectedOption, current)
	} else if err == nil {
		if current != expectedRevision {
			return OathOptionState{}, fmt.Errorf("%w: expected %d, current %d", ErrOathOptionRevisionConflict, expectedRevision, current)
		}
		current++
		_, err = tx.Exec(ctx, `UPDATE character_oath_options SET selected_option=$3,revision=$4,updated_at=now()
 WHERE character_id=$1 AND core_instance_key=$2`, characterID, coreInstanceKey, selectedOption, current)
	}
	if err != nil {
		return OathOptionState{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return OathOptionState{}, err
	}
	return OathOptionState{CharacterID: characterID, CoreInstanceKey: coreInstanceKey, SelectedOption: selectedOption, Revision: current}, nil
}
