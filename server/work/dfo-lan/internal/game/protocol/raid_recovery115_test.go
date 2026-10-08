package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestRaidRecoveryDeadlineMatchesNativeDeathCampVector(t *testing.T) {
	// Official N578 20261005-015111: 1791137134825ms, first camp recovery.
	native, _ := hex.DecodeString("61005c030300000001e20101060000006d6f79697573103501010000000000730b00060000006d6f79696e670002007d95c26a")
	if binary.LittleEndian.Uint32(native[47:]) != 1791137149 {
		t.Fatal("wrong native recovery deadline")
	}
	r := RaidRecruitment{ID: 0x035c0061, Create: RaidCreateRequest{Title: "1234"}, Leader: 482, LeaderName: "moyius", LeaderProfession: 0x10, LeaderAdvancement: 0x35, LeaderLevel: 115, MemberCount: 1, MemberPosition: 1, MemberArea: 2, MemberMax: 12, MemberRecoveryUntil: 1791137149}
	body, e := RaidAssignmentUpdate(r)
	if e != nil {
		t.Fatal(e)
	}
	// The captured row has a six-byte adventure name; this owned row's
	// adventure name is empty. Its deadline is exactly six bytes earlier.
	if binary.LittleEndian.Uint32(body[41:]) != binary.LittleEndian.Uint32(native[47:]) {
		t.Fatal("deadline was not encoded at member+64 field")
	}
	r.MemberRecoveryUntil = 0
	empty, e := RaidAssignmentUpdate(r)
	if e != nil {
		t.Fatal(e)
	}
	if len(empty) != len(body) {
		t.Fatal("recovery changed native row width")
	}
	for i := range body {
		if i < 41 || i >= 45 {
			if body[i] != empty[i] {
				t.Fatal("recovery changed unrelated member fields")
			}
		}
	}
}
