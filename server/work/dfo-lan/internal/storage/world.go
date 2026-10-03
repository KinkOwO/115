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
	if e != nil {
		return e
	}
	// 频道隔离的世界位置（业主 2026-10-02）。
	//
	// 征讨/军团这类特殊频道里，角色只能在自己的赛丽亚房间与副本门口活动；位置必须
	// 与该角色在普通城镇的位置**分开存**，否则切一次频道就把原位置覆盖掉（实际现象：
	// 切到沉月湖后右上角频道号变了、角色却还站在原地，或者反过来把城镇位置弄丢）。
	//
	// 普通频道（channel_type = 0 的语义）仍然走上面的 character_world；本表只在
	// 特殊频道使用，因此对已有存档是**纯新增**，不动任何既有行。
	_, e = s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_channel_world (
 character_id bigint NOT NULL REFERENCES characters(id),
 channel_type integer NOT NULL CHECK(channel_type>0 AND channel_type<=255),
 position jsonb NOT NULL, config_version text NOT NULL,
 revision bigint NOT NULL DEFAULT 1 CHECK(revision>0),
 updated_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(character_id, channel_type));`)
	return e
}

// LoadWorld 取角色的世界位置。
//
// channelType = 0 表示"普通频道"，用共享的 character_world；非 0 表示该特殊频道的
// 独立位置，用 character_channel_world（按 角色 + 频道 一行）。首次进入某频道时用
// 调用方给的 initial —— 对该频道而言就是它自己的赛丽亚房间。
func (s *Store) LoadWorld(ctx context.Context, account, characterID int64, channelType uint32, initial WorldPosition, version string) (WorldState, error) {
	var out WorldState
	if len(version) != 64 {
		return out, errors.New("invalid world configuration version")
	}
	if channelType > 255 {
		return out, errors.New("invalid channel type")
	}
	b, e := json.Marshal(initial)
	if e != nil {
		return out, e
	}
	if channelType == 0 {
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
	// SemiRaid/Legion 频道：位置**不持久化恢复**（业主 2026-10-03 口径）——
	// 每次进频道都落在该频道等候区的标准入口（initial，取自 towns[Type].Spawn
	// 的矩形中心），不恢复上次离开位置。表里仍记录最近一次进入位置（审计用）。
	_, e = s.DB.Exec(ctx, `INSERT INTO character_channel_world(character_id,channel_type,position,config_version)
 SELECT id,$3,$4,$5 FROM characters WHERE id=$2 AND account_id=$1
 ON CONFLICT (character_id, channel_type) DO UPDATE SET position=$4, updated_at=now()`, account, characterID, channelType, b, version)
	if e != nil {
		return out, e
	}
	out.Position = initial
	out.ConfigVersion = version
	out.Revision = 1
	return out, nil
}

// SaveWorld 落库角色位置。channelType = 0（普通频道）写共享 character_world 并按
// revision 乐观并发；非 0（SemiRaid/Legion 频道）**不落库** —— 位置只在会话内
// 流动，每次进频道重置到等候区标准入口（见 LoadWorld，业主 2026-10-03 口径）。
func (s *Store) SaveWorld(ctx context.Context, account, characterID int64, channelType uint32, old WorldState, next WorldPosition) (WorldState, error) {
	out := old
	if channelType != 0 {
		out.Position = next
		out.Revision++
		return out, nil
	}
	if channelType > 255 {
		return out, errors.New("invalid channel type")
	}
	b, e := json.Marshal(next)
	if e != nil {
		return out, e
	}
	if channelType == 0 {
		tag, e := s.DB.Exec(ctx, `UPDATE character_world w SET position=$4,revision=revision+1,updated_at=now()
 FROM characters c WHERE c.id=w.character_id AND c.account_id=$1 AND c.id=$2 AND w.revision=$3`, account, characterID, old.Revision, b)
		if e != nil {
			return out, e
		}
		if tag.RowsAffected() != 1 {
			return out, ErrWorldConflict
		}
	} else {
		tag, e := s.DB.Exec(ctx, `UPDATE character_channel_world w SET position=$5,revision=revision+1,updated_at=now()
 FROM characters c WHERE c.id=w.character_id AND c.account_id=$1 AND c.id=$2 AND w.channel_type=$3 AND w.revision=$4`, account, characterID, channelType, old.Revision, b)
		if e != nil {
			return out, e
		}
		if tag.RowsAffected() != 1 {
			return out, ErrWorldConflict
		}
	}
	out.Position = next
	out.Revision++
	return out, nil
}

// ScrubPollutedWorldPositions 删除普通频道共享行里落在特殊征讨频道专属城镇的
// 位置（历史污染：会话位置隔离修复之前，月湖 215 / Azure 213 / 军团 239 的会话
// 位置曾被写进共享行）。删除后玩家下一次进普通频道走默认落点重建，无资产损失；
// 特殊频道自己的 character_channel_world 行不受影响。幂等：已干净的库影响 0 行。
func (s *Store) ScrubPollutedWorldPositions(ctx context.Context, towns []uint32) (int64, error) {
	if len(towns) == 0 {
		return 0, nil
	}
	ids := make([]int64, 0, len(towns))
	for _, t := range towns {
		ids = append(ids, int64(t))
	}
	tag, err := s.DB.Exec(ctx,
		`DELETE FROM character_world WHERE (position->>'town')::bigint = ANY($1::bigint[])`, ids)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
