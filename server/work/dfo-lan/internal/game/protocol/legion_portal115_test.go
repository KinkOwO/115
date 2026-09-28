package protocol

import (
	"encoding/binary"
	"testing"
)

func TestLegionPortal115UsesInnerPrefixAndRejectsMalformedFields(t *testing.T) {
	for _, length := range []int{46, 48} {
		p := make([]byte, length)
		for i := 0; i < 12; i++ {
			p[i] = 255
		}
		binary.LittleEndian.PutUint32(p[13:], 100004995)
		binary.LittleEndian.PutUint32(p[17:], 2)
		// Position bytes are not portal authority and need not be zero.
		binary.LittleEndian.PutUint32(p[25:], 999)
		r, e := DecodeLegionPortal115(p)
		if e != nil || r.Dungeon != 100004995 || r.Difficulty != 2 {
			t.Fatal(r, e)
		}
		for _, off := range []int{17, 21} {
			bad := append([]byte(nil), p...)
			binary.LittleEndian.PutUint32(bad[off:], 9)
			if _, e = DecodeLegionPortal115(bad); e == nil {
				t.Fatal("invalid field", off)
			}
		}
		if length > 46 {
			p[46] = 1
			if _, e = DecodeLegionPortal115(p); e == nil {
				t.Fatal("padding became field")
			}
		}
	}
	for _, n := range []int{0, 13, 45, 62, 100} {
		if _, e := DecodeLegionPortal115(make([]byte, n)); e == nil {
			t.Fatal("size", n)
		}
	}
	if len(LegionDirectMoveNotice115()) != 38 {
		t.Fatal("direct-move consumer width")
	}
}

func TestLegionPhysicalWarpFormulaParametersDecodeWithoutChoosingPosition(t *testing.T) {
	p := make([]byte, 48)
	for i := 0; i < 12; i++ {
		p[i] = 255
	}
	binary.LittleEndian.PutUint32(p[13:], 100005111)
	binary.LittleEndian.PutUint32(p[17:], 2)
	for i, v := range []uint32{0, 0, 300, 300, 500, 300} {
		binary.LittleEndian.PutUint32(p[21+4*i:], v)
	}
	p[45] = 7
	r, e := DecodeLegionPortal115(p)
	if e != nil || r.Dungeon != 100005111 || r.TargetGrid != [2]int32{} || r.SpawnWindow != [4]int32{300, 300, 500, 300} || r.Mode != 7 {
		t.Fatal(r, e)
	}
}
