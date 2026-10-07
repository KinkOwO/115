package protocol

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// CMD656 sender 145070527..145070664: u8 kind, byte string, six
// option bytes, u32 limit. Dispatcher plaintext is the decrypted BODY ONLY;
// its cipher's zero padding is retained. Native CMD656 vectors are recorded
// in raid_entrance_test.go (2026-10-04 user's create attempts).
type RaidCreateRequest struct {
	Kind    byte
	Title   string
	Options [6]byte
	Limit   uint32
}

func DecodeRaidCreateRequest(p []byte) (RaidCreateRequest, error) {
	var r RaidCreateRequest
	if len(p) < 16 {
		return r, fmt.Errorf("short raid creation")
	}
	b := p
	n := uint64(binary.LittleEndian.Uint32(b[1:5]))
	if n == 0 || n > 255 || n+15 > uint64(len(b)) {
		return r, fmt.Errorf("invalid raid title length or trailing fields")
	}
	if err := raidBodySize(b, int(n)+15); err != nil {
		return r, err
	}
	r.Kind = b[0]
	r.Title = string(b[5 : 5+int(n)])
	if !utf8.ValidString(r.Title) || strings.TrimSpace(r.Title) == "" {
		return r, fmt.Errorf("invalid raid title")
	}
	for _, ch := range r.Title {
		if unicode.IsControl(ch) {
			return r, fmt.Errorf("control character in raid title")
		}
	}
	copy(r.Options[:], b[5+int(n):11+int(n)])
	r.Limit = binary.LittleEndian.Uint32(b[11+int(n):])
	return r, nil
}

// Existing wire.DecryptPayload intentionally preserves zero block padding.
// Accept exactly the logical body or its 8/16-byte aligned form; never scan
// for a magic header or accept arbitrary/nonzero trailing fields.
func raidBodySize(p []byte, size int) error {
	if len(p) < size || (len(p) != size && len(p) != (size+7)/8*8 && len(p) != (size+15)/16*16) {
		return fmt.Errorf("invalid raid body length %d for %d fields", len(p), size)
	}
	for _, v := range p[size:] {
		if v != 0 {
			return fmt.Errorf("nonzero raid padding")
		}
	}
	return nil
}

func DecodeRaidInfoRequest(p []byte) (byte, uint32, error) {
	if err := raidBodySize(p, 5); err != nil {
		return 0, 0, err
	}
	return p[0], binary.LittleEndian.Uint32(p[1:5]), nil
}
func DecodeRaidLeaveRequest(p []byte) (uint16, error) {
	if err := raidBodySize(p, 2); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(p[:2]), nil
}

// Native channel-to-raid-kind switch at 144cf0e70. These are protocol IDs,
// not mutable gameplay rules and not the F7 channel ID.
func RaidKindForChannel(channel uint32) (byte, bool) {
	switch channel {
	case 23, 25:
		return 0, true
	case 45:
		return 2, true
	case 67, 68:
		return 6, true
	case 76, 78:
		return 7, true
	case 82, 83:
		return 8, true
	case 85:
		return 9, true
	case 86:
		return 10, true
	case 93:
		return 11, true
	case 98, 107:
		return 12, true
	case 111:
		return 13, true
	case 120:
		return 14, true
	}
	return 0, false
}

type RaidRecruitment struct {
	State             byte
	ID                uint32
	Create            RaidCreateRequest
	Leader            uint16
	LeaderAdvancement byte
	LeaderProfession  byte
	LeaderName        string
	MemberCount       byte
	LeaderLevel       byte
	MemberPosition    byte
	MemberArea        uint16
}

// NOTI585 reads a u32 record count, then u16 actor / u8 state / u16
// cost-count for each record. Even an empty vector completes the native
// pending query; omitting this notification leaves the raid UI locked.
func RaidEmptyEntryCostInfo() []byte { return add32(nil, 0) }

// NOTI578 compares byte 2 with current channel (14e682a60), byte 3
// with current server (14e682a58). NOTI2435 setter 1452c9140 confirms
// those identities; a plain incrementing number is discarded by this reader.
func RaidTeamID(server, channel, sequence uint32) (uint32, error) {
	if server == 0 || server > 255 || channel == 0 || channel > 255 || sequence == 0 || sequence > 65535 {
		return 0, fmt.Errorf("raid identity does not fit native fields")
	}
	return server<<24 | channel<<16 | sequence, nil
}

// NOTI577 reader 144ce26b0: u32 count, 144cdbf80(flags=0), u8
// presence. The nested leader reader is 144cdc190. Empty optional counters
// are explicit; no member/phase/boss state is invented by this snapshot.
func RaidRecruitmentList(entries []RaidRecruitment) ([]byte, error) {
	if len(entries) > 128 {
		return nil, fmt.Errorf("raid list too large")
	}
	p := add32(nil, uint32(len(entries)))
	for _, r := range entries {
		var err error
		p, err = raidRecruitmentRecord(p, r, false)
		if err != nil {
			return nil, err
		}
		p = append(p, 1)
	}
	return p, nil
}

func raidRecruitmentRecord(p []byte, r RaidRecruitment, details bool) ([]byte, error) {
	if r.ID == 0 || r.Leader == 0 || r.MemberCount == 0 || r.LeaderName == "" {
		return nil, fmt.Errorf("invalid recruitment")
	}
	p = add32(p, r.ID)
	p = addName(p, r.Create.Title)
	state := r.Create.Options[0]
	if r.State != 0 {
		state = r.State
	}
	p = append(p, r.Create.Kind, state, r.Create.Options[1])
	p = add32(p, uint32(r.Leader))
	// Native outer +120,+122,+123,+124..129, +130,+134,+12c.
	p = append(p, r.MemberCount, r.Create.Options[2], r.Create.Options[3], r.Create.Options[4], 0, 0, 0, r.Create.Options[5], 0)
	p = add32(p, r.Create.Limit)
	p = append(p, 0)
	if details {
		p = add32(p, 0)
	} // flags=0x0100 additionally consumes +138
	p = add32(p, 0)
	party := byte(255)
	if details {
		party = r.MemberPosition
	}
	return raidLeaderRecord(p, r, party), nil
}

func raidLeaderRecord(p []byte, r RaidRecruitment, position byte) []byte {
	// Leader: actor/header/name, one signed metadata byte, packed job,
	// assignment and status, counter, another status byte, level and flags.
	// around one u32, followed by guild string / guild flags and counters.
	p = add16(p, r.Leader)
	p = append(p, 0) // member slot/index; not profession
	p = addName(p, r.LeaderName)
	// 144cdc690 projects +28 to profession, +2c to growtype/awakening,
	// +2d is the
	// assignment tested by the native CMD661 sender 14506b890.
	p = append(p, r.LeaderProfession, r.LeaderAdvancement, position, 0)
	p = add32(p, 0)
	p = append(p, 255, r.LeaderLevel, 0, 0)
	p = addName(p, "")
	p = append(p, 0)
	// Native 144cdc318 reads this into member +62. Start admission
	// 144cf4f04 compares it with the source WAITING ROOM area index.
	p = add16(p, r.MemberArea)
	p = add32(p, 0)
	p = add32(p, 0) // no optional per-member statistic map
	p = append(p, 0, 0)
	p = add16(p, 0)
	p = append(p, 0, 0, 0)
	p = add32(p, 0)
	p = add32(p, 0)
	p = append(p, 0, 0)
	p = add32(p, 0)
	p = add32(p, 0)
	p = append(p, 0)
	p = add32(p, 0)
	p = add32(p, 0)
	return p
}

// CMD1353 mode 1 is answered by NOTI578 action 0: full recruitment
// (144cdbf80 flags=0x0100) followed by u8 member count and one 144cdc190
// record per member (144cdc690). Both branches then refresh raid widgets.
// This single-player candidate owns exactly its creator as red-party slot 0.
func RaidOwnedDetails(r RaidRecruitment) ([]byte, error) {
	if r.MemberCount != 1 {
		return nil, fmt.Errorf("owned solo detail requires one member")
	}
	p := add32(add32(nil, r.ID), 0)
	var err error
	p, err = raidRecruitmentRecord(p, r, true)
	if err != nil {
		return nil, err
	}
	p = append(p, 1)
	p = raidLeaderRecord(p, r, r.MemberPosition)
	return p, nil
}

// CMD1353 success callback 1452579d0 reads mode, raid ID and, for
// mode 1, the 144cdc690 member vector. Captured official kind=1/1353
// responses confirm success=1, mode=1, ID, count=1 and member record.
func RaidMemberInfoSuccess(r RaidRecruitment) ([]byte, error) {
	if r.ID == 0 || r.Leader == 0 || r.LeaderName == "" || r.MemberCount != 1 {
		return nil, fmt.Errorf("invalid solo raid member info")
	}
	p := add32([]byte{1, 1}, r.ID)
	p = append(p, 1)
	return raidLeaderRecord(p, r, r.MemberPosition), nil
}

// CMD1353 mode 2 consumes complete metadata followed by the member vector,
// confirmed by the success callback 1452579d0 (145257a3f branch).
func RaidFullInfoSuccess(r RaidRecruitment) ([]byte, error) {
	if r.MemberCount != 1 {
		return nil, fmt.Errorf("full info requires one member")
	}
	p := add32([]byte{1, 2}, r.ID)
	var err error
	p, err = raidRecruitmentRecord(p, r, true)
	if err != nil {
		return nil, err
	}
	p = append(p, 1)
	return raidLeaderRecord(p, r, r.MemberPosition), nil
}

func DecodeRaidAssignment(p []byte) (actor uint16, position uint32, err error) {
	if raidBodySize(p, 12) != nil || binary.LittleEndian.Uint32(p[:4]) != 0 {
		return 0, 0, fmt.Errorf("unsupported raid manager body")
	}
	actor32 := binary.LittleEndian.Uint32(p[4:8])
	if actor32 == 0 || actor32 > 65535 {
		return 0, 0, fmt.Errorf("invalid member actor")
	}
	return uint16(actor32), binary.LittleEndian.Uint32(p[8:12]), nil
}

// NOTI578 action 3 updates the existing member vector and its trailing counter.
func RaidAssignmentUpdate(r RaidRecruitment) ([]byte, error) {
	if r.ID == 0 || r.Leader == 0 || r.MemberCount != 1 {
		return nil, fmt.Errorf("invalid assignment")
	}
	p := add32(add32(nil, r.ID), 3)
	p = append(p, 1)
	p = raidLeaderRecord(p, r, r.MemberPosition)
	return add32(p, 0), nil
}

func RaidAssignmentSuccess() []byte { return add32([]byte{1}, 0) }

func DecodeRaidUpdateControl(p []byte) (bool, error) {
	if raidBodySize(p, 1) != nil || p[0] > 1 {
		return false, fmt.Errorf("invalid raid update control")
	}
	return p[0] == 1, nil
}

func RaidCreateSuccess(id uint32) []byte { return add32([]byte{1}, id) }

// CMD657 callback 144cda220 consumes u8 normal/penalty after success.
func RaidLeaveSuccess() []byte { return []byte{1, 0} }
