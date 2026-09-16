package protocol

import (
	"bytes"
	"testing"
)

func TestPersonalVaultRestoreWithItems(t *testing.T) {
	empty, err := PersonalVaultRestore(8, nil)
	if err != nil {
		t.Fatalf("unexpected error for empty vault: %v", err)
	}
	expectedEmpty := []byte{2, 8, 0, 0, 0}
	if !bytes.Equal(empty, expectedEmpty) {
		t.Fatalf("empty vault mismatch: got %x want %x", empty, expectedEmpty)
	}

	item1 := OrdinaryItem(0, 3037, 100)
	item2 := OrdinaryItem(1, 100261068, 1)
	restored, err := PersonalVaultRestore(8, [][CurrentItemRecordSize]byte{item1, item2})
	if err != nil {
		t.Fatalf("unexpected error for populated vault: %v", err)
	}
	if len(restored) != 1+2+2+2*CurrentItemRecordSize {
		t.Fatalf("unexpected restored length: got %d want %d", len(restored), 1+2+2+2*CurrentItemRecordSize)
	}
	if restored[0] != 2 {
		t.Fatalf("expected list 2, got %d", restored[0])
	}
	if restored[1] != 8 || restored[2] != 0 {
		t.Fatalf("expected capacity 8, got %d", restored[1])
	}
	if restored[3] != 2 || restored[4] != 0 {
		t.Fatalf("expected item count 2, got %d", restored[3])
	}

	update, err := InventorySpaceUpdate(2, [][CurrentItemRecordSize]byte{item1})
	if err != nil {
		t.Fatalf("unexpected space 2 update error: %v", err)
	}
	if update[0] != 2 {
		t.Fatalf("expected space 2, got %d", update[0])
	}
}
