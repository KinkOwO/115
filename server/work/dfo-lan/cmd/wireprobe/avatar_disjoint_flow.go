package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

func (w *worldSession) disjointAvatar(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, fmt.Errorf("avatar disjoint service unavailable")
	}
	r, e := protocol.DecodeDisjointAvatar(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, e := w.loot.DisjointAvatar(ctx, w.role, r)
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
