package protocol

import (
	"encoding/binary"
	"testing"
)

func TestCeraSpecialItemNotification(t *testing.T) {
	endTime := int64(1750000000)
	premiumType := uint8(27)
	p := CeraSpecialItemNotification(premiumType, endTime)
	if len(p) != 11 {
		t.Fatalf("expected length 11, got %d", len(p))
	}
	mode := binary.LittleEndian.Uint16(p[0:2])
	if mode != 2 {
		t.Fatalf("expected mode 2, got %d", mode)
	}
	if p[2] != premiumType {
		t.Fatalf("expected premiumType %d, got %d", premiumType, p[2])
	}
	gotEnd := int64(binary.LittleEndian.Uint64(p[3:11]))
	if gotEnd != endTime {
		t.Fatalf("expected endTime %d, got %d", endTime, gotEnd)
	}
}
