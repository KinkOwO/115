package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"encoding/json"
	"fmt"
	"time"
)

func avatarOption(service *workflow.WearService, w *worldSession, p, keys []byte) ([]preparedPacket, error) {
	if w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("avatar selection requires character")
	}
	r, e := protocol.DecodeAvatarOption(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hash := sha256.Sum256(p)
	var packets []preparedPacket
	saved, _, e := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, fmt.Sprintf("avatar-option:%x", hash), "avatar-option-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		var consumables catalog.LootCatalog
		if r.Paid {
			if w.items == nil {
				return nil, nil, fmt.Errorf("avatar option item catalog unavailable")
			}
			consumables = w.items.Catalog
		}
		raw, e := service.SelectAvatarOption(workflow.InventoryRole(current), r, consumables)
		if e != nil {
			return nil, nil, e
		}
		packets, e = prepareAvatarOptionPackets(keys, r, raw)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(r)
		return raw, receipt, e
	})
	if e != nil {
		return nil, e
	}
	// A replayed idempotency key skips the callback; reconstruct the same
	// response from the already committed character state.
	if packets == nil {
		packets, e = prepareAvatarOptionPackets(keys, r, saved.State)
		if e != nil {
			return nil, e
		}
	}
	saved.WireID = w.role.WireID
	w.role = saved
	return packets, nil
}

func prepareAvatarOptionPackets(keys []byte, r protocol.AvatarOptionRequest, state json.RawMessage) ([]preparedPacket, error) {
	plan := []outboundPacket{{"avatar_option_selected", 1, 451, protocol.AvatarOptionSuccess(r)}}
	if r.Paid {
		bag, err := inventory.ReadBag(state)
		if err != nil {
			return nil, err
		}
		restore, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"avatar_option_inventory_restored", 0, 13, restore})
	}
	return preparePackets(keys, plan)
}
