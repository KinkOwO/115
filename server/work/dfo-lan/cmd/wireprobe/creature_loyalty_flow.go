package main

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"time"
)

func (w *worldSession) refreshCreatureLoyalty(ctx context.Context, now time.Time, inDungeon bool) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil || w.characters == nil {
		return nil, nil
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, err
	}
	if !inventory.HasEquippedCreature(w.role.State) && len(bag.Special[7]) == 0 {
		return nil, nil
	}
	var changed, fed bool
	key := fmt.Sprintf("creature-loyalty:%d", now.UnixNano())
	saved, _, err := w.characters.Store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion,
		key, "creature-loyalty-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			state, didChange, didFeed, e := inventory.AdvanceCreatureLoyalty(current.State, now.Unix(), inDungeon, w.loot.Catalog)
			changed, fed = didChange, didFeed
			return state, json.RawMessage(`{}`), e
		})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	if !changed && !fed {
		return nil, nil
	}
	creatures, err := inventory.CreatureListPayload(saved.State)
	if err != nil {
		return nil, err
	}
	packets := []outboundPacket{}
	if fed {
		bag, err := inventory.ReadBag(saved.State)
		if err != nil {
			return nil, err
		}
		body, err := inventory.PetContainerBody(bag, false)
		if err != nil {
			return nil, err
		}
		packets = append(packets, outboundPacket{"creature_auto_feed_container_updated", 0, 14, body})
	}
	packets = append(packets, outboundPacket{"creature_loyalty_updated", 0, 105, creatures})
	return packets, nil
}
