package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

func (s *equipmentSession) reinforce(service *inventory.WearService, w *worldSession, p, raw []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, fmt.Errorf("强化券需要已选角色且位于城镇")
	}
	r, err := protocol.DecodeReinforcement(p)
	if err != nil {
		return nil, err
	}
	if !s.initialized {
		if _, err = rand.Read(s.nonce[:]); err != nil {
			return nil, err
		}
		s.initialized = true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := fmt.Sprintf("reinforcement-ticket:%x:%x", s.nonce, sha256.Sum256(raw))
	saved, out, err := service.ReinforceWithTicket(ctx, w.role, key, r)
	if err != nil {
		return nil, err
	}
	w.role = saved
	ack, err := protocol.ReinforcementTicketReply(r, out.Remaining, out.Old, out.Level, out.Result)
	if err != nil {
		return nil, err
	}
	// 原生应答先处理旧物品对象，再以权威行刷新；不能先删掉最后一张券。
	plan := []outboundPacket{{"reinforcement_ticket_result", 1, 80, ack}}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	// 即使重放旧回执，也发送当前槽位；采用增量更新保留强化动画引用的装备对象。
	ticketRow, exists := bag.RowAt(r.TicketSlot)
	if !exists {
		ticketRow = protocol.EmptyOrdinaryItem(r.TicketSlot)
	}
	rows := [][protocol.CurrentItemRecordSize]byte{ticketRow}
	if r.EquipmentSpace == 0 {
		gearRow, exists := bag.RowAt(r.EquipmentSlot)
		if !exists {
			gearRow = protocol.EmptyOrdinaryItem(r.EquipmentSlot)
		}
		rows = append(rows, gearRow)
	}
	body, err := protocol.InventoryUpdate(rows)
	if err != nil {
		return nil, err
	}
	plan = append(plan, outboundPacket{"reinforcement_ticket_inventory", 0, 14, body})
	if r.EquipmentSpace == 3 {
		body, err = inventory.WornSpaceUpdate(saved.State)
		if err != nil {
			return nil, err
		}
		if len(body) > 0 {
			plan = append(plan, outboundPacket{"reinforcement_equipment_updated", 0, 14, body})
		}
	}
	event(map[string]any{"kind": "reinforcement_ticket_committed", "character_id": saved.ID,
		"ticket": out.Ticket, "remaining": out.Remaining, "equipment": r.EquipmentTemplate,
		"space": r.EquipmentSpace, "slot": r.EquipmentSlot, "before": out.Old, "after": out.Level, "result": out.Result})
	return plan, nil
}
