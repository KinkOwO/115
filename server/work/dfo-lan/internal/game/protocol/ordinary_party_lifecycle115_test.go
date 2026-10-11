package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestOrdinaryPartyLifecycleNativeVectors115(t *testing.T) {
	// native-r14-party-clear.json: actual 1452f2620 + 145f10390 execute the
	// unlink stores for owner/bot, clear all seats and call roster deletion.
	p, e := OrdinaryPartyGone115(2, [2]byte{1, 10})
	want, _ := hex.DecodeString("01000f270200010a00030100")
	if e != nil || !bytes.Equal(p, want) {
		t.Fatal(e, p)
	}
	for _, id := range []uint16{0, 65535} {
		if _, e := OrdinaryPartyGone115(id, [2]byte{}); e == nil {
			t.Fatal("invalid id accepted")
		}
	}
	for _, n := range []int{4, 8} {
		if e := DecodeOrdinaryPartyLeave115(make([]byte, n)); e != nil {
			t.Fatal(e)
		}
	}
	for _, p := range [][]byte{nil, {0}, {0, 0, 0}, make([]byte, 16), {0, 0, 0, 1}, {0, 0, 0, 0, 0, 0, 0, 1}} {
		if DecodeOrdinaryPartyLeave115(p) == nil {
			t.Fatalf("invalid leave accepted %x", p)
		}
	}
	live, _ := hex.DecodeString("01000000000000000000000000000000")
	if slot, e := DecodeOrdinaryPartyKick115(live); e != nil || slot != 1 {
		t.Fatal(slot, e)
	}
	for _, p := range [][]byte{nil, {0}, {4}, make([]byte, 8), {1, 1}, {1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}} {
		if _, e := DecodeOrdinaryPartyKick115(p); e == nil {
			t.Fatalf("invalid kick accepted %x", p)
		}
	}
}
