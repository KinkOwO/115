package protocol

import (
	"encoding/binary"
	"testing"
)

func TestDecodeBoosterPetSelectionLowByte(t *testing.T) {
	for low := uint32(0); low < 256; low++ {
		id := uint32(783000064) + low
		p := make([]byte, 16)
		binary.LittleEndian.PutUint16(p, 65)
		binary.LittleEndian.PutUint32(p[2:], 1)
		binary.LittleEndian.PutUint32(p[8:], id)
		r, e := DecodeBoosterUseRequest(p)
		if e != nil || len(r.Selections) != 1 || r.Selections[0] != id || len(r.AvatarOptions) != 0 {
			t.Fatalf("pet %d (low byte %02x): decoded %+v, error %v", id, low, r, e)
		}
	}
}
