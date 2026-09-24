package protocol

import "testing"

func TestInformNoticeSeenPayload(t *testing.T) {
	if p := InformNoticeSeen(nil); len(p) != 1 || p[0] != 0 {
		t.Fatalf("empty set = %v", p)
	}
	p := InformNoticeSeen([]uint16{62, 1, 255})
	if len(p) != 4 || p[0] != 3 || p[1] != 62 || p[2] != 1 || p[3] != 255 {
		t.Fatalf("set = %v", p)
	}
	ids := make([]uint16, 300)
	for i := range ids {
		ids[i] = uint16(i)
	}
	if p = InformNoticeSeen(ids); len(p) != 256 || p[0] != 255 {
		t.Fatalf("truncated set length = %d", len(p))
	}
}
