package storage

import (
	"context"
	"fmt"
	"math"
)

// RecordCharacterFame只更新独立的最高名望列，不覆盖角色JSON或物品存档。
// GREATEST在数据库内执行，避免并发回写将已取得的最高值降低。
func (s *Store) RecordCharacterFame(ctx context.Context, account, characterID int64, current uint32) (uint32, error) {
	if account <= 0 || characterID <= 0 || current > math.MaxInt32 {
		return 0, fmt.Errorf("名望记录参数无效")
	}
	var highest uint32
	err := s.DB.QueryRow(ctx, `UPDATE characters SET max_fame=GREATEST(max_fame,$3)
 WHERE account_id=$1 AND id=$2 AND deleted_at IS NULL RETURNING max_fame`, account, characterID, int64(current)).Scan(&highest)
	return highest, err
}
