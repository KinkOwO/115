package protocol

import (
	"encoding/binary"
	"fmt"
)

// NetworkProcCOMMAND 1459a1bb0 consumes status, then the u16 error BEFORE
// dispatch. Handler 14566d3b0 subsequently reads event ID, and 331's vtable
// +0x50 points to 1403e46a0 (clear +0x58 wait). Do not reuse the older event
// refusal helper's event-before-error order for this current-client path.
func AttendanceDailyRefusal115(code uint16) []byte {
	p := []byte{0, byte(code), byte(code >> 8)}
	return add32(p, AttendanceDailyEvent115)
}

// AppendAttendanceOpening115 preserves the complete owned event snapshot.
// The isolated 331 row at offset 2446 in the decompressed Oct-02 NOTI108 has
// flags 1/2/4, three initial strings, dates, calendar/empty strings and flag 1.
// Dates, title and calendar come from the selected source's local adaptation.
func AppendAttendanceOpening115(base []byte, title, calendar string, start, end uint32) ([]byte, error) {
	if len(base) < 3 || base[len(base)-1] != 0 || title == "" || calendar == "" || end <= start || binary.LittleEndian.Uint16(base) == 65535 {
		return nil, fmt.Errorf("invalid attendance opening source/snapshot")
	}
	out := append([]byte(nil), base[:len(base)-1]...)
	binary.LittleEndian.PutUint16(out, binary.LittleEndian.Uint16(base)+1)
	out = add16(out, uint16(AttendanceDailyEvent115))
	out = append(out, 1, 2, 4)
	text := func(s string) { out = add32(out, uint32(len(s))); out = append(out, []byte(s)...) }
	text(title)
	text("")
	text("")
	out = add32(out, start)
	out = add32(out, end)
	text(calendar + "/0/0")
	text("")
	return append(out, 1, 0), nil
}

const AttendanceDailyEvent115 uint32 = 331

// DecodeAttendanceDailyClaim115 is deliberately separate from the 662 helper:
// day zero is valid here. Native sender 143869aa0 appends exactly event/day;
// captured Oct-02 first-day request is 4b01000000000000.
func DecodeAttendanceDailyClaim115(p []byte) (uint32, error) {
	if len(p) != 8 || binary.LittleEndian.Uint32(p) != AttendanceDailyEvent115 {
		return 0, fmt.Errorf("invalid daily attendance request")
	}
	day := binary.LittleEndian.Uint32(p[4:])
	if day >= 28 {
		return 0, fmt.Errorf("attendance day exceeds native array")
	}
	return day, nil
}

// AttendanceDailyStatus115 encodes only the 33 bytes consumed by reader
// 143869190: byte, u32, 28 bytes. Gate stays an explicit raw native field until
// its remaining branches are closed; this codec does not invent its semantics.
// The seven trailing bytes in captured decrypted 40-byte blocks are not fields.
func AttendanceDailyStatus115(gate byte, count uint32, days [28]byte) ([]byte, error) {
	if gate > 1 || count > 28 {
		return nil, fmt.Errorf("invalid daily attendance status")
	}
	p := make([]byte, 33)
	p[0] = gate
	binary.LittleEndian.PutUint32(p[1:], count)
	for i, state := range days {
		if state > 2 {
			return nil, fmt.Errorf("unverified attendance state %d", state)
		}
		p[5+i] = state
	}
	return p, nil
}
