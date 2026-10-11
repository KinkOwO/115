package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestTitleBookNativePutCapture(t *testing.T) {
	p, _ := hex.DecodeString("0000000033000000c76ad92e040000000a000000000000000000000000000000")
	r, e := DecodeTitleBookTransfer(p)
	if e != nil {
		t.Fatal(e)
	}
	if r != (TitleBookTransfer{0, 51, 786000583, 4, 10}) {
		t.Fatalf("native request decoded as %+v", r)
	}
	p[31] = 1
	if _, e = DecodeTitleBookTransfer(p); e == nil {
		t.Fatal("accepted nonzero padding")
	}
}
func TestTitleBookCompactNativeReaderLayout(t *testing.T) {
	p, e := TitleBookList(4, []TitleBookRow{{Slot: 10, Instance: 123, Template: 786000583, Reinforcement: 2, Durability: 3, Sealed: 1, Enchant: 765, EnchantUpgrade: 4, AmplifyType: 5, AmplifyValue: 513, RandomOptions: []byte{6, 7, 8, 9}, Clone: 786000585, Flag: 1}})
	if e != nil {
		t.Fatal(e)
	}
	if len(p) != 11+31+4 || binary.LittleEndian.Uint32(p[3:]) != 4 || binary.LittleEndian.Uint32(p[7:]) != 1 {
		t.Fatalf("invalid header or length: %x", p)
	}
	r := p[11:]
	if binary.LittleEndian.Uint16(r) != 10 || binary.LittleEndian.Uint32(r[2:]) != 786000583 || binary.LittleEndian.Uint32(r[6:]) != 123 || r[10] != 2 || binary.LittleEndian.Uint16(r[11:]) != 3 || r[13] != 1 || binary.LittleEndian.Uint32(r[14:]) != 765 || r[18] != 4 || r[19] != 5 || binary.LittleEndian.Uint16(r[20:]) != 513 || binary.LittleEndian.Uint32(r[22:]) != 4 || binary.LittleEndian.Uint32(r[30:]) != 786000585 || r[34] != 1 {
		t.Fatalf("reader fields shifted: %x", r)
	}
}

func TestTitleBookNativeTemplateDataPrefix(t *testing.T) {
	// Native NOTI344 category 1: slot 40, PVF title 26673, item data 1.
	want, _ := hex.DecodeString("28003168000001000000")
	p, err := TitleBookList(1, []TitleBookRow{{Slot: 40, Template: 26673, Instance: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(p[11:21]) != hex.EncodeToString(want) {
		t.Fatalf("native template/data prefix differs: %x", p[11:21])
	}
}
