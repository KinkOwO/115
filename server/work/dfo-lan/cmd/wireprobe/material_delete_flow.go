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
	reply := protocol.MaterialDeleteReply
	contract := false
	if e != nil {
		rows, e = protocol.DecodeCubeContractDelete(p)
		if e != nil {
			return nil, e
		}
		contract = true
		reply = protocol.CubeContractDeleteReply
	}
	if contract {
		if w.vault.Store == nil {
			return nil, fmt.Errorf("晶体契约存储不可用")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		active, err := w.vault.Store.HasActivePremium(ctx, w.role.AccountID, storage.PremiumCube, time.Now())
		cancel()
		if err != nil {
			return nil, err
		}
		if !active {
			return nil, fmt.Errorf("晶体契约未生效或已到期")
		}
	}
	if w.vault.Catalog.Items[3037].StackableType != "[material]" {
		return nil, fmt.Errorf("missing clear cube definition")
	}
	fromStorage, e := skillMaterialStorageRows(rows)
	if e != nil {
		return nil, e
	}
	if fromStorage {
		return w.spendSkillMaterialFromStorage(p, raw, rows, reply)
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
	return []outboundPacket{{"skill_material_ack", 1, 18, reply(rows, true)}, {"skill_material_inventory", 0, 14, update}}, nil
}

// skillMaterialStorageRows reports whether a skill cost is paid out of the
// account-shared material store rather than the character's ordinary bag.
//
// The client numbers both stores in one list: the bag's material cells are
// 121..176 and the shared store's fixed cells are 363..379. Which template a
// shared cell holds is fixed (inventory.accountMaterialTemplateBySlot), so a
// request that names one of those cells with any other template is refused
// rather than silently charged to the wrong stack. A packet that mixes the two
// stores is refused too - no live client sends one.
func skillMaterialStorageRows(rows []protocol.MaterialDelete) (bool, error) {
	storage, bag := 0, 0
	for _, r := range rows {
		template, ok := inventory.StorageRowTemplate(r.Slot)
		if !ok {
			bag++
			continue
		}
		if template != r.Template {
			return false, fmt.Errorf("account material slot/template mismatch")
		}
		storage++
	}
	if storage > 0 && bag > 0 {
		return false, fmt.Errorf("mixed skill material stores")
	}
	return storage > 0, nil
}

// spendSkillMaterialFromStorage deducts a skill cost from the account-shared
// store. It commits through CommitAccountMaterialEvent, not the sweep path:
// a spend must be applied exactly once, and only that commit carries the
// receipt that makes a client retry harmless. The reply mirrors the bag path -
// rows echoed so the client can release its pending reservation, then the
// authoritative storage and bag panels.
func (w *worldSession) spendSkillMaterialFromStorage(p, raw []byte, rows []protocol.MaterialDelete, reply func([]protocol.MaterialDelete, bool) []byte) ([]outboundPacket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := fmt.Sprintf("skill-material-account:%s:%x", w.activeDungeon.RunID, sha256.Sum256(raw))
	model := fmt.Sprintf("skill-material-account-v1:%x", sha256.Sum256(p))
	saved, counts, _, e := w.loot.Store.CommitAccountMaterialEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, model,
		func(role storage.Character, rawCounts json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			m, e := inventory.ReadAccountMaterials(rawCounts)
			if e != nil {
				return nil, nil, e
			}
			for _, r := range rows {
				next, _, e := m.Spend(r.Template, r.Count)
				if e != nil {
					return nil, nil, e
				}
				m = next
			}
			updated, e := m.Save()
			if e != nil {
				return nil, nil, e
			}
			return role.State, updated, nil
		})
	if e != nil {
		return nil, e
	}
	saved.WireID = w.role.WireID
	w.role = saved
	materials, e := inventory.ReadAccountMaterials(counts)
	if e != nil {
		return nil, e
	}
	refresh, e := accountMaterialRefreshPackets(materials, saved)
	if e != nil {
		return nil, e
	}
	return append([]outboundPacket{{"skill_material_ack", 1, 18, reply(rows, true)}}, refresh...), nil
}
