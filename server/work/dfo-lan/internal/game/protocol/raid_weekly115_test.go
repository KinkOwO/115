package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestRaidWeeklyClearInfo115MatchesOfficialBakalRow(t *testing.T) {
	h, err := os.ReadFile("testdata/raid_weekly_clear_info115.hex")
	if err != nil {
		t.Fatal(err)
	}
	native, err := hex.DecodeString(strings.TrimSpace(string(h)))
	if err != nil {
		t.Fatal(err)
	}
	pos, matched := 1, false
	for i := 0; i < int(native[0]); i++ {
		start := pos
		kind := native[pos]
		pos += 13
		n := int(native[pos])
		pos += 1 + n*13
		if kind != 8 {
			continue
		}
		want := append([]byte{1}, native[start:pos]...)
		got, err := RaidWeeklyClearInfo115(kind, 0, 0)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("native raid8 row differs: %x want %x err=%v", got, want, err)
		}
		matched = true
	}
	if !matched || pos != len(native) {
		t.Fatal("native weekly vector not fully consumed")
	}
}

func TestRaidWeeklyUsedCountersFeedNativeCreationGetters(t *testing.T) {
	p, err := RaidWeeklyClearInfo115(8, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Mirror the two independent native getters used by1420B2C90:
	// outer second u32 and key0 second u32. They store USED values.
	entryUsed := binary.LittleEndian.Uint32(p[6:])
	rewardUsed := binary.LittleEndian.Uint32(p[20:])
	if entryUsed != 1 || rewardUsed != 1 || p[15] != 0 {
		t.Fatal("weekly UI counters do not match entry/reward getter fields")
	}
	if binary.LittleEndian.Uint32(p[2:]) != 0 || binary.LittleEndian.Uint32(p[10:]) != 0 || binary.LittleEndian.Uint32(p[16:]) != 0 || binary.LittleEndian.Uint32(p[24:]) != 0 {
		t.Fatal("unmapped native fields were guessed from the weekly ledger")
	}
	if _, err := RaidWeeklyClearInfo115(8, 0, 0); err != nil {
		t.Fatal("fresh character zero-use row was rejected")
	}
	if _, err := RaidWeeklyClearInfo115(8, 0xffffffff, 0); err == nil {
		t.Fatal("used counter overflow would become native -1")
	}
}
