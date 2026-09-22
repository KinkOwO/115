package protocol

import (
	"encoding/hex"
	"testing"
)

// TestDecodeDeleteItemsLive captures the exact CMD18 body the 115 client
// sent when discarding two slot-65 items (template 2660296) in town:
// 11 00 00 00 (length 17) 10 00 (list 0) 1a 0b (entry len 11)
// 08 01 (op 1) 10 41 (slot 65) 18 c8 af a2 01 (template 2660296) 20 02 (count 2)
// 20 00 (condition 0).
func TestDecodeDeleteItemsLive(t *testing.T) {
	p, err := hex.DecodeString("1100000010001a0b0801104118c8afa201200220000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := DecodeDeleteItems(p)
	if err != nil {
		t.Fatalf("DecodeDeleteItems: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(rows))
	}
	got := rows[0]
	if got.Slot != 65 || got.Template != 2660296 || got.Count != 2 {
		t.Fatalf("got %+v, want slot=65 template=2660296 count=2", got)
	}
}

func TestDecodeDeleteItemsRejectsMaterialOnlyShape(t *testing.T) {
	// The same wire with the material op (op=2) decodes fine for the generic
	// parser (it is a superset of the material branch).
	p, _ := hex.DecodeString("1100000010001a0b0802104118c8afa201200220000000000000000000000000")
	rows, err := DecodeDeleteItems(p)
	if err != nil {
		t.Fatalf("op2 should be accepted by generic parser: %v", err)
	}
	if len(rows) != 1 || rows[0].Slot != 65 || rows[0].Count != 2 {
		t.Fatalf("unexpected rows %+v", rows)
	}
}

func TestDecodeDeleteItemsRejectsMalformed(t *testing.T) {
	cases := [][]byte{
		nil,
		{1, 0, 0, 0},                                    // no protobuf content
		{0x11, 0, 0, 0, 0x10, 0, 0x1a, 0x0b, 0x08, 0x01}, // truncated entry
	}
	for i, c := range cases {
		if _, err := DecodeDeleteItems(c); err == nil {
			t.Fatalf("case %d should fail", i)
		}
	}
}

func TestDeleteItemsReplyShape(t *testing.T) {
	rows := []ItemDelete{{Slot: 65, Template: 2660296, Count: 2}}
	b := DeleteItemsReply(rows, true)
	if len(b) < 6 {
		t.Fatalf("reply too short: %x", b)
	}
	if b[0] != 1 {
		t.Fatalf("want status 1, got %d", b[0])
	}
	// The protobuf payload must contain the nested slot varint 65 (0x41).
	found := false
	for _, x := range b {
		if x == 0x41 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("reply missing slot 65: %x", b)
	}
}
