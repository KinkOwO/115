package protocol

import (
	"encoding/binary"
	"os"
	"testing"
)

func TestLegionRewardStableSeatsAndEquipmentValue(t *testing.T) {
	var rows [4][]ConquestRewardValue115
	rows[0] = []ConquestRewardValue115{{Template: 700001, Value: 999999999, Equipment: true, Metadata: [21]byte{1, 2, 3}}}
	rows[2] = []ConquestRewardValue115{{Template: 700002, Value: 5}}
	p, e := LegionBasicRewards115([4]bool{true, false, true}, rows, 12345)
	if e != nil || len(p) != 7772 {
		t.Fatal(len(p), e)
	}
	if binary.LittleEndian.Uint32(p) != 700001 || binary.LittleEndian.Uint32(p[4:]) != 999999999 || p[14] != 1 || binary.LittleEndian.Uint32(p[800:]) != 700002 || binary.LittleEndian.Uint64(p[7760:]) != 12345 {
		t.Fatal("native fields changed")
	}
	if binary.LittleEndian.Uint32(p[400:]) != 0 {
		t.Fatal("sparse seat was compacted")
	}
	if _, e = LegionBasicRewards115([4]bool{true}, rows, 1); e == nil {
		t.Fatal("absent actor rewarded")
	}
}

func TestLegionSixStarsUseNativeExtraItemRowsWithoutMerging(t *testing.T) {
	var rows [4][]ConquestRewardValue115
	for i := 0; i < 6; i++ {
		rows[3] = append(rows[3], ConquestRewardValue115{Equipment: true, Template: 100401603, Value: uint32(i), Metadata: [21]byte{byte(i + 1)}})
	}
	p, e := LegionBasicRewards115([4]bool{false, false, false, true}, rows, 42)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 6; i++ {
		off := 1200 + 40*i
		if i >= 2 {
			off = 1600 + 1540*3 + 44*(i-2)
		}
		if binary.LittleEndian.Uint32(p[off:]) != 100401603 || binary.LittleEndian.Uint32(p[off+4:]) != uint32(i) || p[off+14] != byte(i+1) {
			t.Fatal("lost independent stone row", i)
		}
		if i >= 2 && binary.LittleEndian.Uint32(p[off+40:]) != 0 {
			t.Fatal("duplicate template would be merged")
		}
	}
	if binary.LittleEndian.Uint32(p[1280:]) != 0 || binary.LittleEndian.Uint32(p[1600:]) != 0 {
		t.Fatal("overwrote unrelated category/seat")
	}
	if out := os.Getenv("US115_TEST_LEGION_REWARD_VECTOR"); out != "" {
		if e := os.WriteFile(out, p, 0600); e != nil {
			t.Fatal(e)
		}
	}
}
