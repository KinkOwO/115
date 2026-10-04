package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
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
	return s.execMigration(ctx, "0019_world.sql")
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
		e = s.queries.EnsureWorld(ctx, sqlcgen.EnsureWorldParams{AccountID: account, CharacterID: characterID, Position: b, ConfigVersion: version})
		if e != nil {
			return out, e
		}
		row, err := s.queries.LoadWorld(ctx, sqlcgen.LoadWorldParams{AccountID: account, CharacterID: characterID})
		e = err
		out.Revision, out.ConfigVersion = row.Revision, row.ConfigVersion
		if e == nil {
			e = json.Unmarshal(row.Position, &out.Position)
		}
		return out, storageError(e)
	}
	// SemiRaid/Legion 频道：位置**不持久化恢复**（业主 2026-10-03 口径）——
	// 每次进频道都落在该频道等候区的标准入口（initial，取自 towns[Type].Spawn
	// 的矩形中心），不恢复上次离开位置。表里仍记录最近一次进入位置（审计用）。
	e = s.queries.EnterChannelWorld(ctx, sqlcgen.EnterChannelWorldParams{AccountID: account, CharacterID: characterID, ChannelType: int32(channelType), Position: b, ConfigVersion: version})
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
	b, e := json.Marshal(next)
	if e != nil {
		return out, e
	}
	changed, e := s.queries.SaveWorld(ctx, sqlcgen.SaveWorldParams{AccountID: account, CharacterID: characterID, Revision: old.Revision, Position: b})
	if e != nil {
		return out, e
	}
	if changed != 1 {
		return out, ErrWorldConflict
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
	return s.queries.ScrubPollutedWorldPositions(ctx, ids)
}
