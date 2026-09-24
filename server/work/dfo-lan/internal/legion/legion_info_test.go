package legion

import (
	"encoding/binary"
	"testing"
)

// The sizes are a hard contract: a short body makes the client's reader write
// past what arrived, and a long prefix makes it refuse the packet without
// reading anything at all.
func TestLegionInfoSizesAndChannelCode(t *testing.T) {
	if LegionInfoBodySize != 204 {
		t.Fatalf("body size %d, want 204 (sub_146EA0BE0(buf, 204))", LegionInfoBodySize)
	}
	p := LegionInfo(DefaultLegionInfo())
	if len(p) != LegionInfoSize || LegionInfoSize != 206 {
		t.Fatalf("payload %d bytes, want 206", len(p))
	}
	if got := binary.LittleEndian.Uint16(p); got != OperationChannelCode {
		t.Fatalf("channel code %d, want %d", got, OperationChannelCode)
	}
}

// Every offset the client's initializer sub_140698280 names is pinned here, so
// a later edit cannot silently move one. The expected bytes were read off that
// initializer (next75 §2) rather than copied from a capture, because the packet
// has never been sent.
func TestLegionInfoBodyLayoutMatchesInitializer(t *testing.T) {
	b := LegionInfoBody(DefaultLegionInfo())
	if len(b) != LegionInfoBodySize {
		t.Fatalf("body %d bytes", len(b))
	}
	get16 := func(off int) uint16 { return binary.LittleEndian.Uint16(b[off:]) }
	get32 := func(off int) uint32 { return binary.LittleEndian.Uint32(b[off:]) }
	get64 := func(off int) uint64 { return binary.LittleEndian.Uint64(b[off:]) }

	if got := get16(0); got != 0xFFFF {
		t.Fatalf("@0 = %#x, want 0xFFFF (-1)", got)
	}
	if b[2] != 0xFF {
		t.Fatalf("@2 = %#x, want 0xFF (-1)", b[2])
	}
	if got := get32(3); got != LegionInfoDefaultState {
		t.Fatalf("@3 = %d, want %d", got, LegionInfoDefaultState)
	}
	if got := get32(7); got != LegionInfoDefaultMode {
		t.Fatalf("@7 = %d, want %d", got, LegionInfoDefaultMode)
	}
	if got := get64(11); got != LegionInfoDefaultThird {
		t.Fatalf("@11 = %#x, want %#x (-1)", got, LegionInfoDefaultThird)
	}
	if b[19] != 0 {
		t.Fatalf("@19 = %#x, want 0", b[19])
	}
	if got := get16(21); got != 0 {
		t.Fatalf("@21 = %#x, want 0", got)
	}
	if got := get32(23); got != 0xFFFFFFFF {
		t.Fatalf("@23 = %#x, want 0xFFFFFFFF (-1)", got)
	}
	// 27..98 is six 12-byte records the initializer clears.
	for off := 27; off < 99; off++ {
		if b[off] != 0 {
			t.Fatalf("@%d = %#x inside the record area, want 0", off, b[off])
		}
	}
	if got := get32(99); got != 0xFFFFFFFF {
		t.Fatalf("@99 = %#x, want 0xFFFFFFFF (-1)", got)
	}
	if got := get32(103); got != 0 {
		t.Fatalf("@103 = %#x, want 0", got)
	}
	for off := 107; off < 123; off++ {
		if b[off] != 0 {
			t.Fatalf("@%d = %#x, want 0", off, b[off])
		}
	}
	if got := get32(123); got != 0x01010101 {
		t.Fatalf("@123 = %#x, want 0x01010101", got)
	}
	if b[127] != 0 {
		t.Fatalf("@127 = %#x, want 0", b[127])
	}
	for off := 128; off < LegionInfoBodySize; off++ {
		if b[off] != 0 {
			t.Fatalf("@%d = %#x past @127, want 0", off, b[off])
		}
	}
}

// The three extracted values have to land exactly where the handler reads them
// and nowhere else, or a packet built from real values would corrupt the fixed
// structure around them.
func TestLegionInfoOverridesOnlyTheThreeFields(t *testing.T) {
	base := LegionInfoBody(DefaultLegionInfo())
	// Each value has to differ from the default in EVERY byte it occupies,
	// otherwise a field that had quietly moved would still look unchanged.
	other := LegionInfoBody(LegionInfoValues{State: ^uint32(0), Mode: ^uint32(0), Third: 0})
	if len(base) != len(other) {
		t.Fatal("length changed with the values")
	}
	changed := []int{}
	for i := range base {
		if base[i] != other[i] {
			changed = append(changed, i)
		}
	}
	// @3..6 state, @7..10 mode, @11..18 third, and nothing else: 16 bytes.
	if len(changed) != 16 {
		t.Fatalf("changed bytes %v, want exactly 3..18", changed)
	}
	for i, off := range changed {
		if off != 3+i {
			t.Fatalf("changed bytes %v, want every byte of 3..18", changed)
		}
	}
	// The values must survive the round trip, byte order included.
	if got := binary.LittleEndian.Uint32(other[3:]); got != ^uint32(0) {
		t.Fatalf("@3 = %#x", got)
	}
	if got := binary.LittleEndian.Uint32(other[7:]); got != ^uint32(0) {
		t.Fatalf("@7 = %#x", got)
	}
	if got := binary.LittleEndian.Uint64(other[11:]); got != 0 {
		t.Fatalf("@11 = %#x", got)
	}
}

// DefaultLegionInfo must be the initializer's own triple, because that is what
// makes the packet state-preserving while the field semantics are unresolved.
func TestDefaultLegionInfoIsTheClientInitializer(t *testing.T) {
	d := DefaultLegionInfo()
	if d.State != 14 || d.Mode != 4 || d.Third != ^uint64(0) {
		t.Fatalf("defaults %+v, want {14 4 -1}", d)
	}
}
