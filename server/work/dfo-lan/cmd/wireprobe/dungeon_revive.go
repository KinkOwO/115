package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

type dungeonReviveStore interface {
	CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error)
}

func (w *worldSession) lifeTokenReviveAllowed(p []byte) error {
	if w == nil || w.role.ID == 0 || w.activeDungeon == nil || !w.activeDungeon.Loaded || w.resultSent {
		return fmt.Errorf("life token revive requires owned loaded dungeon")
	}
	if len(p) != 8 || w.role.WireID == 0 || w.role.WireID == 65535 || binary.LittleEndian.Uint16(p) != w.role.WireID {
		return fmt.Errorf("revive actor mismatch")
	}
	for _, v := range p[2:] {
		if v != 0 {
			return fmt.Errorf("unsupported coin request options")
		}
	}
	if w.pilotDeath == nil || w.pilotDeath.Run != w.activeDungeon.RunID || !w.pilotDeath.Dead {
		return fmt.Errorf("revive requires confirmed player death")
	}
	if w.dungeons == nil {
		return fmt.Errorf("missing map revive rules")
	}
	script, ok := w.dungeons.Maps[w.activeDungeon.Room.Map]
	if !ok {
		return fmt.Errorf("missing current map source")
	}
	for _, v := range script.Cells {
		if v.Type == 3 && v.Text == "[cannot use coin map]" {
			return fmt.Errorf("source map forbids coin revival")
		}
	}
	return nil
}

// lifeTokenRevive consumes one ordinary bag life token and restores the actor.
// The event key is scoped to the dungeon death sequence so a repeated CMD41
// cannot spend a second token, even when the character enters another run.
func (w *worldSession) lifeTokenRevive(ctx context.Context, store dungeonReviveStore, p, frame []byte) ([]outboundPacket, error) {
	hash := sha256.Sum256(frame)
	if w != nil && w.activeDungeon != nil && w.pilotDeath != nil && w.pilotDeath.Run == w.activeDungeon.RunID && w.pilotDeath.Revives[hash] {
		return nil, nil
	}
	if e := w.lifeTokenReviveAllowed(p); e != nil {
		return nil, e
	}
	if store == nil || w.loot == nil {
		return nil, fmt.Errorf("life token storage unavailable")
	}

	key := fmt.Sprintf("dungeon-life-token-revive:%s:%d", w.pilotDeath.Run, w.pilotDeath.Sequence)
	saved, _, e := store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "dungeon-life-token-revive-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		bag, err := inventory.ReadBag(current.State)
		if err != nil {
			return nil, nil, err
		}
		bag, remaining, err := bag.Consume(w.loot.Catalog, 1, 1)
		if err != nil {
			return nil, nil, err
		}
		updated, err := inventory.SaveBag(current.State, bag)
		if err != nil {
			return nil, nil, err
		}
		receipt, err := json.Marshal(map[string]uint32{"before": remaining + 1, "after": remaining})
		return updated, receipt, err
	})
	if e != nil {
		return nil, e
	}

	bag, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	update, e := protocol.InventoryUpdate(bag.Rows())
	if e != nil {
		return nil, e
	}
	death, e := protocol.PlayerDeathState(w.role.WireID)
	if e != nil {
		return nil, e
	}
	// Native1452aadea restores full HP/MP for state1; state2 restores one third.
	death[2] = 1
	ack := binary.LittleEndian.AppendUint16([]byte{1}, w.role.WireID)
	saved.WireID = w.role.WireID
	w.role = saved
	if w.pilotDeath.Revives == nil {
		w.pilotDeath.Revives = map[[32]byte]bool{}
	}
	w.pilotDeath.Dead = false
	w.pilotDeath.Revives[hash] = true
	return []outboundPacket{
		{"life_token_revive_ack", 1, 41, ack},
		{"life_token_revived", 0, 32, death},
		{"life_token_inventory_updated", 0, 14, update},
	}, nil
}
