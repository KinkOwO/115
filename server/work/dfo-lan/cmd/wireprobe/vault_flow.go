package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// moveVault answers CMD 19 (ENUM_CMDPACKET_MOVE_ITEMSPACE) when either source
// or destination is the personal vault (List == 2).
func (w *worldSession) moveVault(rules inventory.BagRules, r protocol.ItemMoveRequest) ([]outboundPacket, bool, error) {
	if r.SourceList != 2 && r.DestinationList != 2 {
		return nil, false, nil
	}
	if w == nil || w.role.ID == 0 || w.vault == nil || w.vault.Store == nil {
		return nil, true, fmt.Errorf("vault move before character or vault service initialization")
	}

	var movedCount uint32
	var newBag inventory.Bag
	var oldVault inventory.Vault
	var newVault inventory.Vault

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	savedRole, _, err := w.vault.Store.CommitVaultMove(ctx, w.account, w.role.ID,
		func(curRole storage.Character, curVaultState storage.VaultState) (json.RawMessage, json.RawMessage, error) {
			b, e := inventory.ReadBag(curRole.State)
			if e != nil {
				return nil, nil, e
			}
			v, e := inventory.ReadVault(curVaultState)
			if e != nil {
				return nil, nil, e
			}
			oldVault = v
			var moveErr error
			newBag, newVault, movedCount, moveErr = inventory.MoveVaultItem(b, v, rules, r)
			if moveErr != nil {
				return nil, nil, moveErr
			}
			savedBagRaw, e := inventory.SaveBag(curRole.State, newBag)
			if e != nil {
				return nil, nil, e
			}
			savedVaultRaw, e := inventory.SaveVault(newVault)
			if e != nil {
				return nil, nil, e
			}
			return savedBagRaw, savedVaultRaw, nil
		})
	if err != nil {
		return nil, true, err
	}

	w.role = savedRole

	plan := []outboundPacket{
		{"vault_move_committed", 1, 19, protocol.ItemMoveSuccess(r, movedCount)},
	}

	if r.SourceList == 0 || r.DestinationList == 0 {
		bagUpdate, e := protocol.InventoryUpdate(newBag.Rows())
		if e != nil {
			return nil, true, e
		}
		plan = append(plan, outboundPacket{"vault_bag_updated", 0, 14, bagUpdate})
	}

	if r.SourceList == 2 || r.DestinationList == 2 {
		affectedMap := make(map[uint16]bool)
		if r.SourceList == 2 {
			affectedMap[r.SourceSlot] = true
		}
		if r.DestinationList == 2 {
			affectedMap[r.DestinationSlot] = true
		}
		for _, it := range oldVault.Items {
			newIt := newVault.ItemAt(it.Slot)
			if newIt == nil || newIt.Template != it.Template || newIt.Amount != it.Amount || newIt.Durability != it.Durability {
				affectedMap[it.Slot] = true
			}
		}
		for _, it := range newVault.Items {
			oldIt := oldVault.ItemAt(it.Slot)
			if oldIt == nil || oldIt.Template != it.Template || oldIt.Amount != it.Amount || oldIt.Durability != it.Durability {
				affectedMap[it.Slot] = true
			}
		}

		var affectedSlots []uint16
		for slot := range affectedMap {
			affectedSlots = append(affectedSlots, slot)
		}
		sort.Slice(affectedSlots, func(i, j int) bool { return affectedSlots[i] < affectedSlots[j] })

		var vaultRows [][protocol.CurrentItemRecordSize]byte
		for _, slot := range affectedSlots {
			if it := newVault.ItemAt(slot); it != nil {
				vaultRows = append(vaultRows, it.Row())
			} else {
				vaultRows = append(vaultRows, protocol.EmptyOrdinaryItem(slot))
			}
		}

		if len(vaultRows) > 0 {
			vaultUpdate, e := protocol.InventorySpaceUpdate(2, vaultRows)
			if e != nil {
				return nil, true, e
			}
			plan = append(plan, outboundPacket{"vault_slots_updated", 0, 14, vaultUpdate})
		}
	}

	return plan, true, nil
}
