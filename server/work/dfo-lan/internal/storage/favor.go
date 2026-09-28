package storage

import (
	"context"
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

// GiveFavor 在同一事务内完成账号材料扣除、好感度加点和每日计数。
// consume 同时接收角色和账号共享材料存档；任一校验失败都会回滚两边，
// 防止出现“扣了无色但没有好感度”或反向的损档状态。
func (s *Store) GiveFavor(ctx context.Context, account, id int64, version string, npcID uint32, g FavorGift, consume func(Character, json.RawMessage) (json.RawMessage, json.RawMessage, error)) (Character, FavorState, json.RawMessage, error) {
	var role Character
	var st FavorState
	if g.Limit <= 0 || len(g.Levels) == 0 || g.MaxPoint <= 0 || g.Delta <= 0 || g.Day == "" || g.Now.IsZero() || consume == nil {
		return role, st, nil, fmt.Errorf("invalid favor gift")
	}
	tx, e := s.DB.Begin(ctx)
	if e != nil {
		return role, st, nil, e
	}
	defer tx.Rollback(ctx)
	e = tx.QueryRow(ctx, `SELECT id,account_id,wire_id,name,profession,create_request,config_version,state,created_at FROM characters WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL FOR UPDATE`, account, id).Scan(&role.ID, &role.AccountID, &role.WireID, &role.Name, &role.Profession, &role.Request, &role.ConfigVersion, &role.State, &role.CreatedAt)
	if e != nil {
		return role, st, nil, e
	}
	if role.ConfigVersion != version {
		return role, st, nil, fmt.Errorf("favor source mismatch")
	}
	if _, e = tx.Exec(ctx, `INSERT INTO account_material_storage(account_id) VALUES($1) ON CONFLICT(account_id) DO NOTHING`, account); e != nil {
		return role, st, nil, e
	}
	var counts json.RawMessage
	if e = tx.QueryRow(ctx, `SELECT counts FROM account_material_storage WHERE account_id=$1 FOR UPDATE`, account).Scan(&counts); e != nil {
		return role, st, nil, e
	}
	var point int64
	var day string
	var dailyCount int
	e = tx.QueryRow(ctx, `INSERT INTO npc_favor(character_id,npc_id,point,daily_count,last_gift_day) VALUES($1,$2,0,1,$3::date)
 ON CONFLICT(character_id,npc_id) DO UPDATE SET
 daily_count=CASE WHEN EXCLUDED.last_gift_day>npc_favor.last_gift_day OR npc_favor.last_gift_day IS NULL THEN 1 ELSE npc_favor.daily_count+1 END,
 last_gift_day=GREATEST(EXCLUDED.last_gift_day,npc_favor.last_gift_day),
 point=npc_favor.point,
 updated_at=now()
	 RETURNING point,daily_count,last_gift_day::text`, id, npcID, g.Day).Scan(&point, &dailyCount, &day)
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
	_, e = tx.Exec(ctx, `UPDATE characters SET state=$2 WHERE id=$1`, id, state)
	if e != nil {
		return role, st, nil, e
	}
	_, e = tx.Exec(ctx, `UPDATE account_material_storage SET counts=$2,updated_at=now() WHERE account_id=$1`, account, updatedCounts)
	if e != nil {
		return role, st, nil, e
	}
	_, e = tx.Exec(ctx, `UPDATE npc_favor SET point=$2,updated_at=now() WHERE character_id=$1 AND npc_id=$3`, id, next, npcID)
	if e != nil {
		return role, st, nil, e
	}
	if e = tx.Commit(ctx); e != nil {
		return role, st, nil, e
	}
	role.State = state
	s.Cache.Del(ctx, fmt.Sprintf("%scharacters:%d", s.prefix, account))
	st.Point = next
	for _, th := range g.Levels {
		if next >= th {
			st.Level++
		}
	}
	st.DailyCount = dailyCount
	return role, st, updatedCounts, nil
}
