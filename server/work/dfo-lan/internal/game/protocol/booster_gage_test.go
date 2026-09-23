package protocol

import (
	"encoding/binary"
	"testing"
)

// TestBoosterGageBodyPins14Bytes guards the most expensive fact of NOTI398:
// the body is 14 bytes, not 18. A reader-shape guess encoding the last field
// as u64 (u64+u64+u8+u8 = 18) overruns the declared frame length and the
// client writer swallows the following CMDs (wire-overflow-report-217).
func TestBoosterGageBodyPins14Bytes(t *testing.T) {
	if p := BoosterGage(0); len(p) != 14 {
		t.Fatalf("BoosterGage(0) = %d bytes, want 14 (18-byte u64+u64+u8+u8 is wrong)", len(p))
	}
}

// TestBoosterGageFieldLayout pins the field positions of the 14-byte body:
// u8 incA @+0, u8 incB @+1 (both 0, they are cumulative increments), u64 rawA
// @+2, u32 displayValue @+10 (little endian).
func TestBoosterGageFieldLayout(t *testing.T) {
	p := BoosterGage(0)
	if p[0] != 0 || p[1] != 0 {
		t.Fatalf("increment bytes at +0/+1 = %d,%d, want 0,0", p[0], p[1])
	}
	if raw := binary.LittleEndian.Uint64(p[2:10]); raw != 0 {
		t.Fatalf("rawA at +2 = %d, want 0", raw)
	}
	if v := binary.LittleEndian.Uint32(p[10:14]); v != 0 {
		t.Fatalf("displayValue at +10 = %d, want 0", v)
	}
}

// TestBoosterGageDisplayValuePosition proves the fourth field is the one that
// moves and sits at the tail.
func TestBoosterGageDisplayValuePosition(t *testing.T) {
	p := BoosterGage(0x01020304)
	if len(p) != 14 {
		t.Fatalf("len = %d, want 14", len(p))
	}
	if v := binary.LittleEndian.Uint32(p[10:14]); v != 0x01020304 {
		t.Fatalf("displayValue at +10 = %#x, want 0x01020304", v)
	}
	if p[0] != 0 || p[1] != 0 {
		t.Fatalf("increment bytes must stay 0, got %d,%d", p[0], p[1])
	}
}
