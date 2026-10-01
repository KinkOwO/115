package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

func (w *worldSession) addAvatarSocket(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, fmt.Errorf("avatar socket service unavailable")
	}
	r, e := protocol.DecodeAddAvatarSocket(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, applied, e := w.loot.AddAvatarSocket(ctx, w.role, r)
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
