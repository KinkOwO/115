package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestDecodeBuyItemMatchesRecoveredLayout(t *testing.T) {
	// 24 bytes: template=1001, count=5, npcId=1, actorId=2, category=7, reserved=0
	var p [24]byte
	binary.LittleEndian.PutUint32(p[0:], 1001)
	binary.LittleEndian.PutUint32(p[4:], 5)
	binary.LittleEndian.PutUint32(p[8:], 1)
	binary.LittleEndian.PutUint32(p[12:], 2)
	binary.LittleEndian.PutUint32(p[16:], 7)
	binary.LittleEndian.PutUint32(p[20:], 0)

	r, err := DecodeBuyItem(p[:])
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if r.Template != 1001 || r.Count != 5 || r.NpcID != 1 || r.ActorID != 2 || r.Category != 7 || r.Reserved != 0 {
		t.Fatalf("buy layout mismatch: %+v", r)
	}
}

func TestDecodeBuyItemRejectsMalformed(t *testing.T) {
	var p [24]byte
	binary.LittleEndian.PutUint32(p[0:], 1001)
	binary.LittleEndian.PutUint32(p[4:], 5)
	binary.LittleEndian.PutUint32(p[8:], 1)
	binary.LittleEndian.PutUint32(p[12:], 2)
	binary.LittleEndian.PutUint32(p[16:], 7)

	// Zero template
	badTemplate := p
	binary.LittleEndian.PutUint32(badTemplate[0:], 0)
	if _, err := DecodeBuyItem(badTemplate[:]); err == nil {
		t.Fatal("expected error on zero template")
	}

	// Zero count (offset 4)
	badCount := p
	binary.LittleEndian.PutUint32(badCount[4:], 0)
	if _, err := DecodeBuyItem(badCount[:]); err == nil {
		t.Fatal("expected error on zero count")
	}

	// Wrong length
	if _, err := DecodeBuyItem(p[:20]); err == nil {
		t.Fatal("expected error on 20 bytes")
	}
	if _, err := DecodeBuyItem(append(p[:], 0, 0, 0, 0)); err == nil {
		t.Fatal("expected error on 28 bytes")
	}
}

// Captured from the user's current client, 2026-09-26 08:40/08:41 UTC.
func TestDecodeSellItemNativeRequests(t *testing.T) {
	for _, tc := range []struct {
		raw          string
		slot         uint16
		count, check uint32
	}{
		{"d20100009400000001004c00c80000002902000000000000", 76, 200, 553},
		{"d20100009400000001007c00e8030000c908000000000000", 124, 1000, 2249},
	} {
		p, _ := hex.DecodeString(tc.raw)
		for _, confirmed := range []bool{true, false} {
			if !confirmed {
				p[16] &^= 1
			}
			r, e := DecodeSellItem(p)
			if e != nil || r.NpcID != 466 || r.ActorID != 148 || r.Entries != 1 || r.List != 0 || r.Slot != tc.slot || r.Count != tc.count || r.Check&^1 != tc.check&^1 {
				t.Fatalf("native sale: %+v, %v", r, e)
			}
		}
	}
}

func TestDecodeSellItemRejectsMalformed(t *testing.T) {
	good, _ := hex.DecodeString("d20100009400000001007c00e8030000c908000000000000")
	for _, change := range []func([]byte) []byte{
		func(p []byte) []byte { return p[:20] },
		func(p []byte) []byte { return append(p, 0) },
		func(p []byte) []byte { p[23] = 1; return p },
		func(p []byte) []byte { p[8] = 2; return p },
		func(p []byte) []byte { binary.LittleEndian.PutUint32(p[12:], 0); return p },
		func(p []byte) []byte { binary.LittleEndian.PutUint32(p[12:], 0xffffffff); return p },
		func(p []byte) []byte { p[16] ^= 2; return p },
	} {
		if _, e := DecodeSellItem(change(append([]byte(nil), good...))); e == nil {
			t.Fatal("accepted malformed sale")
		}
	}
}

func TestBuyItemSuccessAcknowledgement(t *testing.T) {
	req := BuyItemRequest{
		Template: 1001,
		NpcID:    1,
		ActorID:  2,
		Count:    5,
		Category: 7,
	}
	record := OrdinaryItem(65, 1001, 5)
	ack, err := BuyItemSuccess(req, record, 9995, 65)
	if err != nil {
		t.Fatalf("BuyItemSuccess error: %v", err)
	}
	// 1 + 5*4 + 181 + 1 + 4 + 2 = 209 bytes
	if len(ack) != 209 {
		t.Fatalf("BuyItemSuccess width = %d, want 209", len(ack))
	}
	if ack[0] != 1 {
		t.Fatalf("BuyItemSuccess flag = %d, want 1", ack[0])
	}
	// Zero template / count rejected
	if _, err := BuyItemSuccess(BuyItemRequest{}, record, 100, 65); err == nil {
		t.Fatal("expected error on zero template/count")
	}
}

func TestSellItemSuccessAcknowledgement(t *testing.T) {
	// Single item: 16 bytes
	items := []SoldItem{{List: 0, Slot: 65, Count: 1000}}
	ack, err := SellItemSuccess(41997, items)
	if err != nil {
		t.Fatalf("SellItemSuccess error: %v", err)
	}
	// 1 + 4 + 4 + 7 = 16 bytes
	if len(ack) != 16 {
		t.Fatalf("SellItemSuccess width = %d, want 16", len(ack))
	}
	want, _ := hex.DecodeString("010da4000001000000004100e8030000")
	if !bytes.Equal(ack, want) {
		t.Fatalf("sell ACK %x, want %x", ack, want)
	}
	if ack[0] != 1 {
		t.Fatalf("SellItemSuccess flag = %d, want 1", ack[0])
	}

	// Multi-item path rejected
	multi := []SoldItem{
		{List: 0, Slot: 65, Count: 1},
		{List: 0, Slot: 66, Count: 1},
	}
	if _, err := SellItemSuccess(100, multi); err == nil {
		t.Fatal("expected error on multi-item sell")
	}
	if _, err := SellItemSuccess(0, nil); err == nil {
		t.Fatal("expected error on empty item sell")
	}
}
