package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

func (w *worldSession) useEmblems(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, fmt.Errorf("avatar emblem service unavailable")
	}
	req, err := protocol.DecodeUseEmblem(p)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, err := (&workflow.LootService{Store: w.store, Loot: w.loot}).UseEmblems(ctx, w.role, req)
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
