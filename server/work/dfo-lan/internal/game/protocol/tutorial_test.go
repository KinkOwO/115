package protocol

import (
	"encoding/hex"
	"testing"
)

func TestCapturedTutorialChange(t *testing.T) {
	p, _ := hex.DecodeString("001f0000000100000000000000000000")
	r, e := DecodeTutorialChange(p)
	if e != nil || r.Index != 31 || !r.Completed {
		t.Fatalf("%+v %v", r, e)
	}
	for _, q := range [][]byte{p[:5], append([]byte{0, 101, 0, 0, 0, 1}, make([]byte, 10)...), append([]byte{0, 1, 0, 0, 0, 2}, make([]byte, 10)...), append(append([]byte{}, p...), 1)} {
		if _, e = DecodeTutorialChange(q); e == nil {
			t.Fatal("invalid tutorial request accepted")
		}
	}
}
