package protocol

import (
	"encoding/binary"
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

func TestDecodeSellItemMatchesRecoveredLayout(t *testing.T) {
	// 24 bytes: npcId=1, actorId=2, flag=1, list=0, slot=65, template=1, price=1001, pad=0,0,0,0
	var p [24]byte
	binary.LittleEndian.PutUint32(p[0:], 1)
	binary.LittleEndian.PutUint32(p[4:], 2)
	p[8] = 1
	p[9] = 0
	binary.LittleEndian.PutUint16(p[10:], 65)
	binary.LittleEndian.PutUint32(p[12:], 1)
	binary.LittleEndian.PutUint32(p[16:], 1001)

	r, err := DecodeSellItem(p[:])
	if err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	if r.NpcID != 1 || r.ActorID != 2 || r.Flag != 1 || r.List != 0 || r.Slot != 65 || r.Template != 1 || r.Price != 1001 {
		t.Fatalf("sell layout mismatch: %+v", r)
	}
}

func TestDecodeSellItemRejectsMalformed(t *testing.T) {
	var p [24]byte
	binary.LittleEndian.PutUint32(p[0:], 1)
	binary.LittleEndian.PutUint32(p[4:], 2)
	p[8] = 1
	p[9] = 0
	binary.LittleEndian.PutUint16(p[10:], 65)
	binary.LittleEndian.PutUint32(p[12:], 1)
	binary.LittleEndian.PutUint32(p[16:], 1001)

	// Wrong length
	if _, err := DecodeSellItem(p[:20]); err == nil {
		t.Fatal("expected error on unpadded 20 bytes")
	}

	// Non-zero padding
	dirty := p
	dirty[23] = 1
	if _, err := DecodeSellItem(dirty[:]); err == nil {
		t.Fatal("expected error on dirty padding")
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
	items := []SoldItem{{List: 0, Slot: 65, Template: 1001}}
	ack, err := SellItemSuccess(50, items)
	if err != nil {
		t.Fatalf("SellItemSuccess error: %v", err)
	}
	// 1 + 4 + 4 + 7 = 16 bytes
	if len(ack) != 16 {
		t.Fatalf("SellItemSuccess width = %d, want 16", len(ack))
	}
	if ack[0] != 1 {
		t.Fatalf("SellItemSuccess flag = %d, want 1", ack[0])
	}

	// Multi-item path rejected
	multi := []SoldItem{
		{List: 0, Slot: 65, Template: 1001},
		{List: 0, Slot: 66, Template: 1002},
	}
	if _, err := SellItemSuccess(100, multi); err == nil {
		t.Fatal("expected error on multi-item sell")
	}
	if _, err := SellItemSuccess(0, nil); err == nil {
		t.Fatal("expected error on empty item sell")
	}
}
