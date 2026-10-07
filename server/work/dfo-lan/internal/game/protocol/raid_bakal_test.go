package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestBakalCapturedNativeRoomWarp(t *testing.T) {
	p, _ := hex.DecodeString("c0105e890100000089909a4501010000000500000001000000ffff7c01c201ffff320032000000000000000000000000")
	grid, record, err := DecodeBakalRoomWarp115(p)
	if err != nil || grid != [2]byte{1, 5} || binary.LittleEndian.Uint16(record[6:8]) != 380 || binary.LittleEndian.Uint16(record[8:10]) != 450 {
		t.Fatal(grid, record, err)
	}
	for _, size := range []int{0, 21, 38} {
		if _, _, err := DecodeBakalRoomWarp115(p[:size]); err == nil {
			t.Fatal("truncated warp accepted", size)
		}
	}
	p[13] = 255
	p[14] = 255
	if _, _, err := DecodeBakalRoomWarp115(p); err == nil {
		t.Fatal("out of maze warp accepted")
	}
}

func TestBakalNativeNotificationLayouts(t *testing.T) {
	// Fixed consumer layout: count, type1, location24, arrival1,
	// health120, three signed absent-party bytes. Template109014482
	// cannot replace the first u32.
	kind, err := BakalMonsterType("bakal")
	if err != nil || kind != 1 {
		t.Fatal(kind, err)
	}
	got, err := BakalMonsterInfoPayload([]BakalMonsterInfo{{Kind: kind, Location: 24, Action: 1, Health: 120, Parties: [3]int8{-1, -1, -1}}})
	want, _ := hex.DecodeString("0101000000180000000100000078000000ffffff")
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("NOTI2286 layout: %x, %v", got, err)
	}
	got, err = BakalBuffInfoPayload([5]byte{2, 2, 2, 2, 2}, 25, ^uint32(0))
	want, _ = hex.DecodeString("020202020219000000ffffffff")
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("NOTI2288 layout: %x, %v", got, err)
	}
	p := BakalPartyInfo{Party: 1, Location: 52}
	for i := range p.Buffs {
		p.Buffs[i].Kind = 25
	}
	got, err = BakalPartyInfoPayload([]BakalPartyInfo{p})
	want, _ = hex.DecodeString("01010000003400000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000" + "1900000000000000")
	if err != nil || len(got) != 173 || !bytes.Equal(got, want) {
		t.Fatalf("NOTI2285 empty buff sentinels: %x, %v", got, err)
	}
}

func TestBakalNativeReportNamespaces(t *testing.T) {
	p := make([]byte, 24)
	p[13] = 1
	binary.LittleEndian.PutUint32(p[14:], 26)
	binary.LittleEndian.PutUint32(p[18:], 4321)
	slot, hp, err := DecodeBakalHealthReport115(p)
	if err != nil || slot != 26 || hp != 4321 {
		t.Fatal(slot, hp, err)
	}
	p[22] = 1
	if _, _, err = DecodeBakalHealthReport115(p); err == nil {
		t.Fatal("nonzero health padding accepted")
	}
	b := make([]byte, 24)
	binary.LittleEndian.PutUint32(b[13:], 22)
	index, err := DecodeBakalBuffUse115(b)
	if err != nil || index != 22 {
		t.Fatal(index, err)
	}
	b[17] = 1
	if _, err = DecodeBakalBuffUse115(b); err == nil {
		t.Fatal("nonzero buff padding accepted")
	}
}

func TestBakalNotificationsRejectWrongNamespacesAndLocations(t *testing.T) {
	for _, m := range []BakalMonsterInfo{
		{Kind: 109014482, Location: 24, Action: 1},
		{Kind: 1, Location: 52, Action: 1},
		{Kind: 1, Location: 0, Action: 1},
		{Kind: 1, Location: 24, Action: 9},
		{Kind: 1, Location: 24, Action: 1, Parties: [3]int8{-2, -1, -1}},
	} {
		if _, err := BakalMonsterInfoPayload([]BakalMonsterInfo{m}); err == nil {
			t.Fatalf("invalid monster accepted: %+v", m)
		}
	}
	m := BakalMonsterInfo{Kind: 1, Location: 24, Action: 1}
	if _, err := BakalMonsterInfoPayload([]BakalMonsterInfo{m, m}); err == nil {
		t.Fatal("duplicate battlefield occupancy accepted")
	}
	if _, err := BakalMonsterType("foreign"); err == nil {
		t.Fatal("unknown monster enum accepted")
	}
	if _, err := BakalPartyInfoPayload([]BakalPartyInfo{{Party: 1, Location: 52}, {Party: 1, Location: 53}}); err == nil {
		t.Fatal("duplicate party accepted")
	}
	if _, err := BakalBuffInfoPayload([5]byte{}, 26, 0); err == nil {
		t.Fatal("unknown used buff accepted")
	}
}

// Compare the native result reader's ten-byte logical record with a captured
// successful result: phase zero is the single-phase Bakal branch.
func TestBakalClearResultNativeRecord(t *testing.T) {
	want, _ := hex.DecodeString("00000701000000000001")
	if got := BakalClearResult115(263); !bytes.Equal(got, want) {
		t.Fatalf("native clear result: %x, want %x", got, want)
	}
}
