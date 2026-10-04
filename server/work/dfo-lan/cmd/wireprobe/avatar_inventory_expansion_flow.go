package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/cashshop"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"time"
)

func avatarExpansionUsePackets(state json.RawMessage, slot uint16, applied bool) ([]outboundPacket, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, err
	}
	notice, err := protocol.AvatarInventoryExpansionNotice(bag.AvatarExpansion)
	if err != nil {
		return nil, err
	}
	avatars, err := inventory.SpecialEquipmentRestorePayload(state, 1)
	if err != nil {
		return nil, err
	}
	items, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if err != nil {
		return nil, err
	}
	packets := []outboundPacket{
		{"avatar_inventory_expanded", 0, 66, notice},
		{"avatar_inventory_capacity_restored", 0, 13, avatars},
	}
	if applied {
		packets = append(packets, outboundPacket{"avatar_inventory_ticket_used", 1, 507, protocol.AvatarInventoryExpansionSuccess(slot)})
	}
	return append(packets, outboundPacket{"avatar_inventory_ticket_bag_restored", 0, 13, items}), nil
}

func (w *worldSession) useAvatarInventoryExpansion(ctx context.Context, store lotteryItemStore, pilot *cashshop.Pilot, request, raw []byte, prefix string, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID <= 0 || w.role.AccountID <= 0 || store == nil || pilot == nil || w.loot == nil || w.activeDungeon != nil || w.state.Position.Town == 0 || len(raw) < 13 || prefix == "" {
		return nil, fmt.Errorf("avatar inventory expansion requires character in town")
	}
	if pilot.Config.Source.Checksum != w.loot.Catalog.Source.Checksum || w.role.ConfigVersion != pilot.Config.Source.SaveIdentity() {
		return nil, fmt.Errorf("avatar inventory expansion source mismatch")
	}
	slot, err := protocol.DecodeAvatarInventoryExpansion(request)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("avatar-expansion:%s:%x", prefix, sha256.Sum256(raw))
	saved, applied, err := store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "avatar-expansion-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		bag, err := inventory.ReadBag(current.State)
		if err != nil {
			return nil, nil, err
		}
		var template uint32
		for _, item := range bag.Items {
			if item.Slot == slot && item.Amount > 0 {
				if protocol.StoredItemExpired(item.ExpireTime, time.Now().Unix()) {
					return nil, nil, fmt.Errorf("avatar inventory ticket expired")
				}
				template = item.Template
				break
			}
		}
		steps, found, err := pilot.AvatarInventoryExpansion(template)
		if err != nil {
			return nil, nil, err
		}
		if !found || template == 0 {
			return nil, nil, fmt.Errorf("slot has no source avatar inventory ticket")
		}
		if int(bag.AvatarExpansion)+int(steps) > int(protocol.MaxAvatarInventoryExpansion) {
			return nil, nil, fmt.Errorf("avatar inventory fully expanded")
		}
		next, _, err := bag.Consume(w.loot.Catalog, slot, template)
		if err != nil {
			return nil, nil, err
		}
		next.AvatarExpansion += steps
		state, err := inventory.SaveBag(current.State, next)
		if err != nil {
			return nil, nil, err
		}
		if _, err := avatarExpansionUsePackets(state, slot, true); err != nil {
			return nil, nil, err
		}
		receipt, err := json.Marshal(map[string]any{"template": template, "slot": slot, "tier": next.AvatarExpansion})
		return state, receipt, err
	})
	if err != nil {
		return nil, err
	}
	packets, err := avatarExpansionUsePackets(saved.State, slot, applied)
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	if event != nil {
		event(map[string]any{"kind": "avatar_inventory_expansion_saved", "character_id": saved.ID, "slot": slot, "applied": applied})
	}
	return packets, nil
}
