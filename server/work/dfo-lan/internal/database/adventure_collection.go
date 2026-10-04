package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"encoding/json"
)

// 登录时读取账号完整登记集合；旧账号缺省为空，不创建或改写任何进度。
func (s *Store) AdventureCollectionEquipment(ctx context.Context, account, character int64) (map[uint32]bool, error) {
	raw, err := s.queries.AdventureCollectionEquipment(ctx, sqlcgen.AdventureCollectionEquipmentParams{AccountID: account, CharacterID: character})
	if err != nil {
		return nil, storageError(err)
	}
	var equipment map[uint32]bool
	err = json.Unmarshal(raw, &equipment)
	return equipment, err
}

// 从账号收藏真源恢复任务进度，扣物提交后即使掉线也不要求重新登记。
func (s *Store) AdventureEquipmentRegistered(ctx context.Context, account, character int64, template uint32) (bool, error) {
	registered, err := s.queries.AdventureEquipmentRegistered(ctx, sqlcgen.AdventureEquipmentRegisteredParams{AccountID: account, CharacterID: character, Template: int64(template)})
	return registered, storageError(err)
}
