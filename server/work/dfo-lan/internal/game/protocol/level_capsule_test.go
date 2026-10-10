package protocol

import (
	"encoding/hex"
	"testing"
)

func TestJumpLevelCapsuleNativeRequestAndRefusals(t *testing.T) {
	p, err := hex.DecodeString("42000000000000a00000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if slot, err := DecodeJumpLevelCapsule(p); err != nil || slot != 66 {
		t.Fatal(slot, err)
	}
	for _, tc := range []struct {
		offset int
		value  byte
	}{{0, 0}, {2, 1}, {7, 54}, {8, 1}, {11, 1}, {63, 1}} {
		q := append([]byte(nil), p...)
		q[tc.offset] = tc.value
		if _, err := DecodeJumpLevelCapsule(q); err == nil {
			t.Fatalf("accepted altered operand %d", tc.offset)
		}
	}
	if _, err := DecodeJumpLevelCapsule(p[:58]); err == nil {
		t.Fatal("accepted truncated native request")
	}
}
