package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

func decodeCreatureHatchRequest(p []byte) (uint16, error) {
	if len(p) >= 3 && p[0] == 7 {
		return binary.LittleEndian.Uint16(p[1:3]), nil
	}
	if len(p) >= 2 {
		return binary.LittleEndian.Uint16(p[0:2]), nil
	}
	return 0, fmt.Errorf("invalid hatch creature request length %d", len(p))
}

type characterEventStore interface {
	CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(database.Character) (json.RawMessage, json.RawMessage, error)) (database.Character, bool, error)
}

func (w *worldSession) hatchCreature(ctx context.Context, store characterEventStore, cmdID uint16, p, raw []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("creature hatch requires active character")
	}
	slot, err := decodeCreatureHatchRequest(p)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("creature-hatch:%d:%x", w.role.ID, sha256.Sum256(raw))
	saved, _, err := store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "creature-hatch-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		b, err := inventory.ReadBag(current.State)
		if err != nil {
			return nil, nil, err
		}
		foundIdx := -1
		if b.Special != nil {
			for i, it := range b.Special[7] {
				if it.Slot == slot {
					foundIdx = i
					break
				}
			}
		}
		if foundIdx == -1 {
			return nil, nil, fmt.Errorf("no creature at slot %d", slot)
		}
		item := b.Special[7][foundIdx]
		hatchedTemplate, ok := inventory.EggHatchOutputs[item.Template]
		if !ok {
			return nil, nil, fmt.Errorf("template %d is not a hatchable egg", item.Template)
		}
		b.Special[7][foundIdx].Template = hatchedTemplate
		rawBag, err := inventory.SaveBag(current.State, b)
		if err != nil {
			return nil, nil, err
		}
		receipt, _ := json.Marshal(map[string]any{"slot": slot, "egg": item.Template, "hatched": hatchedTemplate})
		return rawBag, receipt, nil
	})
	if err != nil {
		return nil, err
	}
	w.role = saved
	b, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	var hatchedItem inventory.BagEquipment
	for _, it := range b.Special[7] {
		if it.Slot == slot {
			hatchedItem = it
			break
		}
	}
	payload, err := inventory.EquipmentPayload(7, []inventory.BagEquipment{hatchedItem}, false)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{
		{"creature_hatch_ack", 1, cmdID, []byte{1}},
		{"creature_hatch_update", 0, 14, payload},
		{"creature_list_refreshed", 0, 105, []byte{0}},
	}, nil
}
