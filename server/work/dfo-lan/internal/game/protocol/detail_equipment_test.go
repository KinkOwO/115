package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestDetailedEquipmentEmptyCompatibility(t *testing.T) {
	p, e := DetailedEquipment(nil)
	if e != nil || !bytes.Equal(p, make([]byte, 14)) {
		t.Fatalf("empty block: %x %v", p, e)
	}
}

func TestDetailedEquipmentAvatarNativeOffsets(t *testing.T) {
	p, e := DetailedEquipment([]DetailedWorn{{Slot: 3, Template: 40601, Durability: 9, Period: 123}})
	if e != nil {
		t.Fatal(e)
	}
	if len(p) != 149 || p[0] != 1 || p[1] != 3 || binary.LittleEndian.Uint32(p[2:]) != 40601 || binary.LittleEndian.Uint16(p[11:]) != 9 || binary.LittleEndian.Uint32(p[50:]) != 123 || p[54] != 0 {
		t.Fatalf("native row offsets: %x", p)
	}
}

func TestDetailedEquipmentPreservesAvatarBlobs(t *testing.T) {
	p, e := DetailedEquipment([]DetailedWorn{{Slot: 3, Template: 40601, AvatarOptions: []byte{1, 2}, AvatarSockets: []byte{3}}})
	if e != nil {
		t.Fatal(e)
	}
	if binary.LittleEndian.Uint32(p[41:]) != 2 || !bytes.Equal(p[45:47], []byte{1, 2}) || binary.LittleEndian.Uint32(p[47:]) != 1 || p[51] != 3 {
		t.Fatalf("blob projection: %x", p)
	}
}

func TestDetailedEquipmentRejectsUnsupportedInstances(t *testing.T) {
	cases := [][]DetailedWorn{
		{{Slot: 12, Template: 1}}, {{Slot: 26, Template: 1}},
		{{Slot: 48, Template: 1}}, {{Slot: 1}},
		{{Slot: 1, Template: 1}, {Slot: 1, Template: 2}},
		{{Slot: 1, Template: 1, Record: []byte{1}}},
		{{Slot: 1, Template: 1, AvatarOptions: make([]byte, 4097)}},
	}
	for _, rows := range cases {
		if _, e := DetailedEquipment(rows); e == nil {
			t.Fatalf("accepted unsupported rows: %+v", rows)
		}
	}
}
