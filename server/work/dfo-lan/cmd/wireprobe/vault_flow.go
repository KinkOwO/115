package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"fmt"
	"time"
)

func vaultMovePackets(state storage.Character, v storage.VaultState, r protocol.ItemMoveRequest) ([]outboundPacket, error) {
	b, e := inventory.ReadBag(state.State)
	if e != nil {
		return nil, e
	}
	bag, e := protocol.InventoryRestore(b.Rows())
	if e != nil {
		return nil, e
	}
	vault, e := inventory.VaultPayload(v)
	if e != nil {
		return nil, e
	}
	return []outboundPacket{
		{"vault_move_committed", 1, 19, protocol.ItemMoveSuccess(r, r.Count)},
		{"vault_bag_restored", 0, 13, bag},
		{"vault_contents_restored", 0, 13, vault},
	}, nil
}

func (w *worldSession) moveVault(key string, r protocol.ItemMoveRequest) ([]outboundPacket, error) {
	if w == nil || w.vault == nil || w.role.ID == 0 || w.activeDungeon != nil || w.inTutorial || w.selectingDungeon {
		return nil, fmt.Errorf("vault transfer requires a town character")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, v, _, e := w.vault.Move(ctx, w.role, key, r)
	if e != nil {
		return nil, e
	}
	p, e := vaultMovePackets(saved, v, r)
	if e != nil {
		return nil, e
	}
	w.role = saved
	return p, nil
}
