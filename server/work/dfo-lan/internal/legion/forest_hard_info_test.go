package legion

import (
	"encoding/binary"
	"testing"
)

func TestForestHardNativeReaderAlignmentAndConstructorDefaults(t *testing.T) {
	for phase := int32(0); phase < 3; phase++ {
		p, err := ForestHardNativeInfo(2, 0, phase)
		if err != nil || len(p) != 0x36 {
			t.Fatal("native reader length", err)
		}
		//141D42030's direct readers are state@2/outcome@6/phase@10; inserting
		// the normal choice byte shifts all three and prevents the phase timer.
		if binary.LittleEndian.Uint32(p[2:]) != 2 || binary.LittleEndian.Uint32(p[6:]) != 0 || int32(binary.LittleEndian.Uint32(p[10:])) != phase {
			t.Fatal("hard fields shifted into normal layout")
		}
		if binary.LittleEndian.Uint16(p) != 0xffff || binary.LittleEndian.Uint32(p[14:]) != ^uint32(0) {
			t.Fatal("opaque constructor defaults changed")
		}
		for row := 0; row < 3; row++ {
			at := 18 + 12*row
			if p[at] != 255 || p[at+1] != 0 || p[at+2] != 0 || p[at+3] != 0 || binary.LittleEndian.Uint64(p[at+4:]) != ^uint64(0) {
				t.Fatal("native12-byte row default changed")
			}
		}
	}
	for _, v := range [][3]int32{{2, 3, 0}, {3, 1, 1}, {5, 1, -1}, {2, 0, 3}, {14, 4, 0}} {
		if _, err := ForestHardNativeInfo(uint32(v[0]), uint32(v[1]), v[2]); err == nil {
			t.Fatal("invalid phase/state accepted", v)
		}
	}
}
