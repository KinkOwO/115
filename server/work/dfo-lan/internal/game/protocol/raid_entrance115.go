package protocol

import (
	"encoding/binary"
	"fmt"
	"unicode/utf8"
)

// Current-client raid entrance contract, recovered from the handover
// raid_entrance.go:19/156/183/221/239/250 and its DWARF field offsets.
type RaidCreateRequest struct {
	Kind    byte
	Title   string
	Options [6]byte
	Limit   uint32
}
type RaidRecruitment struct {
	State                                    byte
	ID                                       uint32
	Create                                   RaidCreateRequest
	Leader                                   uint16
	LeaderAdvancement, LeaderProfession      byte
	LeaderName                               string
	MemberCount, LeaderLevel, MemberPosition byte
	MemberArea                               uint16
	MemberRecoveryUntil                      uint32
	ActiveStartedAt                          uint32
	MemberMax, LeaderFame                    uint32
}

func DecodeRaidCreateRequest(p []byte) (RaidCreateRequest, error) {
	var r RaidCreateRequest
	if len(p) < 5 {
		return r, fmt.Errorf("short native raid create request")
	}
	n := uint64(binary.LittleEndian.Uint32(p[1:5]))
	end := uint64(5) + n
	if n == 0 || n > 255 || end+10 > uint64(len(p)) {
		return r, fmt.Errorf("invalid native raid title")
	}
	r.Kind = p[0]
	r.Title = string(p[5:end])
	if !utf8.ValidString(r.Title) {
		return r, fmt.Errorf("invalid native raid title encoding")
	}
	copy(r.Options[:], p[end:end+6])
	r.Limit = binary.LittleEndian.Uint32(p[end+6 : end+10])
	for _, v := range p[end+10:] {
		if v != 0 {
			return r, fmt.Errorf("nonzero native raid create padding")
		}
	}
	return r, nil
}
func RaidTeamID(server, channel, sequence uint32) (uint32, error) {
	if server == 0 || server > 255 || channel == 0 || channel > 255 || sequence == 0 || sequence > 65535 {
		return 0, fmt.Errorf("invalid native raid team identity")
	}
	return server<<24 | channel<<16 | sequence, nil
}
func DecodeRaidInfoRequest(p []byte) (byte, uint32, error) {
	if len(p) < 5 {
		return 0, 0, fmt.Errorf("short native raid info request")
	}
	for _, v := range p[5:] {
		if v != 0 {
			return 0, 0, fmt.Errorf("nonzero native raid info padding")
		}
	}
	return p[0], binary.LittleEndian.Uint32(p[1:5]), nil
}

// DecodeRaidUpdateControl is native CMD2121. Handover raid_entrance.go:288
// accepts the one-byte control, with zero padding to an 8/16-byte boundary.
func DecodeRaidUpdateControl(p []byte) (bool, error) {
	if (len(p) != 1 && len(p) != 8 && len(p) != 16) || p[0] > 1 {
		return false, fmt.Errorf("invalid native raid update control")
	}
	for _, v := range p[1:] {
		if v != 0 {
			return false, fmt.Errorf("nonzero native raid update control padding")
		}
	}
	return p[0] == 1, nil
}
func raidLeaderRecord115(p []byte, r RaidRecruitment, slot byte) []byte {
	p = binary.LittleEndian.AppendUint16(p, r.Leader)
	p = append(p, 1) // live member discriminator, captured 1353/N578
	p = binary.LittleEndian.AppendUint32(p, uint32(len(r.LeaderName)))
	p = append(p, r.LeaderName...)
	p = append(p, r.LeaderProfession, r.LeaderAdvancement, slot, 0)
	p = binary.LittleEndian.AppendUint32(p, 0)
	p = append(p, 0xff, r.LeaderLevel, 0, 0)
	p = binary.LittleEndian.AppendUint32(p, 0)
	p = append(p, 0)
	// Reader144CDC190 -> member+62; start predicate144CF4C80 compares this
	// u16 with the raid script's waiting-room AREA. Official wire: the B-layer
	// Bakal start burst (12:29:29 N578) carries 2 while the leader stands in
	// town 152 area 2, and the live channel-92 raid (2026-10-05 01:55:15 N578)
	// carries 6 — the per-raid waiting area, never the town id. A town id here
	// makes dstr101037245 ("party members are not all in the same area") fire
	// and the client never sends CMD2089.
	p = binary.LittleEndian.AppendUint16(p, r.MemberArea)
	// Reader144CDC190 -> member+64. Native death-to-camp N578 stores the
	// recovery deadline here (first +15s, next +30s), not an unused zero.
	p = binary.LittleEndian.AppendUint32(p, r.MemberRecoveryUntil)
	// Source lines201..213: two u32, two u16, u8/u16, two u32,
	// u16, two u32, u8, and two u32 reserved fields.
	return append(p, make([]byte, 38)...)
}
func raidRecruitmentRecord115(p []byte, r RaidRecruitment) []byte {
	p = binary.LittleEndian.AppendUint32(p, r.ID)
	p = binary.LittleEndian.AppendUint32(p, uint32(len(r.Create.Title)))
	p = append(p, r.Create.Title...)
	// Native reader 144CDBF80: normal mode flag is zero, not member count.
	// Extended N578/1353 headers must include the u8/u32 fields before the
	// leader record; omitting them shifts the client's leader-ID read.
	phase := byte(0xff)
	if r.State != 0 {
		phase = 0
	}
	p = append(p, r.Create.Kind, r.State, phase)
	p = binary.LittleEndian.AppendUint32(p, r.ActiveStartedAt)
	p = append(p, 0, r.Create.Options[0], r.Create.Options[1], r.Create.Options[2], 0, 0, r.Create.Options[3], r.Create.Options[4], r.Create.Options[5])
	p = binary.LittleEndian.AppendUint32(p, r.LeaderFame)
	p = append(p, 0)
	p = binary.LittleEndian.AppendUint32(p, r.MemberMax)
	p = binary.LittleEndian.AppendUint32(p, 0)
	return raidLeaderRecord115(p, r, r.MemberPosition)
}
func validateRaidRecruitment115(r RaidRecruitment) error {
	if r.ID == 0 || r.Leader == 0 || r.Leader == 65535 || r.MemberCount != 1 || r.MemberMax == 0 || r.Create.Title == "" || r.LeaderName == "" || len(r.Create.Title) > 255 || len(r.LeaderName) > 255 {
		return fmt.Errorf("invalid owned solo raid recruitment")
	}
	return nil
}
func RaidOwnedDetails(r RaidRecruitment) ([]byte, error) {
	if err := validateRaidRecruitment115(r); err != nil {
		return nil, err
	}
	p := binary.LittleEndian.AppendUint32(nil, r.ID)
	p = binary.LittleEndian.AppendUint32(p, 0)
	p = raidRecruitmentRecord115(p, r)
	p = append(p, 1)
	return raidLeaderRecord115(p, r, r.MemberPosition), nil
}
func RaidMemberInfoSuccess(r RaidRecruitment) ([]byte, error) {
	if err := validateRaidRecruitment115(r); err != nil {
		return nil, err
	}
	p := binary.LittleEndian.AppendUint32([]byte{1, 1}, r.ID)
	p = append(p, 1)
	return raidLeaderRecord115(p, r, r.MemberPosition), nil
}
func RaidFullInfoSuccess(r RaidRecruitment) ([]byte, error) {
	if err := validateRaidRecruitment115(r); err != nil {
		return nil, err
	}
	p := binary.LittleEndian.AppendUint32([]byte{1, 2}, r.ID)
	p = raidRecruitmentRecord115(p, r)
	p = append(p, 1)
	return raidLeaderRecord115(p, r, r.MemberPosition), nil
}

// CMD661 action0: actor u32 at4, formation position u32 at8.
func DecodeRaidAssignment(p []byte) (uint16, uint32, error) {
	if len(p) != 12 && len(p) != 16 {
		return 0, 0, fmt.Errorf("invalid native raid assignment length")
	}
	if binary.LittleEndian.Uint32(p) != 0 {
		return 0, 0, fmt.Errorf("unsupported native raid manager action")
	}
	for _, v := range p[12:] {
		if v != 0 {
			return 0, 0, fmt.Errorf("nonzero native raid assignment padding")
		}
	}
	actor := binary.LittleEndian.Uint32(p[4:])
	if actor == 0 || actor >= 65535 {
		return 0, 0, fmt.Errorf("invalid native raid assignment actor")
	}
	return uint16(actor), binary.LittleEndian.Uint32(p[8:]), nil
}

func RaidAssignmentUpdate(r RaidRecruitment) ([]byte, error) {
	if err := validateRaidRecruitment115(r); err != nil {
		return nil, err
	}
	p := binary.LittleEndian.AppendUint32(nil, r.ID)
	p = binary.LittleEndian.AppendUint32(p, 3)
	p = append(p, r.MemberCount)
	p = raidLeaderRecord115(p, r, r.MemberPosition)
	return binary.LittleEndian.AppendUint32(p, r.LeaderFame), nil
}

// raidBodySize accepts exactly the logical body size or its 8/16-byte aligned
// form; nonzero trailing padding is rejected. Retained for the entry-query and
// leave helpers that mirror the current-client padding contract.
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

// DecodeRaidLeaveRequest reads the native CMD657 context value (a client
// context u16, not our channel ID).
func DecodeRaidLeaveRequest(p []byte) (uint16, error) {
	if err := raidBodySize(p, 2); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint16(p[:2]), nil
}

// RaidKindForChannel is the native channel-to-raid-kind switch. These are
// protocol IDs, not mutable gameplay rules and not the F7 channel ID.
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

// RaidEmptyEntryCostInfo is NOTI585 with an empty record vector. Even an empty
// vector completes the native pending entry-cost query; omitting it leaves the
// raid UI locked.
func RaidEmptyEntryCostInfo() []byte { return add32(nil, 0) }

// RaidAssignmentSuccess is the native CMD661 mode-0 answer.
func RaidAssignmentSuccess() []byte { return add32([]byte{1}, 0) }

// RaidCreateSuccess answers CMD656 create with a u32 raid identity.
func RaidCreateSuccess(id uint32) []byte { return add32([]byte{1}, id) }

// RaidLeaveSuccess answers CMD657; the trailing u8 is the normal/penalty flag.
func RaidLeaveSuccess() []byte { return []byte{1, 0} }

// RaidRecruitmentList is NOTI577 with one record per entry. An empty vector is
// a valid result for the pending query; records use the current-client layout.
func RaidRecruitmentList(entries []RaidRecruitment) ([]byte, error) {
	if len(entries) > 128 {
		return nil, fmt.Errorf("raid list too large")
	}
	p := add32(nil, uint32(len(entries)))
	for _, r := range entries {
		p = raidRecruitmentRecord115(p, r)
		p = append(p, 1)
	}
	return p, nil
}
