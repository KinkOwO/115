package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
	"fmt"
)

// SaveCharacterState 原子地重写一个角色的状态 JSON，**不带事件键**。
//
// 与 `CommitCharacterEvent` 的分工：
//
//	CommitCharacterEvent  按事件键幂等、apply **只在首次**执行 —— 适合「一次操作入一次账」；
//	SaveCharacterState    没有键，apply 必须是**当前状态的纯函数** —— 适合**周期性**状态
//	                      （在线时长这类每 30 秒要落的），重放不会叠加、也不撑事件账本。
//
// 实现照 `CommitAccountMaterialSweep` 的范式：进门事务 → `lockCharacter`（角色行 FOR UPDATE）
// → 校验 ConfigVersion → apply → `UpdateCharacterState` → 提交。**只用既有 query，不新增 SQL。**
func (s *Store) SaveCharacterState(ctx context.Context, account, id int64, version string,
	apply func(Character) (json.RawMessage, error)) (Character, error) {
	var role Character
	if apply == nil {
		return role, fmt.Errorf("character state save needs an apply function")
	}
	tx, e := s.engine.begin(ctx)
	if e != nil {
		return role, e
	}
	defer tx.rollback(ctx)
	role, e = lockCharacter(ctx, tx, account, id)
	if e != nil {
		return role, e
	}
	if role.ConfigVersion != version {
		return role, fmt.Errorf("character state save source mismatch")
	}
	state, e := apply(role)
	if e != nil {
		return role, e
	}
	if len(state) == 0 || !json.Valid(state) {
		return role, fmt.Errorf("invalid character state JSON")
	}
	if e = tx.queries().UpdateCharacterState(ctx, sqlcgen.UpdateCharacterStateParams{CharacterID: id, State: state}); e != nil {
		return role, e
	}
	if e = tx.commit(ctx); e != nil {
		return role, e
	}
	role.State = state
	return role, nil
}
