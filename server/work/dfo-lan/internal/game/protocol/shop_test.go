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
			if e != nil || r.NpcID != 466 || r.ActorID != 148 || len(r.Rows) != 1 {
				t.Fatalf("native sale: %+v, %v", r, e)
			}
			row := r.Rows[0]
			if row.List != 0 || row.Slot != tc.slot || row.Count != tc.count || row.Check&^1 != tc.check&^1 {
				t.Fatalf("native sale row: %+v", row)
			}
		}
	}
}

// Captured from the user's client, 2026-09-27 15:32 UTC. The "Sell All" panel
// registered seven 1-count stacks (slots 10, 11, 12, 13, 14, 43, 48) and
// confirmed them with one 88-byte CMD22. The old decoder accepted only the
// 24-byte single-row layout, so the panel's OK button did nothing.
func TestDecodeSellItemBatchRequest(t *testing.T) {
	raw := "010000000200000007000a000100000016000000000b000100000018000000000c00010000001a000000000d00010000001c000000000e00010000001e000000002b00010000005800000000300001000000620000000000"
	p, _ := hex.DecodeString(raw)
	if len(p) != 88 {
		t.Fatalf("fixture is %d bytes", len(p))
	}
	r, e := DecodeSellItem(p)
	if e != nil || r.NpcID != 1 || r.ActorID != 2 || len(r.Rows) != 7 {
		t.Fatalf("batch sale: %+v, %v", r, e)
	}
	want := []struct {
		slot  uint16
		check uint32
	}{{10, 22}, {11, 24}, {12, 26}, {13, 28}, {14, 30}, {43, 88}, {48, 98}}
	for i, w := range want {
		row := r.Rows[i]
		if row.List != 0 || row.Slot != w.slot || row.Count != 1 || row.Check != w.check {
			t.Fatalf("batch row %d: %+v, want slot %d check %d", i, row, w.slot, w.check)
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

	// Multi-item ("Sell All") path: 1 + 4 + 4 + 7*N bytes
	multi := []SoldItem{
		{List: 0, Slot: 65, Count: 1},
		{List: 0, Slot: 66, Count: 2},
	}
	ack, err = SellItemSuccess(100, multi)
	if err != nil {
		t.Fatalf("SellItemSuccess multi error: %v", err)
	}
	if len(ack) != 9+7*len(multi) {
		t.Fatalf("multi SellItemSuccess width = %d, want %d", len(ack), 9+7*len(multi))
	}
	wantMulti, _ := hex.DecodeString("0164000000020000000041000100000000420002000000")
	if !bytes.Equal(ack, wantMulti) {
		t.Fatalf("multi sell ACK %x, want %x", ack, wantMulti)
	}
	if _, err := SellItemSuccess(0, nil); err == nil {
		t.Fatal("expected error on empty item sell")
	}
}
