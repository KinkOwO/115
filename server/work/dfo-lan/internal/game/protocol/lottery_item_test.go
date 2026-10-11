package protocol

import (
	"encoding/binary"
	"testing"
)

func TestLotteryItemNativeRequestAndSuccessLayout(t *testing.T) {
	slot, err := DecodeLotteryItemUse([]byte{0x5b, 0, 0, 0, 0, 0, 0, 0})
	if err != nil || slot != 91 {
		t.Fatalf("captured request: slot=%d err=%v", slot, err)
	}
	for _, bad := range [][]byte{{0x5b}, {0x5b, 0, 1, 0, 0, 0}, {0, 0, 0, 0, 0, 0}} {
		if _, err := DecodeLotteryItemUse(bad); err == nil {
			t.Fatalf("accepted invalid request %x", bad)
		}
	}
	reward := OrdinaryItem(65, 3600, 1)
	p := LotteryItemSuccess(slot, reward)
	if len(p) != 1+2+2+CurrentItemRecordSize || p[0] != 1 || binary.LittleEndian.Uint16(p[1:]) != 0 || binary.LittleEndian.Uint16(p[3:]) != 91 {
		t.Fatalf("unexpected CMD27 success header: %x", p[:5])
	}
	if binary.LittleEndian.Uint16(p[5:]) != 65 || binary.LittleEndian.Uint32(p[7:]) != 3600 || binary.LittleEndian.Uint32(p[11:]) != 1 {
		t.Fatalf("unexpected CMD27 reward row: %x", p[5:17])
	}
}
