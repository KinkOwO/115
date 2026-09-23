package protocol

import "testing"

func TestCachedFinalLayerStartMap(t *testing.T) {
	record := [18]byte{0, 0, 0, 0, 4, 5, 0xbd, 2, 0xe5, 0, 0, 0, 3, 0, 2, 0, 0, 0}
	p, err := StartMap(StartMapState{Position: [2]byte{3, 0}, Map: 100008697, ReuseRoom: true, LayerChange: true, Transition: &record})
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 34 || p[2] != 1 || p[31] != 0 || p[32] != 0 || p[33] != 255 {
		t.Fatalf("cached final layer shape: %x", p)
	}
	for i, b := range record {
		if p[13+i] != b {
			t.Fatalf("transition byte %d: got %x want %x", i, p[13+i], b)
		}
	}
}
