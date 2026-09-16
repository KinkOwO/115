package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestDisjointItemDecodeLiveCapture(t *testing.T) {
	// Live capture from roles_persist_..._next37/events.jsonl:
	// "00ffff010b00ae690000000000000000"
	p, err := hex.DecodeString("00ffff010b00ae690000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	r, err := DecodeDisjointItem(p)
	if err != nil {
		t.Fatalf("DecodeDisjointItem failed: %v", err)
	}
	if r.Mode != 0 {
		t.Fatalf("expected mode 0, got %d", r.Mode)
	}
	if r.ToolSlot != 0xFFFF {
		t.Fatalf("expected tool_slot 0xFFFF, got 0x%X", r.ToolSlot)
	}
	if len(r.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(r.Items))
	}
	if r.Items[0].Slot != 11 {
		t.Fatalf("expected slot 11, got %d", r.Items[0].Slot)
	}
	if r.Items[0].Template != 27054 {
		t.Fatalf("expected template 27054, got %d", r.Items[0].Template)
	}
}

func TestDisjointItemSuccessBuild(t *testing.T) {
	result := DisjointItemResult{
		DeletedSlots: []uint16{11},
		List:         0,
		ToolSlot:     0xFFFF,
		Rewards: []DisjointRewardEntry{
			{
				Slot:     121,
				Template: 3037,
				Count:    4,
			},
		},
	}
	payload, err := DisjointItemSuccess(result)
	if err != nil {
		t.Fatalf("DisjointItemSuccess failed: %v", err)
	}
	expected, _ := hex.DecodeString("01010b0000ffff017900dd0b000004000000")
	if !bytes.Equal(payload, expected) {
		t.Fatalf("DisjointItemSuccess payload mismatch:\ngot:  %x\nwant: %x", payload, expected)
	}
}

func TestDisjointItemRefused(t *testing.T) {
	refused := DisjointItemRefused(4)
	expected := []byte{0, 4, 0}
	if !bytes.Equal(refused, expected) {
		t.Fatalf("DisjointItemRefused mismatch: got %x, want %x", refused, expected)
	}
}
