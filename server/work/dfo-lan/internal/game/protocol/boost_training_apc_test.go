package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestBoostTrainingAPCNativeRecordAndPreservedModes(t *testing.T) {
	base, e := AdventureEliteSelections([]AdventureEliteSelection{{Mode: 2, Slots: [3]int32{4, -1, -1}}, {Mode: 3, Slots: [3]int32{7, 8, 9}}, {Mode: 4, Slots: [3]int32{-1, 2, -1}}})
	if e != nil {
		t.Fatal(e)
	}
	original := append([]byte(nil), base...)
	got, e := BoostTrainingAPCSettings(base, []uint32{8123})
	if e != nil {
		t.Fatal(e)
	}
	if len(got) != 1+3*531+2 || got[0] != 3 || !bytes.Equal(base, original) {
		t.Fatal("wrong length/count or mutated account snapshot")
	}
	if !bytes.Equal(got[1:532], base[1:532]) || !bytes.Equal(got[1063:1594], base[1063:1594]) {
		t.Fatal("other modes changed")
	}
	row := got[532:1063]
	if binary.LittleEndian.Uint16(row[0xd:]) != 3 || row[0x10] != 1 || binary.LittleEndian.Uint32(row[0x17:]) != 8123 {
		t.Fatal("source APC binding missing")
	}
	for i := 0; i < 4; i++ {
		if int32(binary.LittleEndian.Uint32(row[0x23+i*4:])) != -1 {
			t.Fatal("server invented a client cache slot")
		}
	}
	if got[len(got)-1] != 0 || got[len(got)-2] != 0 {
		t.Fatal("native trailing tables absent")
	}
	empty, e := BoostTrainingAPCSettings([]byte{0}, []uint32{3577, 8123})
	if e != nil || empty[0] != 1 || empty[1+0x11] != 1 {
		t.Fatal("source count not followed", e)
	}
	for _, ids := range [][]uint32{nil, {0}, {1, 1}, {1, 2, 3, 4}, {1 << 31}} {
		if _, e := BoostTrainingAPCSettings(base, ids); e == nil {
			t.Fatal("bad source accepted", ids)
		}
	}
}
func TestBoostTrainingAPCRequestCapturedPadding(t *testing.T) {
	for _, p := range [][]byte{{1}, append([]byte{1}, make([]byte, 15)...)} {
		if e := DecodeBoostTrainingAPCRequest(p); e != nil {
			t.Fatal(e)
		}
	}
	for _, p := range [][]byte{nil, {0}, {2}, {1, 1}, append([]byte{1}, make([]byte, 17)...)} {
		if e := DecodeBoostTrainingAPCRequest(p); e == nil {
			t.Fatal("malformed request accepted")
		}
	}
}
