package storage

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
)

const OdysseyGraduationEvent = "odyssey-graduation-v2"

// CommitOdysseyGraduation commits mode, quests and receipt under the same
// character lock. A v1 receipt must not suppress migration of an older save.
func (s *Store) CommitOdysseyGraduation(ctx context.Context, account, id int64, version string, apply func(Character, bool) (json.RawMessage, json.RawMessage, []uint16, error)) (Character, bool, error) {
	var role Character
	checksum, err := hex.DecodeString(version)
	if err != nil || len(checksum) != 32 || apply == nil {
		return role, false, fmt.Errorf("invalid Odyssey graduation event")
	}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return role, false, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at
 FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).
		Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if err != nil {
		return role, false, err
	}
	if role.ConfigVersion != version {
		return role, false, fmt.Errorf("Odyssey graduation source mismatch")
	}
	var prior string
	err = tx.QueryRow(ctx, `SELECT model FROM character_events WHERE character_id=$1 AND event_key=$2`, id, OdysseyGraduationEvent).Scan(&prior)
	if err == nil {
		if prior != OdysseyGraduationEvent {
			return role, false, fmt.Errorf("Odyssey graduation model mismatch")
		}
		return role, false, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return role, false, err
	}
	var paid bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM character_events WHERE character_id=$1 AND event_key IN ('odyssey-graduate-reward-v1','odyssey-honor-mail-v1'))`, id).Scan(&paid); err != nil {
		return role, false, err
	}
	state, proof, quests, err := apply(role, paid)
	if err != nil {
		return role, false, err
	}
	if !json.Valid(state) || !json.Valid(proof) {
		return role, false, fmt.Errorf("invalid Odyssey graduation JSON")
	}
	for i, q := range quests {
		if q == 0 || q == 65535 || (i > 0 && quests[i-1] >= q) {
			return role, false, fmt.Errorf("graduation quests must be sorted, unique and valid")
		}
	}
	if len(quests) > 0 {
		_, err = tx.Exec(ctx, `INSERT INTO character_quests(character_id,quest_id,status,config_version,progress,progress_model,completed_at)
 SELECT $1,q.id,'completed',$3,0,$4,now() FROM unnest($2::bigint[]) AS q(id)
 ON CONFLICT (character_id,quest_id) DO UPDATE SET status='completed',progress=0,
 progress_model=EXCLUDED.progress_model,config_version=EXCLUDED.config_version,completed_at=now()
 WHERE character_quests.status='accepted'`, id, idsToInt64(quests), version, OdysseyGraduationEvent)
		if err != nil {
			return role, false, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state); err != nil {
		return role, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,$2,$4)`, id, OdysseyGraduationEvent, version, proof); err != nil {
		return role, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return role, false, err
	}
	role.State = state
	return role, true, nil
}
