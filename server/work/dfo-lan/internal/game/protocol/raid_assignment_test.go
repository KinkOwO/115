package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestRaidAssignmentActualClientRequests(t *testing.T) {
	// User supplied 20261002 capture, CMD661 operation 0/actor84/positions.
	for _, position := range []uint32{0, 1, 3, 4, 6, 8, 7, 5} {
		p, _ := hex.DecodeString("00000000540000000100000000000000")
		binary.LittleEndian.PutUint32(p[8:], position)
		actor, got, err := DecodeRaidAssignment(p)
		if err != nil || actor != 84 || got != position {
			t.Fatalf("capture %x => %d/%d/%v", p, actor, got, err)
		}
		if _, _, err := DecodeRaidAssignment(p[:11]); err == nil {
			t.Fatal("truncation")
		}
		p[15] = 1
		if _, _, err := DecodeRaidAssignment(p); err == nil {
			t.Fatal("nonzero padding")
		}
	}
	for _, text := range []string{"010000005400000001000000", "000000000000000001000000", "000000000000010001000000"} {
		p, _ := hex.DecodeString(text)
		if _, _, err := DecodeRaidAssignment(p); err == nil {
			t.Fatal("unsafe manager request", text)
		}
	}
	for _, flag := range []byte{0, 1} {
		p := make([]byte, 8)
		p[0] = flag
		enabled, err := DecodeRaidUpdateControl(p)
		if err != nil || enabled != (flag == 1) {
			t.Fatal(enabled, err)
		}
	}
	for _, p := range [][]byte{nil, {2}, {1, 1}, {1, 0, 0, 0}} {
		if _, err := DecodeRaidUpdateControl(p); err == nil {
			t.Fatal("invalid update toggle accepted", p)
		}
	}
}

func TestRaidQueryModesPreserveMemberAssignment(t *testing.T) {
	id, _ := RaidTeamID(1, 82, 1)
	r := RaidRecruitment{ID: id, Create: RaidCreateRequest{Kind: 8, Title: "test"}, Leader: 9, LeaderName: "Deemo", LeaderProfession: 16, LeaderAdvancement: 0x33, LeaderLevel: 115, MemberCount: 1, MemberPosition: 8, MemberArea: 2}
	member, err := RaidMemberInfoSuccess(r)
	if err != nil {
		t.Fatal(err)
	}
	// Application offsets projected into native +2c (job), +2d (assignment),
	// +35 (level), corroborated by the supplied official member capture.
	if member[19] != 16 || member[20] != 0x33 || member[21] != 8 || member[28] != 115 {
		t.Fatalf("native member projection %x", member)
	}
	// Independent native reader boundary: header7, actor2, slot1,
	// name length4+5, job/status4, counter4, flags/level4,
	// empty guild length4, state1, then u16 area -> native member+62.
	if binary.LittleEndian.Uint16(member[36:38]) != 2 {
		t.Fatalf("native waiting area missing: %x", member)
	}
	full, err := RaidFullInfoSuccess(r)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := RaidOwnedDetails(r)
	if err != nil {
		t.Fatal(err)
	}
	if full[0] != 1 || full[1] != 2 || binary.LittleEndian.Uint32(full[2:]) != id || !bytes.Equal(full[6:], detail[8:]) {
		t.Fatal("mode2 full snapshot")
	}
	update, err := RaidAssignmentUpdate(r)
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint32(update) != id || binary.LittleEndian.Uint32(update[4:]) != 3 || !bytes.Equal(update[8:len(update)-4], member[6:]) {
		t.Fatal("action3 member vector")
	}
	if !bytes.Equal(RaidAssignmentSuccess(), []byte{1, 0, 0, 0, 0}) {
		t.Fatal("manager acknowledgement")
	}
}
