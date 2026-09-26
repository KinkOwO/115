package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

func (w *worldSession) buyItem(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, fmt.Errorf("buy item before character selection")
	}
	r, e := protocol.DecodeBuyItem(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	before, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	saved, receipt, applied, e := w.loot.Buy(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	if !applied {
		// 幂等命中 = 服务端没有发货。绝不能拿旧 receipt 回"成功 + 那个 slot"，那会
		// 让客户端画出一个存档里并不存在的物品（实机复现的"幽灵盒子/库存已满"）。
		return nil, fmt.Errorf("duplicate shop purchase request")
	}
	b, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}

	// Use the authoritative stack total for this slot so client batch purchases display correctly
	stackTotal := receipt.Count
	for _, it := range b.Items {
		if it.Slot == receipt.Slot {
			stackTotal = it.Amount
			break
		}
	}
	record := protocol.OrdinaryItem(receipt.Slot, r.Template, stackTotal)
	ack, e := protocol.BuyItemSuccess(r, record, b.Gold, receipt.Slot)
	if e != nil {
		return nil, e
	}
	update, e := protocol.InventoryUpdate(inventory.ChangedItemRows(before, b))
	if e != nil {
		return nil, e
	}
	w.role = saved
	return []outboundPacket{
		{"shop_buy_ack", 1, 21, ack},
		{"shop_buy_inventory_updated", 0, 14, update},
	}, nil
}

func (w *worldSession) sellItem(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, fmt.Errorf("sell item before character selection")
	}
	r, e := protocol.DecodeSellItem(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	before, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	saved, receipt, applied, e := w.loot.Sell(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	if !applied {
		return nil, fmt.Errorf("duplicate shop sale request")
	}
	ack, e := protocol.SellItemSuccess(receipt.NewGold, []protocol.SoldItem{{
		List:  r.List,
		Slot:  r.Slot,
		Count: receipt.Count,
	}})
	if e != nil {
		return nil, e
	}
	b, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	update, e := protocol.InventoryUpdate(inventory.ChangedItemRows(before, b))
	if e != nil {
		return nil, e
	}
	w.role = saved
	return []outboundPacket{
		{"shop_sell_ack", 1, 22, ack},
		{"shop_sell_inventory_updated", 0, 14, update},
	}, nil
}
