package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestRaidCreateNativeSenderFields(t *testing.T) {
	p := []byte{8}
	p = addName(p, "测试队伍")
	p = append(p, 1, 2, 3, 4, 5, 0)
	p = add32(p, 15951)
	r, err := DecodeRaidCreateRequest(p)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != 8 || r.Title != "测试队伍" || r.Options != [6]byte{1, 2, 3, 4, 5, 0} || r.Limit != 15951 {
		t.Fatalf("fields shifted: %+v", r)
	}
	for _, bad := range [][]byte{p[:len(p)-1], append(append([]byte(nil), p...), 1), p[:4]} {
		if _, err := DecodeRaidCreateRequest(bad); err == nil {
			t.Fatal("malformed request accepted")
		}
	}
	if b := RaidCreateSuccess(0x12345678); len(b) != 5 || b[0] != 1 || binary.LittleEndian.Uint32(b[1:]) != 0x12345678 {
		t.Fatal("wrong create reply")
	}
	kind, ok := RaidKindForChannel(82)
	if !ok || kind != 8 {
		t.Fatal("channel ID was confused with raid kind")
	}
	if _, ok := RaidKindForChannel(81); ok {
		t.Fatal("legion treated as raid")
	}
	if b := RaidLeaveSuccess(); len(b) != 2 || b[0] != 1 || b[1] != 0 {
		t.Fatal("leave success omitted the native normal/penalty byte")
	}
}

func TestRaidCreateActualClientBodies20261004(t *testing.T) {
	for _, v := range []struct {
		raw, title string
		kind       byte
	}{
		{"08050000004465656d6f00000007000000000000000000000000000000000000", "Deemo", 8},
		{"08010000003100000007000000000000", "1", 8},
		{"0d010000003100000007000000000000", "1", 13},
	} {
		p, err := hex.DecodeString(v.raw)
		if err != nil {
			t.Fatal(err)
		}
		r, err := DecodeRaidCreateRequest(p)
		if err != nil {
			t.Fatalf("actual CMD656 %s: %v", v.raw, err)
		}
		if r.Kind != v.kind || r.Title != v.title || r.Options != [6]byte{0, 0, 0, 7, 0, 0} || r.Limit != 0 {
			t.Fatalf("native fields shifted: %+v", r)
		}
		bad := append([]byte(nil), p...)
		binary.LittleEndian.PutUint32(bad[1:], ^uint32(0))
		if _, err = DecodeRaidCreateRequest(bad); err == nil {
			t.Fatal("overflow length accepted")
		}
	}
	p, _ := hex.DecodeString("08050000004465656d6f00000007000000000000000000000000000000000000")
	p[len(p)-1] = 1
	if _, err := DecodeRaidCreateRequest(p); err == nil {
		t.Fatal("nonzero block padding accepted")
	}
}

func TestRaidInfoAndLeaveBodyOnlyPadding(t *testing.T) {
	p := make([]byte, 16)
	p[0] = 1
	binary.LittleEndian.PutUint32(p[1:], 23)
	mode, id, err := DecodeRaidInfoRequest(p)
	if err != nil || mode != 1 || id != 23 {
		t.Fatalf("info fields %d/%d: %v", mode, id, err)
	}
	p = make([]byte, 16)
	binary.LittleEndian.PutUint16(p, 82)
	ch, err := DecodeRaidLeaveRequest(p)
	if err != nil || ch != 82 {
		t.Fatalf("leave fields %d: %v", ch, err)
	}
	p[2] = 1
	if _, err = DecodeRaidLeaveRequest(p); err == nil {
		t.Fatal("nonzero leave tail accepted")
	}
}

// Independently consume the native reader's exact widths, including both
// variable byte strings, optional map count and all trailing leader fields.
func TestRaidRecruitmentReaderConsumesEntireSnapshot(t *testing.T) {
	r := RaidRecruitment{ID: 7, Create: RaidCreateRequest{Kind: 8, Title: "队伍", Limit: 15951}, Leader: 9, LeaderName: "角色", LeaderAdvancement: 3, MemberCount: 1}
	p, err := RaidRecruitmentList([]RaidRecruitment{r})
	if err != nil {
		t.Fatal(err)
	}
	at := 0
	take := func(n int) []byte {
		t.Helper()
		if at+n > len(p) {
			t.Fatalf("reader overrun at %d width %d total %d", at, n, len(p))
		}
		b := p[at : at+n]
		at += n
		return b
	}
	str := func() { n := binary.LittleEndian.Uint32(take(4)); take(int(n)) }
	if binary.LittleEndian.Uint32(take(4)) != 1 {
		t.Fatal("count")
	}
	take(4)
	str()
	take(3)
	take(4)
	take(9)
	take(4)
	take(1)
	take(4)
	take(2)
	take(1)
	str()
	take(4)
	take(4)
	take(4)
	str()
	take(1)
	take(2)
	take(4)
	if binary.LittleEndian.Uint32(take(4)) != 0 {
		t.Fatal("optional map count")
	}
	take(2)
	take(2)
	take(3)
	take(8)
	take(2)
	take(8)
	take(1)
	take(8)
	if take(1)[0] != 1 {
		t.Fatal("presence")
	}
	if at != len(p) {
		t.Fatalf("unconsumed snapshot bytes %d", len(p)-at)
	}
}
