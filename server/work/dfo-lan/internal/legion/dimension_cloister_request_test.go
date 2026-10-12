package legion

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestDimCloisterOperationUsesNativeEnvelopeBoundary(t *testing.T) {
	for _, tc := range []struct {
		body   string
		action uint32
		index  int16
	}{
		{"90aefd9401000000010000000101000000ffff0000000000", 1, -1},
		{"000000000000000006000000000200000007000000000000", 2, 7},
	} {
		body, _ := hex.DecodeString(tc.body)
		body[4], body[5] = 0xab, 0xcd // opaque envelope cannot become action/close
		r, err := DecodeDimCloisterOperation(body)
		if err != nil || r.Action != tc.action || r.Index != tc.index || r.Close {
			t.Fatalf("native C2080 decoded as %+v: %v", r, err)
		}
		if _, err = DecodeDimCloisterOperation(body[:18]); err == nil {
			t.Fatal("truncated operation accepted")
		}
	}
}

func TestCloisterNativeEntryNonzeroPhases(t *testing.T) {
	for _, phase := range []uint32{0, 1, 2, 0x01020304} {
		p := make([]byte, 21)
		binary.LittleEndian.PutUint32(p[13:], uint32(DimCloisterPartyType))
		binary.LittleEndian.PutUint32(p[17:], phase)
		_, got, err := DecodeDimCloisterStageConfirm(p)
		if err != nil || got != phase {
			t.Fatal(phase, got, err)
		}
		p[16] = 1
		if _, _, err = DecodeDimCloisterStageConfirm(p); err == nil {
			t.Fatal("truncated foreign content accepted")
		}
	}
	if _, _, err := DecodeDimCloisterStageConfirm(make([]byte, 20)); err == nil {
		t.Fatal("truncated entry accepted")
	}
}
