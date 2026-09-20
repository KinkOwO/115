package protocol

import (
	"encoding/binary"
	"testing"
)

func TestExitContentBriefingIsFixedWidth(t *testing.T) {
	packet := ExitContentBriefingDefaults()
	if len(packet) != ContentBriefingPacketSize {
		t.Fatalf("packet = %d bytes, want %d", len(packet), ContentBriefingPacketSize)
	}
	if packet[0] != 1 {
		t.Fatalf("result byte = %#x, want 1", packet[0])
	}
	for i, b := range packet[1:] {
		if b != 0 {
			t.Fatalf("default slot byte %d = %#x, want 0", i, b)
		}
	}
}

func TestExitContentBriefingKeepsSlotOrder(t *testing.T) {
	var slots [contentBriefingSlots]uint32
	slots[0], slots[3], slots[13], slots[22], slots[31] = 1, 4, 14, 23, 32
	packet := ExitContentBriefing(slots)
	if len(packet) != ContentBriefingPacketSize {
		t.Fatalf("packet = %d bytes, want %d", len(packet), ContentBriefingPacketSize)
	}
	for i, want := range slots {
		if got := binary.LittleEndian.Uint32(packet[1+i*4:]); got != want {
			t.Fatalf("slot %d = %d, want %d", i, got, want)
		}
	}
}
