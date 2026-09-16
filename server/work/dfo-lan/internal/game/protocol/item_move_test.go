package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestCurrentItemMoveAckCursors(t *testing.T) {
	r := ItemMoveRequest{SourceList: 0, SourceSlot: 9, DestinationList: 3, DestinationSlot: 19}
	if !bytes.Equal(ItemMoveSuccess(r, 1)[1:], nativeInventoryFixture(t, "item_move_success35")) {
		t.Fatal("success cursor")
	}
	if !bytes.Equal(ItemMoveRefused(r)[3:], nativeInventoryFixture(t, "item_move_failure35")) {
		t.Fatal("refusal cursor")
	}
	p := make([]byte, 32)
	p[11] = 3
	binary.LittleEndian.PutUint16(p[1:], 9)
	binary.LittleEndian.PutUint32(p[3:], 20002)
	binary.LittleEndian.PutUint32(p[7:], 1)
	binary.LittleEndian.PutUint16(p[12:], 19)
	binary.LittleEndian.PutUint32(p[22:], 0xffffffff)
	got, e := DecodeItemMove(p)
	if e != nil || got.SourceSlot != 9 || got.DestinationSlot != 19 || got.SourceItem != 20002 || got.Selection != 0xffffffff {
		t.Fatal(got, e)
	}
	p[31] = 1
	if _, e = DecodeItemMove(p); e == nil {
		t.Fatal("nonzero tail accepted")
	}
}
