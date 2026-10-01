package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

func (w *worldSession) recastAvatar(service *workflow.WearService, p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || service == nil {
		return nil, fmt.Errorf("avatar recast service unavailable")
	}
	r, e := protocol.DecodeRecastAvatar(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, e := service.RecastAvatar(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	w.role = saved
	if event != nil {
		event(map[string]any{"kind": "avatar_recast_emblems_applied", "id": 795, "character_id": saved.ID, "mode": r.Mode, "inputs": r.Items, "option": r.Option, "rewards": receipt.Rewards, "matches": receipt.Matches, "sequence": receipt.Sequence, "applied": applied, "reward_popup_id": 1807})
	}
	return []outboundPacket{
		{"avatar_recast_ack", 1, 795, receipt.Ack},
		{"avatar_recast_avatars_restored", 0, 13, receipt.Avatars},
		{"avatar_recast_emblems_updated", 0, 14, receipt.Inventory},
		{"avatar_recast_reward_popup", 1, 1807, receipt.Popup},
	}, nil
}
