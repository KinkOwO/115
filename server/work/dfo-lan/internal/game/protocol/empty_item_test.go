package protocol

import (
	"encoding/binary"
	"testing"
)

func TestEmptyOrdinaryItem(t *testing.T) {
	row := EmptyOrdinaryItem(5)
	slot := binary.LittleEndian.Uint16(row[0:2])
	template := binary.LittleEndian.Uint32(row[2:6])
	if slot != 5 {
		t.Fatalf("expected slot 5, got %d", slot)
	}
	if template != 0xFFFFFFFF {
		t.Fatalf("expected template 0xFFFFFFFF (-1), got 0x%X", template)
	}
}
