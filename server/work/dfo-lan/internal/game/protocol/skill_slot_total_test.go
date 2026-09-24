package protocol

import (
	"testing"
)

func frame(body []byte) []byte {
	return append(append([]byte{}, body...), 0xff, 0xff, 0xff, 0xff)
}

func TestDecodeSkillSlotTotal(t *testing.T) {
	// Captured 2026-09-23 08:35:04.697 (tree 0, 13 swaps), ffffffff tail.
	pairs := []byte{22, 0, 9, 1, 8, 2, 11, 3, 10, 4, 7, 5, 10, 7, 11, 8, 9, 26, 10, 28, 11, 29, 12, 30, 13, 31}
	p := append(append([]byte{0, byte(len(pairs) / 2)}, pairs...), 0xff, 0xff, 0xff, 0xff)
	r, e := DecodeSkillSlotTotal(p)
	if e != nil {
		t.Fatal(e)
	}
	if r.Tree != 0 || len(r.Pairs) != 13 || r.Pairs[0] != (SkillSlotSwap{22, 0}) || r.Pairs[12] != (SkillSlotSwap{13, 31}) {
		t.Fatalf("decoded %+v", r)
	}

	// Tree 0xff means selected tree zero.
	r, e = DecodeSkillSlotTotal([]byte{255, 1, 5, 6, 0xff, 0xff, 0xff, 0xff})
	if e != nil || r.Tree != 0 || len(r.Pairs) != 1 {
		t.Fatalf("tree sentinel: %+v %v", r, e)
	}

	bad := [][]byte{
		{0},                                  // short
		{0, 0, 0xff, 0xff, 0xff, 0xff, 0xff}, // zero count
		{0, 1, 5, 6},                         // missing tail
		{0, 1, 5, 6, 0x00, 0x00, 0x00, 0x00}, // wrong tail
		{1, 1, 5, 6, 0xff, 0xff, 0xff, 0xff}, // non-zero tree
		{0, 1, 5, 5, 0xff, 0xff, 0xff, 0xff}, // source == target
		{0, 1, 255, 6, 0xff, 0xff, 0xff, 0xff},
		{0, 1, 5, 255, 0xff, 0xff, 0xff, 0xff},
	}
	for _, b := range bad {
		if _, e = DecodeSkillSlotTotal(b); e == nil {
			t.Fatalf("accepted invalid body %x", b)
		}
	}
}
