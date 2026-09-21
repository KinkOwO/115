package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// Live capture 2026-09-21T18:53: slot 10, no scroll, zero cipher padding.
func TestDecodeUnsealLiveVector(t *testing.T) {
	p, e := hex.DecodeString("0a00ffff00000000")
	if e != nil {
		t.Fatal(e)
	}
	r, e := DecodeUnseal(p)
	if e != nil {
		t.Fatal(e)
	}
	if r.TargetSlot != 10 || r.ScrollSlot != UnsealNoScrollSlot {
		t.Fatalf("live vector drifted: %+v", r)
	}
}

func TestDecodeUnsealRejects(t *testing.T) {
	if _, e := DecodeUnseal([]byte{0x0a}); e == nil {
		t.Fatal("short body accepted")
	}
	if _, e := DecodeUnseal([]byte{0x0a, 0, 0xff, 0xff, 1}); e == nil {
		t.Fatal("nonzero padding accepted")
	}
	if _, e := DecodeUnseal([]byte{0xff, 0xff, 0xff, 0xff}); e == nil {
		t.Fatal("sentinel target slot accepted")
	}
}

func TestUnsealResponses(t *testing.T) {
	if !bytes.Equal(UnsealSuccess(), []byte{1}) {
		t.Fatal("success status drifted")
	}
	if !bytes.Equal(UnsealRefused(4), []byte{0, 4, 0}) {
		t.Fatal("refusal layout drifted")
	}
}
