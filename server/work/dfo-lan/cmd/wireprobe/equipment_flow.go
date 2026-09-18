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

type equipmentSession struct {
	nonce       [16]byte
	initialized bool
}

func (s *equipmentSession) handle(service *inventory.WearService, w *worldSession, p, raw []byte) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("equipment move requires owned character")
	}
	r, e := protocol.DecodeItemMove(p)
	if e != nil {
		return nil, e
	}
	// CMD19 carries every bag move, not only equipment. A move involving
	// the personal vault (list 2) belongs to the vault path. A stack going onto
	// the quick-use belt belongs to the stackable path; anything neither
	// recognises falls through to the equipment move unchanged.
	if plan, handled, e := w.moveVault(service.BagRules, r); handled {
		return plan, e
	}
	if !s.initialized {
		if _, e = rand.Read(s.nonce[:]); e != nil {
			return nil, e
		}
		s.initialized = true
	}
	hash := sha256.Sum256(raw)
	// A stack going onto the quick-use belt belongs to the stackable path; anything it does not
	// recognise falls through to the equipment move unchanged.
	if plan, handled, e := w.moveStack(service.BagRules, r, fmt.Sprintf("bagmove:%x:%x", s.nonce, hash)); handled {
		return plan, e
	}
	key := fmt.Sprintf("equipment:%x:%x", s.nonce, hash)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, applied, e := service.Move(ctx, w.role, key, r)
	if e != nil {
		return nil, e
	}
	if !applied {
		w.role = saved
		return nil, nil
	}
	b, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	plan := []outboundPacket{{"equipment_move_committed", 1, 19, protocol.ItemMoveSuccess(r, 1)}}
	for _, space := range []byte{0, 1, 3, 7} {
		var rows []inventory.BagEquipment
		for _, loc := range []struct {
			space byte
			slot  uint16
		}{{r.SourceList, r.SourceSlot}, {r.DestinationList, r.DestinationSlot}} {
			if loc.space != space {
				continue
			}
			row := inventory.BagEquipment{Slot: loc.slot}
			items := b.Equipment
			if space == 3 {
				items = b.Worn
			}
			if space == 1 || space == 7 {
				items = b.Special[space]
			}
			for _, item := range items {
				if item.Slot == loc.slot {
					row = item
					break
				}
			}
			rows = append(rows, row)
		}
		if len(rows) > 0 {
			body, e := inventory.EquipmentPayload(space, rows, false)
			if e != nil {
				return nil, e
			}
			plan = append(plan, outboundPacket{"equipment_slots_updated", 0, 14, body})
		}
	}
	w.role = saved
	return plan, nil
}
