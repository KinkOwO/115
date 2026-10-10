package protocol

import (
	"encoding/binary"
	"fmt"
)

const WeeklyAttendanceEvent115 uint32 = 387

type WeeklyAttendanceRequest115 struct {
	Sync       bool
	Cumulative bool
	Index      uint32
	NativeID   uint32
}

// Current US senders: 142e14c00 sync (8 bytes), 142e14c50 period
// (12 logical bytes, padded to 16 by CMD680's eight-byte transport blocks),
// 142e14b10 cumulative (16 bytes, duplicate claim ID). The 2026-10-08 native
// curRecv capture is 83010000020000000100000000000000.
// 147b402c0 counts the [accu period reward info] vector at parser+80.
// 142e14b10 chooses base 200 when that vector is nonempty, otherwise 100.
// [click button on accu reward] is a different field at parser+cc.
// Only a period ID may have a zero transport tail. Cumulative requests must
// retain their duplicate ID; never trim zero bytes from arbitrary requests.
func DecodeWeeklyAttendanceRequest115(p []byte, hasTimeCumulative bool) (WeeklyAttendanceRequest115, error) {
	var r WeeklyAttendanceRequest115
	if len(p) < 8 || binary.LittleEndian.Uint32(p) != WeeklyAttendanceEvent115 {
		return r, fmt.Errorf("invalid weekly event request")
	}
	action := binary.LittleEndian.Uint32(p[4:])
	if action == 1 && len(p) == 8 {
		r.Sync = true
		return r, nil
	}
	if action != 2 || (len(p) != 12 && len(p) != 16) {
		return r, fmt.Errorf("invalid weekly request shape")
	}
	r.NativeID = binary.LittleEndian.Uint32(p[8:])
	if r.NativeID >= 1 && r.NativeID <= 31 {
		if len(p) == 16 && binary.LittleEndian.Uint32(p[12:]) != 0 {
			return r, fmt.Errorf("weekly period has nonzero transport padding")
		}
		r.Index = r.NativeID
		return r, nil
	}
	if len(p) == 12 {
		return r, fmt.Errorf("weekly period exceeds native mask")
	}
	base := uint32(100)
	if hasTimeCumulative {
		base = 200
	}
	if r.NativeID != binary.LittleEndian.Uint32(p[12:]) || r.NativeID <= base || r.NativeID > base+31 {
		return r, fmt.Errorf("invalid weekly cumulative identity")
	}
	r.Cumulative, r.Index = true, r.NativeID-base
	return r, nil
}

// NOTI1753 reader 142e133b0 consumes u32/u8/u32/u32/u32/u32.
// Native bt helper 147211ab0 uses 1<<index, with period indices starting at 1.
// These are raw fields; this encoder does not infer eligibility from a clock.
type WeeklyAttendanceStatus115 struct {
	EligiblePeriods       uint32
	CurrentPeriod         byte
	OnlineSeconds         uint32
	ClaimedPeriods        uint32
	ClaimedCumulative     uint32
	ClaimedTimeCumulative uint32
}

func (s WeeklyAttendanceStatus115) Bytes() ([]byte, error) {
	if s.CurrentPeriod > 31 || (s.EligiblePeriods|s.ClaimedPeriods|s.ClaimedCumulative|s.ClaimedTimeCumulative)&1 != 0 {
		return nil, fmt.Errorf("weekly status exceeds native one-based masks")
	}
	p := add32(nil, s.EligiblePeriods)
	p = append(p, s.CurrentPeriod)
	for _, v := range []uint32{s.OnlineSeconds, s.ClaimedPeriods, s.ClaimedCumulative, s.ClaimedTimeCumulative} {
		p = add32(p, v)
	}
	return p, nil
}
