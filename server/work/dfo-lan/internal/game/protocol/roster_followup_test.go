package protocol

import "testing"

func TestMercenaryRequestBounds(t *testing.T) {
	for _, p := range [][]byte{nil, {8, 0, 0, 0, 0, 0, 0, 0}, {0, 1, 0, 0, 0, 0, 0, 0}} {
		if _, e := EmptyMercenaryInfo(p); e == nil {
			t.Fatalf("accepted malformed %x", p)
		}
	}
	p, e := EmptyMercenaryInfo([]byte{5, 0, 1, 2, 3, 4, 0, 0})
	if e != nil || len(p) != 2 || p[0] != 1 || p[1] != 0 {
		t.Fatalf("empty collection: %x %v", p, e)
	}
}
