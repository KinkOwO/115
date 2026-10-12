package protocol

import (
	"encoding/hex"
	"testing"
)

func TestTownArrivalMotionPositionPrefix(t *testing.T) {
	for _, raw := range []string{"2e025c01056400000000000000000000", "2e025c01086400000000000000000000"} {
		p, _ := hex.DecodeString(raw)
		got, err := DecodeUserStateMotionRequest(p)
		if err != nil || got.X != 558 || got.Y != 348 || got.Speed != 100 || got.Motion != p[4] {
			t.Fatalf("motion %s: %+v %v", raw, got, err)
		}
		if _, err := DecodeUserStateMotionRequest(p[:12]); err == nil {
			t.Fatal("short prefix accepted")
		}
		p[10] = 1
		if _, err := DecodeUserStateMotionRequest(p); err == nil {
			t.Fatal("reserved action bytes accepted")
		}
	}
}
