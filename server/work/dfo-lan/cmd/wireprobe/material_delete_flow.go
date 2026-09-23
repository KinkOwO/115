package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"time"
)

func (w *worldSession) deleteSkillMaterial(p, raw []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil || w.vault == nil || w.activeDungeon == nil || !w.activeDungeon.Loaded {
		return nil, fmt.Errorf("material use outside owned loaded dungeon")
	}
	rows, e := protocol.DecodeMaterialDelete(p)
	if e != nil {
		return nil, e
	}
	if w.vault.Catalog.Items[3037].StackableType != "[material]" {
		return nil, fmt.Errorf("missing clear cube definition")
	}
	key := fmt.Sprintf("skill-material:%s:%x", w.activeDungeon.RunID, sha256.Sum256(raw))
	model := fmt.Sprintf("skill-material-v1:%x", sha256.Sum256(p))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, e := w.loot.Store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, model, func(role storage.Character) (json.RawMessage, json.RawMessage, error) {
		bag, e := inventory.ReadBag(role.State)
		if e != nil {
			return nil, nil, e
		}
		for _, r := range rows {
			found := false
			for i := range bag.Items {
				v := &bag.Items[i]
				if v.Slot != r.Slot {
					continue
				}
				if v.Template != r.Template || v.Amount < r.Count {
					return nil, nil, fmt.Errorf("insufficient owned skill material")
				}
				v.Amount -= r.Count
				found = true
				break
			}
			if !found {
				return nil, nil, fmt.Errorf("material slot missing")
			}
		}
		kept := bag.Items[:0]
		for _, v := range bag.Items {
			if v.Amount > 0 {
				kept = append(kept, v)
			}
		}
		bag.Items = kept
		state, e := inventory.SaveBag(role.State, bag)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(rows)
		return state, receipt, e
	})
	if e != nil {
		return nil, e
	}
	saved.WireID = w.role.WireID
	w.role = saved
	bag, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	var delta [][protocol.CurrentItemRecordSize]byte
	for _, r := range rows {
		item := protocol.EmptyOrdinaryItem(r.Slot)
		for _, v := range bag.Items {
			if v.Slot == r.Slot {
				item = protocol.OrdinaryItem(v.Slot, v.Template, v.Amount)
			}
		}
		delta = append(delta, item)
	}
	update, e := protocol.InventoryUpdate(delta)
	if e != nil {
		return nil, e
	}
	// Re-acknowledge uncertain commits to release the client's reservation.
	// The following absolute slot values correct any repeated client decrement.
	return []outboundPacket{{"skill_material_ack", 1, 18, protocol.MaterialDeleteReply(rows, true)}, {"skill_material_inventory", 0, 14, update}}, nil
}
