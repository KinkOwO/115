package protocol

import (
	"bytes"
	"encoding/hex"
	"testing"
)

// The native CMT13042 closing record is from the manual 2026-09-29 run.
// IDB 1452b787f/7876 clears the layer ordinal only for flag 2. Mode 0
// consumes neither a map ID nor spawn rows, preserving the base cache.
func TestStartMapCachedLayerExit(t *testing.T) {
	record := [18]byte{0, 0, 0, 0, 4, 5, 165, 0, 33, 1}
	p, err := StartMap(StartMapState{Position: [2]byte{1, 1}, Seed: 7, Map: 100004325,
		ReuseRoom: true, ExitLayer: true, Transition: &record})
	if err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString("010102070000000000ffffffff000000000405a500210100000000000000000000ff")
	if err != nil || !bytes.Equal(p, want) {
		t.Fatalf("cached layer exit packet: got %x want %x err=%v", p, want, err)
	}
	for _, s := range []StartMapState{
		{Map: 100004325, ReuseRoom: true, ExitLayer: true},
		{Map: 100004325, ReuseRoom: true, ExitLayer: true, LayerChange: true, Transition: &record},
	} {
		if _, err := StartMap(s); err == nil {
			t.Fatal("accepted ambiguous or missing layer exit record")
		}
	}
}
