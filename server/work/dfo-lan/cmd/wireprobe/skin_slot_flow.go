package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

// stackableAction 分发 CMD507（USE_STACKABLE_ACTION）。客户端对同一个 opcode
// 复用多种动作：54 是疲劳恢复药水，197 是宠物幻化栏券（10309084）。
func (w *worldSession) stackableAction(p []byte) ([]outboundPacket, error) {
	slot, action, e := protocol.DecodeStackableAction(p)
	if e != nil {
		return nil, e
	}
	switch action {
	case protocol.ActionRecoverFatigue:
		return w.recoverFatiguePotion(p)
	case protocol.ActionOpenCreatureSkinSlot:
		return w.expandSkinSlot(p, slot)
	}
	return nil, fmt.Errorf("unsupported stackable action %d", action)
}

// expandSkinSlot 处理宠物幻化栏扩展券的 CMD507 形态（动作 197）。请求里带了券的
// 真实槽位，所以券源可以直接核对，并由券的模板反查出要开的那一位（宠物券 bit5）。
func (w *worldSession) expandSkinSlot(p []byte, slot uint16) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.characters == nil || w.characters.Store == nil {
		return nil, fmt.Errorf("skin slot expansion before character selection")
	}
	before, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	var mask byte
	for _, item := range before.Items {
		if item.Slot == slot {
			if m, ok := inventory.SkinSlotMaskForTicket(item.Template); ok {
				mask = m
			}
			break
		}
	}
	if mask == 0 {
		return nil, fmt.Errorf("slot %d does not hold a skin slot ticket", slot)
	}
	return w.unlockSkinSlot(slot, mask, fmt.Sprintf("skin-slot-expand:%d:%x", w.role.ID, sha256.Sum256(p)))
}

// unlockSkinSlot 是落地：扣一张券、置 USERINFO1 位、补发权威刷新。开启状态不是
// 独立字段，而是塞在 USERINFO1 的解锁字节里：原生 14563d692 把它直接写进角色的
// +0x198，宠物幻化栏读 bit5（145f02500）。因此置位后必须重发一次 mode0+mode1 的
// USERINFO1，见 unlockRefresh。
func (w *worldSession) unlockSkinSlot(ticketSlot uint16, mask byte, key string) ([]outboundPacket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if ticketSlot == 0 {
		before, e := inventory.ReadBag(w.role.State)
		if e != nil {
			return nil, e
		}
		slot, ok := inventory.SkinSlotTicketSlot(before, mask)
		if !ok {
			return nil, fmt.Errorf("no ticket for unlock mask %#02x in the main bag", mask)
		}
		ticketSlot = slot
	}
	saved, applied, e := w.characters.ExpandSkinSlot(ctx, w.role, key, mask)
	if e != nil {
		return nil, e
	}
	saved.WireID = w.role.WireID
	w.role = saved
	plan := []outboundPacket{}
	if applied {
		bag, e := inventory.ReadBag(saved.State)
		if e != nil {
			return nil, e
		}
		// 花掉的那一格用绝对行回给客户端：数量归零时发一个显式空行，否则客户端会
		// 留着最后一张券的图标（与疲劳药水路径同一范式）。
		row := protocol.EmptyOrdinaryItem(ticketSlot)
		for _, item := range bag.Items {
			if item.Slot == ticketSlot {
				row = protocol.OrdinaryItem(ticketSlot, item.Template, item.Amount)
			}
		}
		update, e := protocol.InventoryUpdate([][protocol.CurrentItemRecordSize]byte{row})
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"skin_slot_ticket_spent", 0, 14, update})
	}
	refresh, e := w.unlockRefresh(w.role)
	if e != nil {
		return nil, e
	}
	plan = append(plan, refresh...)
	return plan, nil
}
