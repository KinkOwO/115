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

func TestDetailedEquipmentVisualOverrideCells(t *testing.T) {
	p, e := DetailedEquipment([]DetailedWorn{{
		Slot: 1, Template: 517560000,
		HeaderTemplateA: 517562678,
	}})
	if e != nil {
		t.Fatal(e)
	}
	// The native mode-1 reader consumes two u32 cells at row offsets 24/28.
	// The block begins with a one-byte row count.
	if got := binary.LittleEndian.Uint32(p[25:]); got != 517562678 {
		t.Fatalf("appearance override cell=%d", got)
	}
	if got := binary.LittleEndian.Uint32(p[29:]); got != 0 {
		t.Fatalf("random-avatar secondary cell=%d", got)
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
		{{Slot: 12, Template: 1}}, {{Slot: 25, Template: 1}},
		{{Slot: 48, Template: 1}}, {{Slot: 1}},
		{{Slot: 1, Template: 1}, {Slot: 1, Template: 2}},
		{{Slot: 1, Template: 1, Record: []byte{1}}},
		{{Slot: 1, Template: 1, AvatarOptions: make([]byte, 4097)}},
		{{Slot: 26, Template: 1, AvatarOptions: []byte{1}}},
		{{Slot: 27, Template: 1, AvatarSockets: []byte{1}}},
		{{Slot: 26, Template: 1, HeaderTemplateA: 2}},
	}
	for _, rows := range cases {
		if _, e := DetailedEquipment(rows); e == nil {
			t.Fatalf("accepted unsupported rows: %+v", rows)
		}
	}
}

// Creature row layout, pinned against native reader sub_1452C1540: 40-byte
// header, then the 5-byte creature extension @0x1452c186f (u32 + u8, gated on
// itemdef+2120 == 26), then the shared 87-byte tail; NO avatar blobs (the
// avatar dispatcher sub_145A83FA0 only fires for itemdef type <= 11).
func TestDetailedEquipmentCreatureRow(t *testing.T) {
	p, e := DetailedEquipment([]DetailedWorn{{Slot: 26, Template: 500991361, Durability: 7, Period: 99, Record: []byte{1, 2, 3}}})
	if e != nil {
		t.Fatal(e)
	}
	// 1 count + 132 row (40 header + 5 ext + 87 tail) + 13 trailer.
	if len(p) != 146 || p[0] != 1 {
		t.Fatalf("creature block size=%d, want 146", len(p))
	}
	if p[1] != 26 || binary.LittleEndian.Uint32(p[2:]) != 500991361 || binary.LittleEndian.Uint16(p[11:]) != 7 {
		t.Fatalf("creature header: %x", p)
	}
	// Extension all zero, then tail: aux count, period, nested count.
	if !bytes.Equal(p[41:46], make([]byte, 5)) {
		t.Fatalf("creature extension must be five zero bytes: %x", p[41:46])
	}
	if p[46] != 0 || binary.LittleEndian.Uint32(p[47:]) != 99 || p[51] != 0 {
		t.Fatalf("creature tail offsets: %x", p)
	}
}

// 幻化槽（穿戴槽 32）装的是 [creature] 物品，物品定义类型与槽 26 同为 26，而
// 原生 reader sub_1452C1540 是按定义类型分派的，所以它拿到的仍是 creature 行
// 布局（40 字节头 + 5 字节扩展 + 87 字节尾，无头像 blob）。
func TestDetailedEquipmentCreatureSkinRow(t *testing.T) {
	p, e := DetailedEquipment([]DetailedWorn{{Slot: 32, Template: 63008, Durability: 3, Period: 2147483647}})
	if e != nil {
		t.Fatal(e)
	}
	if len(p) != 146 || p[0] != 1 {
		t.Fatalf("幻化槽行块大小=%d，期望 146", len(p))
	}
	if p[1] != 32 || binary.LittleEndian.Uint32(p[2:]) != 63008 || binary.LittleEndian.Uint16(p[11:]) != 3 {
		t.Fatalf("幻化槽行头: %x", p)
	}
	if !bytes.Equal(p[41:46], make([]byte, 5)) {
		t.Fatalf("幻化槽扩展必须是五个零字节: %x", p[41:46])
	}
	if p[46] != 0 || binary.LittleEndian.Uint32(p[47:]) != 2147483647 || p[51] != 0 {
		t.Fatalf("幻化槽行尾偏移: %x", p)
	}
}

// Creature gear slots 27..29 ride the plain row layout: no avatar blobs, no
// creature extension.
func TestDetailedEquipmentCreatureGearRow(t *testing.T) {
	p, e := DetailedEquipment([]DetailedWorn{{Slot: 27, Template: 100950255}})
	if e != nil {
		t.Fatal(e)
	}
	// 1 count + 127 row (40 header + 87 tail) + 13 trailer.
	if len(p) != 141 || p[0] != 1 || p[1] != 27 || binary.LittleEndian.Uint32(p[2:]) != 100950255 {
		t.Fatalf("creature gear block: %x", p)
	}
	if binary.LittleEndian.Uint32(p[42:]) != 0 {
		t.Fatalf("plain row tail starts with the auxiliary pair count: %x", p)
	}
}
