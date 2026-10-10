package protocol

import (
	"encoding/binary"
	"testing"
)

func TestRaidAuctionNativeWriterBidBounds(t *testing.T) {
	for _, size := range []int{5, 8, 16} {
		p := make([]byte, size)
		p[0] = 1
		binary.LittleEndian.PutUint32(p[1:], 123456)
		got, err := DecodeRaidAuctionBid115(p)
		if err != nil || got.Pass || got.Amount != 123456 {
			t.Fatal(got, err)
		}
		if size > 5 {
			p[5] = 1
			if _, err = DecodeRaidAuctionBid115(p); err == nil {
				t.Fatal("nonzero cipher padding accepted")
			}
		}
	}
	for _, p := range [][]byte{nil, {1, 0, 0, 0}, {0, 0, 0, 0, 0}, {2, 1, 0, 0, 0}} {
		if _, err := DecodeRaidAuctionBid115(p); err == nil {
			t.Fatal("unsupported or truncated action accepted", p)
		}
	}
}

func TestRaidAuctionCurrentReaderRowsAndOwnerCursor(t *testing.T) {
	items, err := RaidAuctionItems115([]uint32{123, 456})
	if err != nil || len(items) != 30 || binary.LittleEndian.Uint32(items) != 2 {
		t.Fatal(items, err)
	}
	for i, id := range []uint32{123, 456} {
		row := items[4+i*13 : 4+(i+1)*13]
		if binary.LittleEndian.Uint32(row) != id || binary.LittleEndian.Uint32(row[4:]) != 1 {
			t.Fatal("native item preview", row)
		}
		for _, flag := range row[8:] {
			if flag != 0 {
				t.Fatal("equipment/optional branch unexpectedly entered")
			}
		}
	}
	start := RaidAuctionStart115(123, 100)
	if len(start) != 12 || binary.LittleEndian.Uint32(start) != 123 || binary.LittleEndian.Uint32(start[8:]) != 100 {
		t.Fatal(start)
	}
	joined := RaidAuctionJoin115(9, true)
	if len(joined) != 7 || binary.LittleEndian.Uint16(joined[4:]) != 9 || joined[6] != 1 {
		t.Fatal(joined)
	}
	for _, outcome := range []byte{0, 1, 2} {
		p, err := RaidAuctionUpdate115(9, "验收", 2, 500, outcome, 7123)
		if err != nil {
			t.Fatal(err)
		}
		// Reader1425c2dc0 uses UTF8 byte count, not rune/UTF16 character count.
		n := int(binary.LittleEndian.Uint32(p[3:]))
		at := 7 + n
		if p[0] != 1 || string(p[7:at]) != "验收" || p[at] != 2 || binary.LittleEndian.Uint32(p[at+9:]) != 500 || p[at+13] != outcome || binary.LittleEndian.Uint32(p[at+14:]) != 7123 || len(p) != at+18 {
			t.Fatal("native cursor mismatch", p)
		}
	}
	if _, err := RaidAuctionUpdate115(0, "owner", 0, 100, 0, 1); err == nil {
		t.Fatal("invalid owner accepted")
	}
}
