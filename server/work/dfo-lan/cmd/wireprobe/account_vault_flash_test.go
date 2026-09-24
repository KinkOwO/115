package main

import (
	"dfolan/internal/inventory"
	"encoding/binary"
	"testing"
)

func TestChangedAccountVaultRowsOnlyTouchedSlots(t *testing.T) {
	old := inventory.Vault{Slots: 8, Items: []inventory.VaultItem{{Slot: 1, Template: 10, Amount: 5}, {Slot: 3, Template: 20, Amount: 8}}}
	next := inventory.Vault{Slots: 8, Items: []inventory.VaultItem{{Slot: 1, Template: 10, Amount: 6}}}
	rows := changedAccountVaultRows(old, next)
	if len(rows) != 2 {
		t.Fatalf("rows=%d", len(rows))
	}
	if binary.LittleEndian.Uint16(rows[0][:2]) != 1 || binary.LittleEndian.Uint16(rows[1][:2]) != 3 {
		t.Fatalf("unexpected slots %x %x", rows[0][:4], rows[1][:4])
	}
	if binary.LittleEndian.Uint32(rows[1][2:6]) != 0xffffffff {
		t.Fatalf("removed slot must be a delete row: %x", rows[1][2:6])
	}
}
