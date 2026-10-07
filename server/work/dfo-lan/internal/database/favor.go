package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"fmt"
	"time"
)

type FavorGift struct {
	Day      string  // 送礼日（yyyy-mm-dd）
	Limit    int     // 每日送礼次数上限
	Levels   []int64 // 好感度累计门槛（升序，来自 [favor level point down]）
	MaxPoint int64   // 好感度点数上限（最高门槛）
	Delta    int64   // 本次提升点数（礼物区间内随机）
	Now      time.Time
}

type FavorState struct {
	Point      int64
	Level      int64 // 已跨过的累计门槛数
	DailyCount int
}

// FavorPoint 是角色对单个 NPC 的已存好感度。
type FavorPoint struct {
	NPCID uint32
	Point int64
}

// ListFavor 返回该角色所有好感度非零的 NPC（按 npc_id 排序）。用于进城镇
// 时编码 NOTI733 全量好感度同步列表。
func (s *Store) ListFavor(ctx context.Context, characterID int64) ([]FavorPoint, error) {
	rows, e := s.queries.ListFavor(ctx, characterID)
	if e != nil {
		return nil, e
	}
	out := []FavorPoint{}
	for _, row := range rows {
		out = append(out, FavorPoint{NPCID: uint32(row.NpcID), Point: row.Point})
	}
	return out, nil
}

// GiveFavor 在同一事务内完成账号材料扣除、好感度加点和每日计数。
// consume 同时接收角色和账号共享材料存档；任一校验失败都会回滚两边，
// 防止出现“扣了无色但没有好感度”或反向的损档状态。
func (s *Store) GiveFavor(ctx context.Context, account, id int64, version string, npcID uint32, g FavorGift, consume func(Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (Character, FavorState, json.RawMessage, error) {
	var role Character
	var st FavorState
	if g.Limit <= 0 || len(g.Levels) == 0 || g.MaxPoint <= 0 || g.Delta <= 0 || g.Day == "" || g.Now.IsZero() || consume == nil {
		return role, st, nil, fmt.Errorf("invalid favor gift")
	}
	tx, e := s.engine.begin(ctx)
	if e != nil {
		return role, st, nil, e
	}
	defer tx.rollback(ctx)
	role, e = lockCharacter(ctx, tx, account, id)
	if e != nil {
		return role, st, nil, e
	}
	if role.ConfigVersion != version {
		return role, st, nil, fmt.Errorf("favor source mismatch")
	}
	queries := tx.queries()
	if e = queries.InitializeAccountMaterials(ctx, account); e != nil {
		return role, st, nil, e
	}
	var counts json.RawMessage
	if counts, e = queries.LockAccountMaterials(ctx, account); e != nil {
		return role, st, nil, e
	}
	row, e := queries.ReserveFavorGift(ctx, sqlcgen.ReserveFavorGiftParams{CharacterID: id, NpcID: int64(npcID), Day: g.Day})
	point, dailyCount, day := row.Point, int(row.DailyCount), row.LastGiftDay
	if e != nil {
		return role, st, nil, e
	}
	if day != g.Day {
		return role, st, nil, fmt.Errorf("favor clock moved backwards")
	}
	if dailyCount > g.Limit {
		return role, st, nil, fmt.Errorf("favor daily quota exhausted")
	}
	if point >= g.MaxPoint {
		// 用户要求去掉好感度上限：满值后仍可继续送礼，point 不截断，
		// 客户端百分比会超过 100%（一般被截断显示为 100%）。
		// 如需恢复原版行为，改为 return error。
	}
	state, updatedCounts, e := consume(role, counts)
	if e != nil {
		return role, st, nil, e
	}
	if !json.Valid(state) || !json.Valid(updatedCounts) {
		return role, st, nil, fmt.Errorf("invalid favor inventory")
	}
	next := point + g.Delta
	// 不截断到 MaxPoint：用户要求满值后仍可继续送礼。
	e = tx.queries().UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state})
	if e != nil {
		return role, st, nil, e
	}
	e = queries.SaveAccountMaterials(ctx, sqlcgen.SaveAccountMaterialsParams{AccountID: account, Counts: updatedCounts})
	if e != nil {
		return role, st, nil, e
	}
	e = queries.SaveFavorPoint(ctx, sqlcgen.SaveFavorPointParams{CharacterID: id, NpcID: int64(npcID), Point: next})
	if e != nil {
		return role, st, nil, e
	}
	if e = tx.commit(ctx); e != nil {
		return role, st, nil, e
	}
	role.State = state
	st.Point = next
	for _, th := range g.Levels {
		if next >= th {
			st.Level++
		}
	}
	st.DailyCount = dailyCount
	return role, st, updatedCounts, nil
}
