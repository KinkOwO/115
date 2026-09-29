package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// NPC 商店限购。
//
// ⚠️ **本仓取舍（单机化的体验改动，非原版设计）**：原版源里确实有 `[purchase limit]`
// （全库 account/accumulate 1215、weekly 652、daily 270、monthly 121 …），但本仓**暂不实施限购** ——
// `configs/itemshop-candidate.json` 目前不含 `limit_*` 字段（`cmd/itemshopimport` 还没解析物品 `.stk` 的
// `[purchase limit]`），于是 `catalog.ItemShops.PurchaseLimit` 恒返回 `ok=false`、每条购买都放行。
// 这是**我们为单机体验做的取舍**，不是原版行为；代码路径完整保留，接上数据即生效，不必改业务逻辑。
//
// 来源：物品自身 `.stk` 的 `[purchase limit] <scope> <period> <count>`，
// 由 cmd/itemshopimport 导进 configs/itemshop-candidate.json 的 offer 字段，
// 运行时由 internal/loot 的 Buy 校验。
//
// 这里只做「存 + 数」两件事，周期语义由调用方给定窗口起点（见 PeriodStart），
// 避免 storage 层依赖 catalog 的枚举。

// MigrateShopPurchases 建限购流水表。
//
// 为什么是新表而不是复用 character_events：那张表的主键是
// (character_id, event_key)，同一 key 只能落一行 —— 限购要的是**可累加的行**。
func (s *Store) MigrateShopPurchases(ctx context.Context) error {
	_, e := s.DB.Exec(ctx, `CREATE TABLE IF NOT EXISTS character_shop_purchases (
 character_id bigint NOT NULL REFERENCES characters(id),
 account_id bigint NOT NULL,
 npc_id integer NOT NULL,
 template integer NOT NULL,
 bought_at timestamptz NOT NULL DEFAULT now());`)
	if e != nil {
		return e
	}
	_, e = s.DB.Exec(ctx, `CREATE INDEX IF NOT EXISTS character_shop_purchases_window
 ON character_shop_purchases(character_id, npc_id, template, bought_at);`)
	return e
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
	var n int
	var e error
	if scope == ShopScopeAccount {
		e = s.DB.QueryRow(ctx, `SELECT count(*) FROM character_shop_purchases
 WHERE account_id=$1 AND npc_id=$2 AND template=$3 AND bought_at >= $4`,
			accountID, int32(npcID), int32(template), start).Scan(&n)
	} else {
		e = s.DB.QueryRow(ctx, `SELECT count(*) FROM character_shop_purchases
 WHERE character_id=$1 AND npc_id=$2 AND template=$3 AND bought_at >= $4`,
			characterID, int32(npcID), int32(template), start).Scan(&n)
	}
	if e != nil {
		return 0, e
	}
	return n, nil
}

// RecordShopPurchase 记一次购买。必须与背包变更在**同一事务**里，
// 否则会出现「货到手但次数没记」或反之。
func RecordShopPurchase(ctx context.Context, tx pgx.Tx, accountID, characterID int64, npcID, template uint32) error {
	_, e := tx.Exec(ctx, `INSERT INTO character_shop_purchases(account_id, character_id, npc_id, template)
 VALUES($1,$2,$3,$4)`, accountID, characterID, int32(npcID), int32(template))
	if e != nil {
		return fmt.Errorf("record shop purchase: %w", e)
	}
	return nil
}
