package protocol

import (
	"encoding/binary"
	"fmt"
)

const DailyAttendanceEvent115 uint32 = 365
const DailyAttendanceInfo115 uint16 = 1536

// DailyAttendanceStatus115 preserves NOTI1536 reader 1435478e0's fields.
// The suffixes identify native object offsets whose gameplay meanings have
// not all been established. Do not substitute eligibility guesses for them.
// No gateway emits this status until the source and action semantics close.
type DailyAttendanceStatus115 struct {
	CurrentDay byte
	Field281   byte
	Counts     []uint32
	Field2A0   uint32
	Field2A4   uint32
	Claimed    uint32
	Cumulative uint32
	Field2B0   byte
}

func (s DailyAttendanceStatus115) Bytes() ([]byte, error) {
	if len(s.Counts) > 255 {
		return nil, fmt.Errorf("daily status count exceeds native byte")
	}
	p := []byte{s.CurrentDay, s.Field281, byte(len(s.Counts))}
	for _, n := range s.Counts {
		p = add32(p, n)
	}
	for _, n := range []uint32{s.Field2A0, s.Field2A4, s.Claimed, s.Cumulative} {
		p = add32(p, n)
	}
	return append(p, s.Field2B0), nil
}

type DailyAttendanceRequest115 struct {
	Cumulative bool
	Index      uint32
}

// Native 143548370 sends CMD680/event365/action/index as three dwords.
// UI callbacks 143f78850 and 143f78700 select actions 1 and 2. The
// current refresh path 144102660 also requests cumulative rows with action 2;
// it is not evidence for an eight-byte synchronization action.
func DecodeDailyAttendanceRequest115(p []byte) (DailyAttendanceRequest115, error) {
	if len(p) != 12 || binary.LittleEndian.Uint32(p) != DailyAttendanceEvent115 {
		return DailyAttendanceRequest115{}, fmt.Errorf("invalid daily request shape")
	}
	action, index := binary.LittleEndian.Uint32(p[4:]), binary.LittleEndian.Uint32(p[8:])
	if (action != 1 && action != 2) || index == 0 || index > 32 {
		return DailyAttendanceRequest115{}, fmt.Errorf("unverified daily action or native mask index")
	}
	return DailyAttendanceRequest115{Cumulative: action == 2, Index: index}, nil
}
