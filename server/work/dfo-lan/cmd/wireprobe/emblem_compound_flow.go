package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

func (w *worldSession) compoundEmblems(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, fmt.Errorf("emblem compound service unavailable")
	}
	r, e := protocol.DecodeCompoundEmblem(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, e := w.loot.CompoundEmblems(ctx, w.role, r)
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
