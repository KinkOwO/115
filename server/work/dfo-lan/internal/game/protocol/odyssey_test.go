package protocol

import (
	"encoding/binary"
	"testing"
)

func TestOdysseyJournalFixedArray(t *testing.T) {
	for _, ids := range [][]uint32{nil, {100004934}, {100004934, 100004935}} {
		p, err := OdysseyCharacterProgress(ids)
		if err != nil || len(p) != 280 {
			t.Fatal(len(p), err)
		}
		for i := 0; i < 70; i++ {
			var want uint32
			if i < len(ids) {
				want = ids[i]
			}
			if got := binary.LittleEndian.Uint32(p[i*4:]); got != want {
				t.Fatalf("entry %d: got %d want %d", i, got, want)
			}
		}
	}
	for _, ids := range [][]uint32{{0}, {1, 1}, make([]uint32, 71)} {
		if _, err := OdysseyCharacterProgress(ids); err == nil {
			t.Fatal("invalid journal accepted", ids)
		}
	}
}
