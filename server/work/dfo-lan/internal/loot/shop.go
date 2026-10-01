package loot

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
)

// shopEventSeq 是进程内的请求计数，只用于让事件键可读。
//
// 幂等键**必须**带时间戳：单靠这个自增计数，进程重启后它会归零并与重启前的
// 事件撞键。实机 2026-09-23（test-jh）就是这么复现的：23:57 买过一次
// 10417798（键 buy:10417798:1:1），00:09 重启服务端后 00:11 再买同一个盒子，
// 自增又给到 1 → 键完全相同 → CommitCharacterEvent 判定"已处理过"，于是不发货、
// 不改存档，却仍然回了成功 ack（用的是旧 receipt 里的 slot 66），客户端凭空画出
// 一个服务端并不存在的盒子，右键时被服务端如实拒绝（客户端显示「库存已满」）。
var shopEventSeq uint64

type BuyReceipt struct {
	NpcID    uint32 `json:"npc_id"`
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
	Slot     uint16 `json:"slot"`
	Cost     uint32 `json:"cost"`
	NewGold  uint32 `json:"new_gold"`
	Source   string `json:"source"`
	Seq      uint64 `json:"seq"`
}

// SoldRowReceipt records one row of a completed sale.
type SoldRowReceipt struct {
	List      byte   `json:"list"`
	Slot      uint16 `json:"slot"`
	Template  uint32 `json:"template"`
	Count     uint32 `json:"count"`
	UnitPrice uint32 `json:"unit_price"`
}

type SellReceipt struct {
	NpcID      uint32           `json:"npc_id"`
	Rows       []SoldRowReceipt `json:"rows"`
	GoldGained uint32           `json:"gold_gained"`
	NewGold    uint32           `json:"new_gold"`
	Source     string           `json:"source"`
	Seq        uint64           `json:"seq"`
}

// Buy processes an NPC shop purchase transaction durably.
// Uses a process-level monotonic sequence to prevent idempotency key collision
// during rapid burst purchases.
// shopBootStamp 标记本次进程启动，与进程内计数一起保证事件键跨重启不复用。
//
// 不能只用 time.Now() 的纳秒值当"唯一"成分：Windows 的时钟精度在毫秒级，同一毫秒
// 内两次调用会拿到完全相同的值（本用例第一版就是这么失败的）。
var shopBootStamp = time.Now().UnixNano()

// shopEventKey 生成商店事件的幂等键：请求内容 + 本次进程启动标记 + 进程内计数。
//
// 进程内计数会随重启归零，所以必须再带上启动标记——只用计数会让重启后的新购买
// 命中重启前的旧事件（实机 2026-09-23 的幽灵盒子：重启后买同一个盒子得到与重启前
// 相同的键，于是判定"已处理"、不发货，却仍回了成功 ack）。
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

func (s *Service) shopPrice(template uint32) (catalog.ShopPrice, error) {
	if s.Prices != nil && s.Prices.Source == s.Catalog.Source.Checksum {
		if price, ok := s.Prices.Items[template]; ok {
			return price, nil
		}
	}
	return catalog.ShopPrice{}, fmt.Errorf("missing current-source shop price for item %d", template)
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
func (s *Service) checkShopLimit(ctx context.Context, tx pgx.Tx, current storage.Character, shopID, template uint32) error {
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

func (s *Service) Buy(ctx context.Context, role storage.Character, r protocol.BuyItemRequest) (storage.Character, BuyReceipt, bool, error) {
	var out BuyReceipt
	fail := func(e error) (storage.Character, BuyReceipt, bool, error) {
		return role, out, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("buy source mismatch"))
	}
	if s.Catalog.HasRuntimeDetails() && s.Catalog.Items[r.Template].Kind == "stackable" {
		if _, err := s.Catalog.ItemScript(r.Template); err != nil {
			return fail(err)
		}
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
	shopID := r.NpcID
	if id, ok := s.ItemShops.ResolveShop(r.NpcID, r.ActorID); ok {
		shopID = id
	}
	shopMats, _, shopPaid := s.ItemShops.Materials(shopID, r.Template)
	itemMats, itemPaid := s.ItemMaterials.Materials(r.Template)
	var mats []inventory.MaterialCost
	switch {
	case shopPaid:
		for _, m := range shopMats {
			mats = append(mats, inventory.MaterialCost{Template: m.Template, Count: m.Count})
		}
	case itemPaid:
		for _, m := range itemMats {
			mats = append(mats, inventory.MaterialCost{Template: m.Template, Count: m.Count})
		}
	}
	var cost uint32
	if len(mats) == 0 {
		p, err := s.shopPrice(r.Template)
		if err != nil {
			return fail(err)
		}
		buy := p.Buy
		if buy == nil {
			fallback := p.Sell * 5
			buy = &fallback
		}
		if r.Count == 0 || uint64(*buy)*uint64(r.Count) > math.MaxUint32 {
			return fail(fmt.Errorf("missing or overflowing source purchase price"))
		}
		cost = *buy * r.Count
	}

	var stackableType string
	if item, ok := s.Catalog.Items[r.Template]; ok {
		stackableType = item.StackableType
	}

	var saved storage.Character
	var applied bool
	if len(mats) > 0 {
		// ★ 材料支付：把「扣账号材料」与「改角色存档」放进**同一事务**
		// （CommitAccountMaterialEvent）。账号材料仓库（space 35）里存着 3033..3037 等
		// 共享晶块，它们平时不在角色背包里；旧路径只查背包 → 「背包里有晶块，商店却说 have 0」。
		var e error
		saved, _, applied, e = s.Store.CommitAccountMaterialEventTx(ctx, role.AccountID, role.ID,
			s.Catalog.Source.Checksum, key, s.Rules.Model,
			func(tx pgx.Tx, current storage.Character, rawCounts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
				if e := s.checkShopLimit(ctx, tx, current, shopID, r.Template); e != nil {
					return nil, nil, e
				}
				b, e := inventory.ReadBag(current.State)
				if e != nil {
					return nil, nil, e
				}
				m, e := inventory.ReadAccountMaterials(rawCounts)
				if e != nil {
					return nil, nil, e
				}
				b, m, slot, e := b.BuyWithMaterialsWithStore(s.BagRules, m, r.Template, r.Count, mats, stackableType)
				if e != nil {
					return nil, nil, e
				}
				state, e := inventory.SaveBag(current.State, b)
				if e != nil {
					return nil, nil, e
				}
				updated, e := m.Save()
				if e != nil {
					return nil, nil, e
				}
				out = BuyReceipt{
					NpcID: r.NpcID, Template: r.Template, Count: r.Count, Slot: slot,
					Cost: cost, NewGold: b.Gold, Source: s.Catalog.Source.Checksum, Seq: seq,
				}
				return state, updated, nil
			})
		if e != nil {
			return fail(e)
		}
		if applied && (out.Source != s.Catalog.Source.Checksum || out.Template != r.Template || out.Count != r.Count) {
			return fail(fmt.Errorf("buy receipt conflict"))
		}
	} else {
		var e error
		saved, applied, e = s.Store.CommitCharacterEventTx(ctx, role.AccountID, role.ID,
			s.Catalog.Source.Checksum, key, s.Rules.Model,
			func(tx pgx.Tx, current storage.Character) (json.RawMessage, json.RawMessage, error) {
				if e := s.checkShopLimit(ctx, tx, current, shopID, r.Template); e != nil {
					return nil, nil, e
				}
				b, e := inventory.ReadBag(current.State)
				if e != nil {
					return nil, nil, e
				}
				b, slot, e := b.Buy(s.BagRules, r.Template, r.Count, cost, stackableType)
				if e != nil {
					return nil, nil, e
				}
				updated, e := inventory.SaveBag(current.State, b)
				if e != nil {
					return nil, nil, e
				}
				out = BuyReceipt{
					NpcID: r.NpcID, Template: r.Template, Count: r.Count, Slot: slot,
					Cost: cost, NewGold: b.Gold, Source: s.Catalog.Source.Checksum, Seq: seq,
				}
				receipt, e := json.Marshal(out)
				return updated, receipt, e
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
		if out.Source != s.Catalog.Source.Checksum || out.Template != r.Template || out.Count != r.Count {
			return fail(fmt.Errorf("buy receipt conflict"))
		}
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

// Sell processes an NPC shop item sale transaction durably. One request may
// carry several rows (the "Sell All" panel registers multiple stacks and
// confirms with a single CMD22), so every row is removed in the same
// transaction: the sale either applies in full or not at all.
func (s *Service) Sell(ctx context.Context, role storage.Character, r protocol.SellItemRequest) (storage.Character, SellReceipt, bool, error) {
	var out SellReceipt
	fail := func(e error) (storage.Character, SellReceipt, bool, error) {
		return role, out, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("sell source mismatch"))
	}
	if len(r.Rows) == 0 {
		return fail(fmt.Errorf("sell requires at least one row"))
	}
	seq := atomic.AddUint64(&shopEventSeq, 1)
	key := shopEventKey("sell", seq, uint32(len(r.Rows)), uint32(r.Rows[0].Slot), uint32(r.Rows[len(r.Rows)-1].Slot))

	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, s.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			rows := make([]SoldRowReceipt, 0, len(r.Rows))
			var gained uint32
			for _, row := range r.Rows {
				// Resolve the identity from the transaction's current owned bag, never
				// from request metadata or a potentially stale session snapshot.
				_, template, _, e := b.Sell(s.BagRules, row.List, row.Slot, row.Count, 0)
				if e != nil {
					return nil, nil, e
				}
				price, e := s.shopPrice(template)
				if e != nil {
					return nil, nil, e
				}
				next, template, goldGained, e := b.Sell(s.BagRules, row.List, row.Slot, row.Count, price.Sell)
				if e != nil {
					return nil, nil, e
				}
				b = next
				gained += goldGained
				rows = append(rows, SoldRowReceipt{
					List:      row.List,
					Slot:      row.Slot,
					Template:  template,
					Count:     row.Count,
					UnitPrice: price.Sell,
				})
			}
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			out = SellReceipt{
				NpcID:      r.NpcID,
				Rows:       rows,
				GoldGained: gained,
				NewGold:    b.Gold,
				Source:     s.Catalog.Source.Checksum,
				Seq:        seq,
			}
			receipt, e := json.Marshal(out)
			return updated, receipt, e
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
	if out.Source != s.Catalog.Source.Checksum || len(out.Rows) != len(r.Rows) {
		return fail(fmt.Errorf("sell receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
