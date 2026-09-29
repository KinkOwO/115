package storage

import (
	"context"
	"encoding/json"
)

// 登录时读取账号完整登记集合；旧账号缺省为空，不创建或改写任何进度。
func (s *Store) AdventureCollectionEquipment(ctx context.Context, account, character int64) (map[uint32]bool, error) {
	var raw []byte
	err := s.DB.QueryRow(ctx, `SELECT COALESCE(a.data->'collection_equipment','{}'::jsonb)
 FROM characters c LEFT JOIN account_adventures a ON a.account_id=c.account_id
 WHERE c.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL`, account, character).Scan(&raw)
	if err != nil {
		return nil, err
	}
	var equipment map[uint32]bool
	err = json.Unmarshal(raw, &equipment)
	return equipment, err
}

// 从账号收藏真源恢复任务进度，扣物提交后即使掉线也不要求重新登记。
func (s *Store) AdventureEquipmentRegistered(ctx context.Context, account, character int64, template uint32) (bool, error) {
	var registered bool
	err := s.DB.QueryRow(ctx, `SELECT COALESCE((a.data->'collection_equipment'->>($3::bigint::text))::boolean,false)
 FROM characters c LEFT JOIN account_adventures a ON a.account_id=c.account_id
 WHERE c.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL`, account, character, template).Scan(&registered)
	return registered, err
}
