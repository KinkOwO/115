package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

func (w *worldSession) disjointAvatar(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.items == nil {
		return nil, fmt.Errorf("avatar disjoint service unavailable")
	}
	r, e := protocol.DecodeDisjointAvatar(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, e := (&workflow.ItemService{Store: w.store, Items: w.items}).DisjointAvatar(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	w.role = saved
	if event != nil {
		event(map[string]any{"kind": "avatar_disjoint_applied", "id": 202, "character_id": saved.ID, "slot": r.Slot, "template": r.Template, "sequence": receipt.Sequence, "applied": applied, "rewards": receipt.Rewards})
	}
	return []outboundPacket{
		{"avatar_disjoint_ack", 1, 202, receipt.Ack},
		{"avatar_disjoint_avatars_restored", 0, 13, receipt.Avatars},
		// InventoryRestore includes the expansion u16 and belongs to NOTI13.
		// NOTI14 would read those bytes as the row count and shift every row.
		{"avatar_disjoint_inventory_restored", 0, 13, receipt.Inventory},
	}, nil
}

func (w *worldSession) addAvatarSocket(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.items == nil {
		return nil, fmt.Errorf("avatar socket service unavailable")
	}
	r, e := protocol.DecodeAddAvatarSocket(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, e := (&workflow.ItemService{Store: w.store, Items: w.items}).AddAvatarSocket(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	w.role = saved
	if event != nil {
		event(map[string]any{"kind": "avatar_socket_applied", "id": 206, "character_id": saved.ID, "avatar_slot": r.AvatarSlot, "template": r.Template, "device_slot": r.DeviceSlot, "device_template": receipt.DeviceTemplate, "sequence": receipt.Sequence, "applied": applied})
	}
	return []outboundPacket{
		// The ACK reader opens the result window using the existing avatar object.
		{"avatar_socket_avatar_updated", 0, 14, receipt.Avatar},
		{"avatar_socket_ack", 1, 206, receipt.Ack},
		// The ACK already decrements the local device stack: resync AFTER it.
		{"avatar_socket_inventory_restored", 0, 13, receipt.Inventory},
	}, nil
}

func (w *worldSession) compoundEmblems(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.items == nil {
		return nil, fmt.Errorf("emblem compound service unavailable")
	}
	r, e := protocol.DecodeCompoundEmblem(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, e := (&workflow.ItemService{Store: w.store, Items: w.items}).CompoundEmblems(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	w.role = saved
	if event != nil {
		event(map[string]any{"kind": "emblem_compound_applied", "id": 256, "character_id": saved.ID, "inputs": r.Inputs, "mode": r.Mode, "sequence": receipt.Sequence, "applied": applied, "rewards": receipt.Rewards})
	}
	return []outboundPacket{
		{"emblem_compound_ack", 1, 256, receipt.Ack},
		{"emblem_compound_inventory_restored", 0, 13, receipt.Inventory},
	}, nil
}

func (w *worldSession) useEmblems(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.items == nil {
		return nil, fmt.Errorf("avatar emblem service unavailable")
	}
	req, err := protocol.DecodeUseEmblem(p)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, err := (&workflow.ItemService{Store: w.store, Items: w.items}).UseEmblems(ctx, w.role, req)
	if err != nil {
		return nil, err
	}
	w.role = saved
	if event != nil {
		event(map[string]any{"kind": "avatar_emblem_applied", "id": 201, "character_id": saved.ID, "avatar_slot": req.AvatarSlot, "template": req.Template, "inputs": req.Inputs, "sequence": receipt.Sequence, "applied": applied})
	}
	return []outboundPacket{
		{"avatar_emblem_avatar_updated", 0, 14, receipt.Avatar},
		{"avatar_emblem_ack", 1, 201, receipt.Ack},
		// Restores absolute counts, including removal of exhausted stacks.
		{"avatar_emblem_inventory_restored", 0, 13, receipt.Inventory},
	}, nil
}
