package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
	"math"
)

// RecordCharacterFame只更新独立的最高名望列，不覆盖角色JSON或物品存档。
// GREATEST在数据库内执行，避免并发回写将已取得的最高值降低。
func (s *Store) RecordCharacterFame(ctx context.Context, account, characterID int64, current uint32) (uint32, error) {
	if account <= 0 || characterID <= 0 || current > math.MaxInt32 {
		return 0, fmt.Errorf("名望记录参数无效")
	}
	highest, err := s.queries.RecordCharacterFame(ctx, sqlcgen.RecordCharacterFameParams{
		AccountID: account, CharacterID: characterID, CurrentFame: int32(current),
	})
	return uint32(highest), err
}
