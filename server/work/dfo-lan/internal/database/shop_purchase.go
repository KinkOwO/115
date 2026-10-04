package database

import (
	"context"
	"dfolan/internal/database/sqlcgen"
	"fmt"
	"time"
)

// NPC 商店限购。
//
// 当前原生目录由 ItemShopSourcePolicy 的 purchase_limit_mode=disabled 明确停用限购。
// 本次保留该策略，只修正持久化事务边界，不改变源规则或启用限购。
// 限购规则由 catalog 提供；storage 只接受作用域和时间窗口，不解释 PVF 字段。
//
// 这里只做「存 + 数」两件事，周期语义由调用方给定窗口起点（见 PeriodStart），
// 避免 storage 层依赖 catalog 的枚举。

// MigrateShopPurchases 建限购流水表。
//
// 为什么是新表而不是复用 character_events：那张表的主键是
// (character_id, event_key)，同一 key 只能落一行 —— 限购要的是**可累加的行**。
func (s *Store) MigrateShopPurchases(ctx context.Context) error {
	return s.execMigration(ctx, "0011_shop_purchases.sql")
}

// ShopPurchaseScope 决定「次数算在谁头上」。
type ShopPurchaseScope string

const (
	ShopScopeCharacter ShopPurchaseScope = "character"
	ShopScopeAccount   ShopPurchaseScope = "account"
)

// PeriodStart 返回限购窗口的起点（闭区间起点）。
//
//	"daily"   -> 当天 00:00
//	"weekly"  -> 本周一 00:00
//	"monthly" -> 本月 1 号 00:00
//	其它（含 "accumulate"/"version"/""）-> 零时间，即「从创建账号起累计」
//
// 用本地时区切分：源码里的 daily/weekly/monthly 是玩家视角的日历周期。
func PeriodStart(period string, now time.Time) time.Time {
	y, m, d := now.Date()
	switch period {
	case "daily":
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
	case "weekly":
		// Go 的 Weekday 里 Sunday=0；源码的「本周」按周一为首日。
		off := (int(now.Weekday()) + 6) % 7
		return time.Date(y, m, d, 0, 0, 0, 0, now.Location()).AddDate(0, 0, -off)
	case "monthly":
		return time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
	default:
		return time.Time{} // 累计：不限窗口
	}
}

// CountShopPurchases 统计窗口内已购次数。
//
// scope 为 account 时按账号统计（跨角色共享限购），否则按角色。
func (s *Store) CountShopPurchases(ctx context.Context, scope ShopPurchaseScope,
	accountID, characterID int64, npcID, template uint32, start time.Time) (int, error) {
	return countShopPurchases(ctx, s.queries, scope, accountID, characterID, npcID, template, start)
}

// CountShopPurchases reads through the same transaction as the save and ledger.
// The callback entry point holds its account lock before locking the character.
func (tx *Tx) CountShopPurchases(ctx context.Context, scope ShopPurchaseScope, npcID, template uint32, start time.Time) (int, error) {
	return countShopPurchases(ctx, tx.queries, scope, tx.accountID, tx.characterID, npcID, template, start)
}

func countShopPurchases(ctx context.Context, queries *sqlcgen.Queries, scope ShopPurchaseScope,
	accountID, characterID int64, npcID, template uint32, start time.Time) (int, error) {
	var count int64
	var err error
	if scope == ShopScopeAccount {
		count, err = queries.CountAccountShopPurchases(ctx, sqlcgen.CountAccountShopPurchasesParams{
			AccountID: accountID, NpcID: int32(npcID), Template: int32(template), WindowStart: start,
		})
	} else {
		count, err = queries.CountCharacterShopPurchases(ctx, sqlcgen.CountCharacterShopPurchasesParams{
			CharacterID: characterID, NpcID: int32(npcID), Template: int32(template), WindowStart: start,
		})
	}
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

// RecordShopPurchase is bound to the account and character whose save is locked.
// The enclosing event owns commit/rollback; replay never invokes this method.
func (tx *Tx) RecordShopPurchase(ctx context.Context, npcID, template uint32) error {
	err := tx.queries.RecordShopPurchase(ctx, sqlcgen.RecordShopPurchaseParams{
		AccountID: tx.accountID, CharacterID: tx.characterID, NpcID: int32(npcID), Template: int32(template),
	})
	if err != nil {
		return fmt.Errorf("record shop purchase: %w", err)
	}
	return nil
}
