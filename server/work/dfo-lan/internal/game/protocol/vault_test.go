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
	expectedEmpty := []byte{2, 8, 0, 0, 0, 0}
	if !bytes.Equal(empty, expectedEmpty) {
		t.Fatalf("empty vault mismatch: got %x want %x", empty, expectedEmpty)
	}

	item1 := OrdinaryItem(0, 3037, 100)
	item2 := OrdinaryItem(1, 100261068, 1)
	restored, err := PersonalVaultRestore(8, [][CurrentItemRecordSize]byte{item1, item2})
	if err != nil {
		t.Fatalf("unexpected error for populated vault: %v", err)
	}
	if len(restored) != 1+2+2+2*CurrentItemRecordSize+1 {
		t.Fatalf("unexpected restored length: got %d want %d", len(restored), 1+2+2+2*CurrentItemRecordSize+1)
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
	if restored[len(restored)-1] != 0 {
		t.Fatal("personal vault missing the client's trailing zero byte")
	}

	update, err := InventorySpaceUpdate(2, [][CurrentItemRecordSize]byte{item1})
	if err != nil {
		t.Fatalf("unexpected space 2 update error: %v", err)
	}
	if update[0] != 2 {
		t.Fatalf("expected space 2, got %d", update[0])
	}
}

func TestPersonalVaultSevenRowsDoNotDependOnCipherPadding(t *testing.T) {
	rows := make([][CurrentItemRecordSize]byte, 7)
	for i := range rows {
		rows[i] = OrdinaryItem(uint16(i), uint32(100000+i), 1)
	}
	body, err := PersonalVault(104, rows)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 5+7*CurrentItemRecordSize+1 || body[len(body)-1] != 0 {
		t.Fatalf("seven-row body must include an explicit byte after the block-aligned rows: len=%d", len(body))
	}
	if (len(body)-1)%8 != 0 {
		t.Fatal("the captured seven-row failure was not at an eight-byte boundary")
	}
	rowBody, err := itemRows(rows)
	if err != nil {
		t.Fatal(err)
	}
	legacy := append(add16([]byte{2}, 104), rowBody...)
	if !bytes.Equal(body[:len(body)-1], legacy) {
		t.Fatal("the explicit tail changed the verified capacity, count or item rows")
	}
	second, err := PersonalVaultSpace(45, 104, rows)
	if err != nil || len(second) != 5+7*CurrentItemRecordSize {
		t.Fatalf("unverified list-45 layout changed: len=%d err=%v", len(second), err)
	}
}
