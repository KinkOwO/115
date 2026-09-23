package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestSynopsisLiveRequests(t *testing.T) {
	// Player-labelled open, next page and X clicks, 2026-09-23 13:10–13:12 UTC.
	for _, h := range []string{
		"800cc724010000000096f56d010200000000000000000000",
		"205bf06d010000000096f56d010200000000000000000000",
		"40acde270100000080a06d0b010200000000000000000000",
	} {
		p, _ := hex.DecodeString(h)
		id, err := DecodeSynopsisRead(p)
		if err != nil || id != 2 {
			t.Fatalf("%s: id=%d err=%v", h, id, err)
		}
	}
	p := make([]byte, 24)
	for _, size := range []int{0, 4, 16, 17, 23} {
		if _, err := DecodeSynopsisRead(p[:size]); err == nil {
			t.Fatalf("accepted length %d", size)
		}
	}
	p[17] = 1
	if _, err := DecodeSynopsisRead(p); err == nil {
		t.Fatal("accepted nonzero padding")
	}
	p[17], p[16] = 0, 128
	if _, err := DecodeSynopsisRead(p); err == nil {
		t.Fatal("accepted negative ID")
	}
}

func TestSynopsisNativeReaderLayout(t *testing.T) {
	// Expected bytes follow the native reader, not a server encode/decode loop.
	p, err := SynopsisTableInfo([]uint32{2, 0x12345678})
	if err != nil || !bytes.Equal(p, []byte{0, 2, 0, 2, 0, 0, 0, 0x78, 0x56, 0x34, 0x12}) {
		t.Fatalf("body=%x err=%v", p, err)
	}
	p, err = SynopsisTableInfo(nil)
	if err != nil || !bytes.Equal(p, []byte{0, 0, 0}) {
		t.Fatalf("empty=%x %v", p, err)
	}
	if _, err = SynopsisTableInfo(make([]uint32, 32768)); err == nil {
		t.Fatal("signed count overflow")
	}
	if _, err = SynopsisTableInfo([]uint32{0x80000000}); err == nil {
		t.Fatal("signed ID overflow")
	}
}
