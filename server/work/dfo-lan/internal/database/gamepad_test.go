package database

import (
	"bytes"
	"testing"
)

func TestBuildGamepadPayload(t *testing.T) {
	// 1. Check default options and zero TSV
	p := BuildGamepadPayload(nil, nil)
	if len(p) != GamepadPayloadSize {
		t.Fatalf("expected payload size %d, got %d", GamepadPayloadSize, len(p))
	}
	if p[0] != 0x00 {
		t.Fatalf("expected offset 0 to be 0x00, got 0x%02x", p[0])
	}
	// Middle 1400 bytes should be zero
	for i := 1; i <= 1400; i++ {
		if p[i] != 0 {
			t.Fatalf("expected zero at offset %d, got %d", i, p[i])
		}
	}
	// Tail 10 bytes should match DefaultGamepadOptions
	if !bytes.Equal(p[1401:1411], DefaultGamepadOptions) {
		t.Fatalf("expected default options at 1401..1410, got %x", p[1401:1411])
	}

	// 2. Custom TSV and custom options
	tsv := []byte("K20\t141\t0\nI01\t140\t0\n")
	opts := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	p2 := BuildGamepadPayload(tsv, opts)
	if len(p2) != GamepadPayloadSize {
		t.Fatalf("expected payload size %d, got %d", GamepadPayloadSize, len(p2))
	}
	if p2[0] != 0x00 {
		t.Fatalf("expected offset 0 to be 0x00, got 0x%02x", p2[0])
	}
	if !bytes.Equal(p2[1:1+len(tsv)], tsv) {
		t.Fatalf("TSV content mismatch at offset 1..%d", 1+len(tsv))
	}
	if p2[1+len(tsv)] != 0x00 {
		t.Fatalf("expected trailing zero padding after TSV")
	}
	if !bytes.Equal(p2[1401:1411], opts) {
		t.Fatalf("expected custom options at 1401..1410, got %x", p2[1401:1411])
	}

	// 3. Overflow TSV truncate at 1400
	hugeTSV := bytes.Repeat([]byte("A"), 2000)
	p3 := BuildGamepadPayload(hugeTSV, nil)
	if len(p3) != GamepadPayloadSize {
		t.Fatalf("expected payload size %d, got %d", GamepadPayloadSize, len(p3))
	}
	if !bytes.Equal(p3[1:1401], bytes.Repeat([]byte("A"), 1400)) {
		t.Fatalf("expected truncated TSV of 1400 bytes")
	}
	if !bytes.Equal(p3[1401:1411], DefaultGamepadOptions) {
		t.Fatalf("expected default options fallback, got %x", p3[1401:1411])
	}
}
