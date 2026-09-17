package protocol

import (
	"encoding/binary"
	"testing"
)

func TestCinematicSkipBitmap(t *testing.T) {
	p, e := CinematicSkippedScenes([]uint16{0, 7, 8, 39999})
	if e != nil || len(p) != 5004 || binary.LittleEndian.Uint32(p) != 5000 || p[4] != 129 || p[5] != 1 || p[5003] != 128 {
		t.Fatal("bitmap", e)
	}
	for _, id := range []uint16{0, 7, 8, 39999} {
		raw := make([]byte, 8)
		binary.LittleEndian.PutUint16(raw, id)
		got, e := DecodeCinematicSkip(raw)
		if e != nil || got != id {
			t.Fatal(got, e)
		}
	}
	if _, e := DecodeCinematicSkip([]byte{0x40, 0x9c}); e == nil {
		t.Fatal("invalid scene")
	}
	if _, e := CinematicSkippedScenes([]uint16{40000}); e == nil {
		t.Fatal("invalid bitmap")
	}
}
