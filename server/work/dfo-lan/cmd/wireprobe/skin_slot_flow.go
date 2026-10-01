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
// 复用多种动作：54 是疲劳恢复药水，101 是光环幻化栏券（10157209），197 是宠物
// 幻化栏券（10309084）。三种券发的帧形状完全一样，只有动作号不同。
func (w *worldSession) stackableAction(p []byte) ([]outboundPacket, error) {
	slot, action, e := protocol.DecodeStackableAction(p)
	if e != nil {
		return nil, e
	}
	switch action {
	case protocol.ActionRecoverFatigue:
		return w.recoverFatiguePotion(p)
	case protocol.ActionOpenAuraSkinSlot, protocol.ActionOpenCreatureSkinSlot:
		return w.expandSkinSlot(p, slot)
	}
	return nil, fmt.Errorf("unsupported stackable action %d", action)
}

// expandSkinSlot 处理幻化栏扩展券的 CMD507 形态（光环动作 101、宠物动作 197）。请求
// 里带了券的真实槽位，所以券源可以直接核对，再由券的模板反查出要开的那一位（光环券
// bit3、宠物券 bit5）。动作号是跟着券走的，不能按槽位猜。
func (w *worldSession) expandSkinSlot(p []byte, slot uint16) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.characters == nil || w.store == nil {
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

// openSkinSlot 处理 CMD857（ENUM_CMDPACKET_OPEN_AURA_SKIN_SLOT），也就是
// 「Unlock the Aura Skin slot?」确认框的 OK。它和 CMD507 是两条不同的入口：同一个
// 窗口「从背包直接使用券」走 507，「点确认框 OK」走 857，两条最终落到同一套落地逻辑。
//
// 请求体只有前四个字节有意义：第一个 u16 恒为 0xffff（客户端不报券在哪一格），第二个
// u16 才是窗口类型 —— 11 光环、32 宠物，也就是要开哪一栏。券由模板反查。
func (w *worldSession) openSkinSlot(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.characters == nil || w.store == nil {
		return nil, fmt.Errorf("skin slot open before character selection")
	}
	req, e := protocol.DecodeOpenSkinSlot(p)
	if e != nil {
		return nil, e
	}
	var mask byte
	switch req.Window {
	case protocol.SkinSlotWindowAura:
		mask = inventory.ExpandAuraSkin
	case protocol.SkinSlotWindowCreature:
		mask = inventory.ExpandCreatureSkin
	default:
		return nil, fmt.Errorf("unsupported skin slot window %d", req.Window)
	}
	// 已经开过的栏再被请求一次（客户端理论上不会弹这个框，但重连/重开窗口后会）：
	// 不再扣券，但**仍然要回成功包** —— 客户端在等这一包，不回它就永远停在等待态，
	// 观感与「这条命令没实现」一模一样。这条与「重放」不同：重放是同一个事务 key
	// 再次命中，由 unlockSkinSlot 返回的 applied 处理，不在这里。
	if before, e := inventory.ReadBag(w.role.State); e == nil && before.ExpandEquipFlags&mask != 0 {
		refresh, e := w.unlockRefresh(w.role)
		if e != nil {
			return nil, e
		}
		return append(refresh, outboundPacket{"open_skin_slot_answered", 1, 857, protocol.OpenSkinSlotReply(req.Window)}), nil
	}
	plan, e := w.unlockSkinSlot(0, mask, fmt.Sprintf("skin-slot-expand:%d:%x", w.role.ID, sha256.Sum256(p)))
	if e != nil {
		return nil, e
	}
	// 857 的回包必须**恰好 3 字节** {1, lo, hi}：收包分发器先把 body[0] 当成功标志并
	// 推进游标，handler 0x145286a50 再从偏移 1 读回窗口类型。多写字节会把 handler 之后
	// 的读取整体错位。
	plan = append(plan, outboundPacket{"open_skin_slot_answered", 1, 857, protocol.OpenSkinSlotReply(req.Window)})
	return plan, nil
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
