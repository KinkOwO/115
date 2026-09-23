package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// Account material storage ("soul storage") orchestration. The 115 client
// pins seventeen fixed stackable templates (colored cube fragments, souls,
// old souls) to account-shared list 35 at slots 363..379
// (docs/protocol/next43-account-material-storage.md). The server owns the
// counts per account; any of these templates entering a character bag is
// swept into the account storage inside one transaction.

// sweepAccountMaterials moves every account-shared material stack out of the
// role's ordinary bag into the account storage. It is a pure state-derived
// migration: once swept, a retry finds nothing left to move.
func sweepAccountMaterials(ctx context.Context, store *storage.Store, role storage.Character) (storage.Character, inventory.AccountMaterials, error) {
	materials := inventory.NewAccountMaterials()
	if store == nil || role.ID == 0 {
		return role, materials, fmt.Errorf("account material storage unavailable")
	}
	saved, counts, e := store.CommitAccountMaterialSweep(ctx, role.AccountID, role.ID, role.ConfigVersion,
		func(current storage.Character, raw json.RawMessage) (json.RawMessage, json.RawMessage, error) {
			m, e := inventory.ReadAccountMaterials(raw)
			if e != nil {
				return nil, nil, e
			}
			b, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			swept, deltas, e := inventory.SweepAccountMaterials(b)
			if e != nil {
				return nil, nil, e
			}
			if len(deltas) == 0 {
				return current.State, raw, nil
			}
			if m, e = m.ApplyDeltas(deltas); e != nil {
				return nil, nil, e
			}
			state, e := inventory.SaveBag(current.State, swept)
			if e != nil {
				return nil, nil, e
			}
			updated, e := m.Save()
			if e != nil {
				return nil, nil, e
			}
			return state, updated, nil
		})
	if e != nil {
		return role, materials, e
	}
	materials, e = inventory.ReadAccountMaterials(counts)
	if e != nil {
		return role, materials, e
	}
	saved.WireID = role.WireID
	return saved, materials, nil
}

// accountMaterialSnapshot builds the NOTI13 list35 storage snapshot the
// client expects ahead of the ordinary list0 snapshot. The list0 reader
// harvests slots 363..379 into the account storage pipeline
// (sub_145ADC2A0), so the storage snapshot must arrive first and the bag
// snapshot must follow it.
func accountMaterialSnapshot(m inventory.AccountMaterials) ([]byte, error) {
	return protocol.InventoryRestoreSpace(inventory.AccountMaterialSpace, m.Rows())
}

// accountMaterialRefreshPackets returns the authoritative two-packet refresh
// after the storage changed mid-session: list35 storage rows, then the full
// list0 bag snapshot that triggers the client-side harvest.
func accountMaterialRefreshPackets(m inventory.AccountMaterials, role storage.Character) ([]outboundPacket, error) {
	storageBody, e := accountMaterialSnapshot(m)
	if e != nil {
		return nil, e
	}
	b, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, e
	}
	bagBody, e := protocol.InventoryRestore(b.Rows(), b.Expansion)
	if e != nil {
		return nil, e
	}
	return []outboundPacket{
		{"account_materials_restored", 0, 13, storageBody},
		{"inventory_restored", 0, 13, bagBody},
	}, nil
}

// storageDestinationSlot remaps a bag slot to the fixed account storage slot
// when the awarded template belongs to the seventeen shared materials, so
// ACK/scene packets point at where the stack actually lives.
func storageDestinationSlot(template uint32, slot uint16) uint16 {
	if fixed, ok := inventory.AccountMaterialSlot(template); ok {
		return fixed
	}
	return slot
}
