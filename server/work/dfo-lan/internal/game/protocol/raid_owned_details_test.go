package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"strings"
	"testing"
)

func TestRaidIdentityMatchesNativeDetailGate(t *testing.T) {
	id, err := RaidTeamID(1, 82, 3)
	if err != nil {
		t.Fatal(err)
	}
	if byte(id>>16) != 82 || byte(id>>24) != 1 || uint16(id) != 3 {
		t.Fatalf("native channel/server gate rejects ID %08x", id)
	}
	for _, v := range [][3]uint32{{0, 82, 1}, {1, 256, 1}, {256, 82, 1}, {1, 82, 65536}, {1, 82, 0}} {
		if _, err := RaidTeamID(v[0], v[1], v[2]); err == nil {
			t.Fatalf("invalid identity accepted %v", v)
		}
	}
}

func TestRaidMemberInfoOfficialResponseLayout(t *testing.T) {
	fixture, err := os.ReadFile("testdata/raid_member_info_official_20261002.hex")
	if err != nil {
		t.Fatal(err)
	}
	official, err := hex.DecodeString(strings.TrimSpace(string(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := RaidTeamID(1, 79, 6)
	r := RaidRecruitment{ID: id, Leader: 84, LeaderName: "naiqiang", LeaderProfession: 5, LeaderAdvancement: 0x35, LeaderLevel: 115, MemberCount: 1}
	generated, err := RaidMemberInfoSuccess(r)
	if err != nil {
		t.Fatal(err)
	}
	consume := func(p []byte) int {
		t.Helper()
		at := 0
		read := func(n int) []byte {
			t.Helper()
			if n < 0 || at+n > len(p) {
				t.Fatalf("member info overrun %d+%d/%d", at, n, len(p))
			}
			v := p[at : at+n]
			at += n
			return v
		}
		str := func() string { n := binary.LittleEndian.Uint32(read(4)); return string(read(int(n))) }
		if read(1)[0] != 1 || read(1)[0] != 1 || binary.LittleEndian.Uint32(read(4)) != id || read(1)[0] != 1 {
			t.Fatal("success/mode/ID/count")
		}
		if binary.LittleEndian.Uint16(read(2)) != 84 {
			t.Fatal("actor")
		}
		read(1)
		if str() != "naiqiang" {
			t.Fatal("member name")
		}
		if read(1)[0] != 5 {
			t.Fatal("native separate profession")
		}
		if read(1)[0] != 0x35 || read(1)[0] != 0 {
			t.Fatal("native packed job/assignment")
		}
		read(1)
		read(4)
		if read(1)[0] != 255 || read(1)[0] != 115 {
			t.Fatal("packed job/level")
		}
		read(2)
		str()
		read(1)
		read(2)
		read(4)
		if binary.LittleEndian.Uint32(read(4)) != 0 {
			t.Fatal("optional map")
		}
		read(2)
		read(2)
		read(3)
		read(8)
		read(2)
		read(8)
		read(1)
		read(8)
		return at
	}
	if n := consume(generated); n != len(generated) {
		t.Fatalf("generated unread bytes %d", len(generated)-n)
	}
	// The capture decoder retains cryptographic trailer and block padding;
	// the application reader stops before those bytes.
	if n := consume(official); n >= len(official) || len(official)-n > 16 {
		t.Fatalf("official trailer boundary %d/%d", n, len(official))
	}
	if _, err := RaidMemberInfoSuccess(RaidRecruitment{}); err == nil {
		t.Fatal("invalid member info accepted")
	}
}

func TestRaidOwnedDetailNativeFieldsAndMemberConsumption(t *testing.T) {
	id, _ := RaidTeamID(1, 82, 1)
	r := RaidRecruitment{ID: id, Create: RaidCreateRequest{Kind: 8, Title: "1"}, Leader: 2, LeaderAdvancement: 0x31, LeaderLevel: 115, LeaderName: "Deemo", MemberCount: 1}
	p, err := RaidOwnedDetails(r)
	if err != nil {
		t.Fatal(err)
	}
	at := 0
	read := func(n int) []byte {
		t.Helper()
		if at+n > len(p) {
			t.Fatalf("native overrun %d/%d", at, n)
		}
		v := p[at : at+n]
		at += n
		return v
	}
	u32 := func() uint32 { return binary.LittleEndian.Uint32(read(4)) }
	str := func() string { return string(read(int(u32()))) }
	if u32() != id || u32() != 0 {
		t.Fatal("NOTI578 action-0 header")
	}
	if u32() != id || str() != "1" {
		t.Fatal("full recruitment ID/name")
	}
	read(3)
	read(4)
	read(9)
	read(4)
	read(1)
	read(4) // flags 0x0100: +138 is mandatory before +12c
	read(4)
	member := func() {
		t.Helper()
		if binary.LittleEndian.Uint16(read(2)) != 2 {
			t.Fatal("actor ID")
		}
		read(1)
		if str() != "Deemo" {
			t.Fatal("member name")
		}
		read(1)
		if read(1)[0] != 0x31 || read(1)[0] != 0 {
			t.Fatal("native packed job/assignment")
		}
		read(1)
		read(4)
		if read(1)[0] != 255 || read(1)[0] != 115 {
			t.Fatal("member status or level")
		}
		read(2)
		str()
		read(1)
		read(2)
		read(4)
		if u32() != 0 {
			t.Fatal("optional statistics count")
		}
		read(2)
		read(2)
		read(3)
		read(8)
		read(2)
		read(8)
		read(1)
		read(8)
	}
	member() // nested recruitment leader
	if read(1)[0] != 1 {
		t.Fatal("raid member vector is empty")
	}
	member() // 144cdc690 consumes the independent member vector
	if at != len(p) {
		t.Fatalf("native detail has %d unread bytes", len(p)-at)
	}
	r.MemberCount = 2
	if _, err := RaidOwnedDetails(r); err == nil {
		t.Fatal("inconsistent solo count accepted")
	}
}
