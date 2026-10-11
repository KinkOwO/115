package protocol

import (
	"bytes"
	"encoding/binary"
	"os"
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
	if len(p) != 1+2+CurrentItemRecordSize || p[0] != 1 || binary.LittleEndian.Uint16(p[1:]) != 91 {
		t.Fatalf("unexpected CMD27 success header: %x", p[:3])
	}
	if binary.LittleEndian.Uint16(p[3:]) != 65 || binary.LittleEndian.Uint32(p[5:]) != 3600 || binary.LittleEndian.Uint32(p[9:]) != 1 {
		t.Fatalf("unexpected CMD27 reward row: %x", p[3:15])
	}
}

func TestLotteryEquipmentPotNativeSuccessCursor(t *testing.T) {
	// Actual 115 coat pot reward receipt: source 10416086 in slot 74,
	// equipment 100051280 in slot 27. No player identity is in this fixture.
	raw, err := os.ReadFile("testdata/lottery-pot-equipment-reward-20261007.bin")
	if err != nil || len(raw) != CurrentItemRecordSize {
		t.Fatalf("native item record fixture: len=%d err=%v", len(raw), err)
	}
	var reward [CurrentItemRecordSize]byte
	copy(reward[:], raw)
	p := LotteryItemSuccess(74, reward)
	// Dispatcher 1459A1CA2 reads one flag. 1459A1D0D..1D1B reads
	// an error word only on failure. Reader 14529EE3C then reads the
	// consumed slot, and 14529EF7E reads the entire 181-byte reward.
	r := bytes.NewReader(p)
	flag, err := r.ReadByte()
	if err != nil || flag != 1 {
		t.Fatalf("success flag: %d %v", flag, err)
	}
	var sourceSlot uint16
	if err := binary.Read(r, binary.LittleEndian, &sourceSlot); err != nil || sourceSlot != 74 {
		t.Fatalf("client would look up wrong source (and miss reform UI): slot=%d err=%v", sourceSlot, err)
	}
	var nativeRow [CurrentItemRecordSize]byte
	if err := binary.Read(r, binary.LittleEndian, &nativeRow); err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(nativeRow[:]) != 27 || binary.LittleEndian.Uint32(nativeRow[2:]) != 100051280 || nativeRow != reward || r.Len() != 0 {
		t.Fatalf("client reward cursor misaligned: row=%x remaining=%d", nativeRow[:18], r.Len())
	}
}
