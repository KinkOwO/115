package loot

import (
	"context"
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

// shopEventSeq 是进程内的请求计数，只用于让事件键可读。
//
// 幂等键**必须**带时间戳：单靠这个自增计数，进程重启后它会归零并与重启前的
// 事件撞键。实机 2026-09-23（test-jh）就是这么复现的：23:57 买过一次
// 10417798（键 buy:10417798:1:1），00:09 重启服务端后 00:11 再买同一个盒子，
// 自增又给到 1 → 键完全相同 → CommitCharacterEvent 判定"已处理过"，于是不发货、
// 不改存档，却仍然回了成功 ack（用的是旧 receipt 里的 slot 66），客户端凭空画出
// 一个服务端并不存在的盒子，右键时被服务端如实拒绝（客户端显示「库存已满」）。
var shopEventSeq uint64

const shopUnitPrice = 1

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

type SellReceipt struct {
	NpcID      uint32 `json:"npc_id"`
	Slot       uint16 `json:"slot"`
	Template   uint32 `json:"template"`
	Count      uint32 `json:"count"`
	GoldGained uint32 `json:"gold_gained"`
	NewGold    uint32 `json:"new_gold"`
	Source     string `json:"source"`
	Seq        uint64 `json:"seq"`
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

func (s *Service) Buy(ctx context.Context, role storage.Character, r protocol.BuyItemRequest) (storage.Character, BuyReceipt, bool, error) {
	var out BuyReceipt
	fail := func(e error) (storage.Character, BuyReceipt, bool, error) {
		return role, out, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("buy source mismatch"))
	}
	seq := atomic.AddUint64(&shopEventSeq, 1)
	key := shopEventKey("buy", seq, r.Template, r.Count)
	cost := r.Count * shopUnitPrice

	var stackableType string
	if item, ok := s.Catalog.Items[r.Template]; ok {
		stackableType = item.StackableType
	}

	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, s.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			var slot uint16
			if mats, _, paid := s.ItemShops.Materials(r.NpcID, r.Template); paid {
				// 源用 [need material] 定价的商品（奥德赛商店的银币/金币）：按材料
				// 支付，不再扣金币。数量不足时 PayMaterials 整笔拒绝。
				costs := make([]inventory.MaterialCost, 0, len(mats))
				for _, m := range mats {
					costs = append(costs, inventory.MaterialCost{Template: m.Template, Count: m.Count})
				}
				b, slot, e = b.BuyWithMaterials(s.BagRules, r.Template, r.Count, costs, stackableType)
			} else {
				b, slot, e = b.Buy(s.BagRules, r.Template, r.Count, cost, stackableType)
			}
			if e != nil {
				return nil, nil, e
			}
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			out = BuyReceipt{
				NpcID:    r.NpcID,
				Template: r.Template,
				Count:    r.Count,
				Slot:     slot,
				Cost:     cost,
				NewGold:  b.Gold,
				Source:   s.Catalog.Source.Checksum,
				Seq:      seq,
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
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

// Sell processes an NPC shop item sale transaction durably.
func (s *Service) Sell(ctx context.Context, role storage.Character, r protocol.SellItemRequest) (storage.Character, SellReceipt, bool, error) {
	var out SellReceipt
	fail := func(e error) (storage.Character, SellReceipt, bool, error) {
		return role, out, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("sell source mismatch"))
	}
	seq := atomic.AddUint64(&shopEventSeq, 1)
	key := shopEventKey("sell", seq, uint32(r.List), uint32(r.Slot))

	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, s.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			b, template, goldGained, e := b.Sell(s.BagRules, r.List, r.Slot, r.Count, shopUnitPrice)
			if e != nil {
				return nil, nil, e
			}
			updated, e := inventory.SaveBag(current.State, b)
			if e != nil {
				return nil, nil, e
			}
			out = SellReceipt{
				NpcID:      r.NpcID,
				Slot:       r.Slot,
				Template:   template,
				Count:      r.Count,
				GoldGained: goldGained,
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
	if out.Source != s.Catalog.Source.Checksum || out.Slot != r.Slot || out.Count != r.Count {
		return fail(fmt.Errorf("sell receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
