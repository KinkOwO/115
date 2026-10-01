package workflow

import (
	"context"
	"dfolan/internal/db"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// ShopService settles NPC purchases and sales using inventory rules.
type ShopService struct {
	inventory.ShopService
	Store *storage.Store
}

// shopEventSeq 是进程内的请求计数，只用于让事件键可读。
//
// 幂等键**必须**带时间戳：单靠这个自增计数，进程重启后它会归零并与重启前的
// 事件撞键。实机 2026-09-23（test-jh）就是这么复现的：23:57 买过一次
// 10417798（键 buy:10417798:1:1），00:09 重启服务端后 00:11 再买同一个盒子，
// 自增又给到 1 → 键完全相同 → CommitCharacterEvent 判定"已处理过"，于是不发货、
// 不改存档，却仍然回了成功 ack（用的是旧 receipt 里的 slot 66），客户端凭空画出
// 一个服务端并不存在的盒子，右键时被服务端如实拒绝（客户端显示「库存已满」）。
var shopEventSeq uint64
var shopBootStamp = time.Now().UnixNano()

func shopEventKey(op string, seq uint64, fields ...uint32) string {
	return shopEventKeyAt(shopBootStamp, op, seq, fields...)
}

func shopEventKeyAt(boot int64, op string, seq uint64, fields ...uint32) string {
	var sb strings.Builder
	sb.WriteString(op)
	for _, f := range fields {
		sb.WriteByte(':')
		sb.WriteString(strconv.FormatUint(uint64(f), 10))
	}
	sb.WriteByte(':')
	sb.WriteString(strconv.FormatInt(boot, 10))
	sb.WriteByte('-')
	sb.WriteString(strconv.FormatUint(seq, 10))
	return sb.String()
}

// checkShopLimit 在**事务内**校验限购并记录本次购买。
//
// ⚠️ **本仓取舍（单机化的体验改动，非原版设计）**：原版源里有 `[purchase limit]`，本仓**暂不实施** ——
// 目录里还没有 `limit_*` 字段（导入器未接），`PurchaseLimit` 因此恒返回 `ok=false`，本函数直接放行。
// 代码路径保留，接上数据即生效。详见 internal/storage/shop_purchase.go 的头部说明。
//
// 规格：物品自身 `.stk` 的 `[purchase limit] <scope> <period> <count>`，由
// cmd/itemshopimport 导进 itemshop 目录的 offer 字段（catalog.ItemShopOffer）。
//
//   - scope：account ⇒ 按账号累计（跨角色共享）；其余按角色。
//   - period：daily/weekly/monthly ⇒ 按日历窗口；accumulate/version/空 ⇒ 从首次购买起累计。
//   - 达上限 ⇒ **拒绝**本次购买（不回退到另一种支付方式，回退等于扣错东西）；
//   - 不限购（Limited=false）⇒ 直接放行，不写流水。
//
// 校验与记录都必须与背包变更在同一个事务里：否则会出现「货到手但次数没记」，
// 或重发请求重复计数（重发的同 key 请求走幂等回执，不会重跑 apply）。
func (s *ShopService) checkShopLimit(ctx context.Context, tx db.Tx, current storage.Character, shopID, template uint32) error {
	scope, period, count, limited := s.ItemShops.PurchaseLimit(shopID, template)
	if !limited {
		return nil
	}
	kind := storage.ShopPurchaseScope(scope)
	if kind != storage.ShopScopeAccount {
		kind = storage.ShopScopeCharacter
	}
	used, e := s.Store.CountShopPurchases(ctx, kind, current.AccountID, current.ID,
		shopID, template, storage.PeriodStart(period, time.Now()))
	if e != nil {
		return e
	}
	if uint32(used) >= count {
		return fmt.Errorf("shop %d template %d reached its purchase limit (%d/%d, %s %s)",
			shopID, template, used, count, scope, period)
	}
	return storage.RecordShopPurchase(ctx, tx, current.AccountID, current.ID, shopID, template)
}

func (s *ShopService) Buy(ctx context.Context, role storage.Character, r protocol.BuyItemRequest) (storage.Character, inventory.BuyReceipt, bool, error) {
	var out inventory.BuyReceipt
	fail := func(e error) (storage.Character, inventory.BuyReceipt, bool, error) {
		return role, out, false, e
	}
	if err := s.ValidateBuy(InventoryRole(role), r); err != nil {
		return fail(err)
	}
	seq := atomic.AddUint64(&shopEventSeq, 1)
	key := shopEventKey("buy", seq, r.Template, r.Count)
	// 商店的支付方式有两个「材料」来源：
	//   1. .shp 里店主写死的 [need material]（如奥德赛商店要银币）；
	//   2. ★ 物品脚本自带的 [need material] —— 商店表 .shp **没有价格字段**，
	//      所以用材料交换的商品，材料成本只写在物品脚本里（3242=1000×3037 等）。
	// 两者都没有才走金币价；金币价缺 [price] 时按物品基础价值 [value] 兜底
	// （价格表里 Sell=[value]/5，故用 Sell*5），再没有就是 0。
	// CMD21 的 p[8]/p[12] 都是 npc-like id，**哪个是商店取决于客户端从哪个 NPC 打开界面**
	// （2026-09-29 作者侧 42/42 样本实证，见 catalog.ItemShops.ResolveShop）：
	//   p8=100000694（场景物·圣诞树） p12=100001774（装备之力魔法书） -> 商店是 p12
	//   p8=100001019（奥德赛商店）      p12=100003035             -> 商店是 p8
	// 两个候选都不在商店表里时退回 NpcID，保持历史行为。
	plan, err := s.QuoteBuy(r)
	if err != nil {
		return fail(err)
	}
	shopID, mats := plan.ShopID, plan.Materials
	var saved storage.Character
	var applied bool
	if len(mats) > 0 {
		// ★ 材料支付：把「扣账号材料」与「改角色存档」放进**同一事务**
		// （CommitAccountMaterialEvent）。账号材料仓库（space 35）里存着 3033..3037 等
		// 共享晶块，它们平时不在角色背包里；旧路径只查背包 → 「背包里有晶块，商店却说 have 0」。
		var e error
		saved, _, applied, e = s.Store.CommitAccountMaterialEventTx(ctx, role.AccountID, role.ID,
			s.Catalog.Source.SaveIdentity(), key, s.EventModel,
			func(tx db.Tx, current storage.Character, rawCounts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
				if e := s.checkShopLimit(ctx, tx, current, shopID, r.Template); e != nil {
					return nil, nil, e
				}
				next, counts, receipt, e := s.ApplyBuyMaterials(InventoryRole(current), rawCounts, r, plan, seq)
				out = receipt
				return next, counts, e
			})
		if e != nil {
			return fail(e)
		}
		if applied && (out.Source != s.Catalog.Source.SaveIdentity() || out.Template != r.Template || out.Count != r.Count) {
			return fail(fmt.Errorf("buy receipt conflict"))
		}
	} else {
		var e error
		saved, applied, e = s.Store.CommitCharacterEventTx(ctx, role.AccountID, role.ID,
			s.Catalog.Source.SaveIdentity(), key, s.EventModel,
			func(tx db.Tx, current storage.Character) (json.RawMessage, json.RawMessage, error) {
				if e := s.checkShopLimit(ctx, tx, current, shopID, r.Template); e != nil {
					return nil, nil, e
				}
				next, receipt, e := s.ApplyBuyGold(InventoryRole(current), r, plan, seq)
				if e != nil {
					return nil, nil, e
				}
				out = receipt
				encoded, e := json.Marshal(receipt)
				return next, encoded, e
			})
		if e != nil {
			return fail(e)
		}
		receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
		if e != nil {
			return fail(e)
		}
		if e = json.Unmarshal(receipt, &out); e != nil {
			return fail(e)
		}
		if out.Source != s.Catalog.Source.SaveIdentity() || out.Template != r.Template || out.Count != r.Count {
			return fail(fmt.Errorf("buy receipt conflict"))
		}
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

func (s *ShopService) Sell(ctx context.Context, role storage.Character, r protocol.SellItemRequest) (storage.Character, inventory.SellReceipt, bool, error) {
	var out inventory.SellReceipt
	fail := func(e error) (storage.Character, inventory.SellReceipt, bool, error) {
		return role, out, false, e
	}
	if err := s.ValidateSell(InventoryRole(role), r); err != nil {
		return fail(err)
	}
	seq := atomic.AddUint64(&shopEventSeq, 1)
	key := shopEventKey("sell", seq, uint32(len(r.Rows)), uint32(r.Rows[0].Slot), uint32(r.Rows[len(r.Rows)-1].Slot))

	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.SaveIdentity(), key, s.EventModel,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			next, receipt, e := s.ApplySell(InventoryRole(current), r, seq)
			if e != nil {
				return nil, nil, e
			}
			out = receipt
			encoded, e := json.Marshal(receipt)
			return next, encoded, e
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &out); e != nil {
		return fail(e)
	}
	if out.Source != s.Catalog.Source.SaveIdentity() || len(out.Rows) != len(r.Rows) {
		return fail(fmt.Errorf("sell receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
