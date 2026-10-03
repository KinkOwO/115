package storage

import (
	"context"
	"dfolan/internal/world"
	"encoding/json"
	"errors"
)

// 世界位置类型归 internal/world 拥有；这里保留类型别名（迁移期），
// 使调用点无需一次性改动，且持久化方向为 storage -> world（契约 R3 允许）。
type WorldPosition = world.WorldPosition
type WorldReturn = world.WorldReturn
type WorldState = world.WorldState

var ErrWorldConflict = errors.New("world position changed concurrently")

func (s *Store) MigrateWorld(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_world (
 character_id bigint PRIMARY KEY REFERENCES characters(id),
 position jsonb NOT NULL, config_version text NOT NULL,
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 updated_at timestamptz NOT NULL DEFAULT now());`)
	return e
}
func (s *Store) LoadWorld(ctx context.Context, account, characterID int64, initial WorldPosition, version string) (WorldState, error) {
	var out WorldState
	if len(version) != 64 {
		return out, errors.New("invalid world configuration version")
	}
	b, e := json.Marshal(initial)
	if e != nil {
		return out, e
	}
	_, e = s.DB.Exec(ctx, `INSERT INTO character_world(character_id,position,config_version)
 SELECT id,$3,$4 FROM characters WHERE id=$2 AND account_id=$1 ON CONFLICT DO NOTHING`, account, characterID, b, version)
	if e != nil {
		return out, e
	}
	var raw []byte
	e = s.DB.QueryRow(ctx, `SELECT w.position,w.revision,w.config_version FROM character_world w JOIN characters c ON c.id=w.character_id WHERE c.account_id=$1 AND c.id=$2`, account, characterID).Scan(&raw, &out.Revision, &out.ConfigVersion)
	if e == nil {
		e = json.Unmarshal(raw, &out.Position)
	}
	return out, e
}
func (s *Store) SaveWorld(ctx context.Context, account, characterID int64, old WorldState, next WorldPosition) (WorldState, error) {
	out := old
	b, e := json.Marshal(next)
	if e != nil {
		return out, e
	}
	tag, e := s.DB.Exec(ctx, `UPDATE character_world w SET position=$4,revision=revision+1,updated_at=now()
 FROM characters c WHERE c.id=w.character_id AND c.account_id=$1 AND c.id=$2 AND w.revision=$3`, account, characterID, old.Revision, b)
	if e != nil {
		return out, e
	}
	if tag.RowsAffected() != 1 {
		return out, ErrWorldConflict
	}
	out.Position = next
	out.Revision++
	return out, nil
}
