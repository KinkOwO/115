package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestRaidEntrance115CapturedCreateAndOwnedRecord(t *testing.T) {
	request, _ := hex.DecodeString("08010000003100000007000000000000")
	r, err := DecodeRaidCreateRequest(request)
	if err != nil || r.Kind != 8 || r.Title != "1" || r.Options != [6]byte{0, 0, 0, 7, 0, 0} {
		t.Fatalf("native request decode: %+v %v", r, err)
	}
	id, err := RaidTeamID(1, 82, 1)
	if err != nil || id != 0x01520001 {
		t.Fatalf("team id=%x %v", id, err)
	}
	row := RaidRecruitment{ID: id, Create: r, Leader: 2, LeaderName: "Lansmt", LeaderAdvancement: 5, LeaderLevel: 115, MemberCount: 1, MemberPosition: 0, MemberArea: 2, MemberMax: 12}
	// Source raidLeaderRecord lines187..200, with the exact reserved tail
	// widths from lines201..213. Not a zero-filled replacement of the record.
	leader, _ := hex.DecodeString("020001060000004c616e736d740005000000000000ff73000000000000000200")
	leader = append(leader, make([]byte, 42)...)
	info, err := RaidMemberInfoSuccess(row)
	if err != nil || !bytes.Equal(info[7:], leader) || len(info) != 81 || binary.LittleEndian.Uint32(info[2:6]) != id {
		t.Fatalf("native member layout len=%d err=%v hex=%x", len(info), err, info)
	}
	owned, err := RaidOwnedDetails(row)
	if err != nil || len(owned) != 195 || !bytes.Equal(owned[len(owned)-len(leader):], leader) {
		t.Fatalf("owned recruitment len=%d err=%v", len(owned), err)
	}
	for _, identity := range [][3]uint32{{0, 82, 1}, {1, 256, 1}, {1, 82, 0}, {1, 82, 65536}} {
		if _, err := RaidTeamID(identity[0], identity[1], identity[2]); err == nil {
			t.Fatal("invalid identity accepted")
		}
	}
}

func TestRaidEntrance115OfficialNormalHeader(t *testing.T) {
	// 20261005-015111, 01:55:04: owned N578, title1234 and live leader.
	request, _ := hex.DecodeString("0804000000313233340000000700000000000000000000000000000000000000")
	r, err := DecodeRaidCreateRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	row := RaidRecruitment{ID: 0x035c0061, Create: r, Leader: 482, LeaderName: "moyius", LeaderProfession: 16, LeaderAdvancement: 53, LeaderLevel: 115, MemberCount: 1, MemberArea: 6, MemberMax: 12, LeaderFame: 55936}
	owned, err := RaidOwnedDetails(row)
	if err != nil {
		t.Fatal(err)
	}
	prefix, _ := hex.DecodeString("61005c030000000061005c0304000000313233340800ff0000000000000000000007000080da0000000c00000000000000e20101060000006d6f796975731035000000000000ff730000")
	if !bytes.HasPrefix(owned, prefix) {
		t.Fatalf("native header/leader misaligned:\nwant %x\ngot  %x", prefix, owned)
	}
	if owned[27] != 0 || binary.LittleEndian.Uint16(owned[49:]) != 482 || owned[51] != 1 {
		t.Fatal("normal raid became guide mode or lost its live leader")
	}
	row.MemberMax = 16
	changed, _ := RaidOwnedDetails(row)
	if binary.LittleEndian.Uint32(changed[41:]) != 16 {
		t.Fatal("source raid member limit ignored")
	}
}
