package protocol

import (
	"encoding/binary"
	"fmt"
)

// AppendWeeklyOpening115 activates the already registered native actor and
// icon 197 through presence of event 387 in the complete NOTI108 snapshot.
// 145696a90 diffs IDs; 144922220 sets the registered icon's active byte.
// No generic XUI popup is requested: this actor owns popup 1008 itself.
// 145699bb0 accepts an empty XUI and the row remains in the parsed table.
func AppendWeeklyOpening115(base []byte, title string, start, end uint32) ([]byte, error) {
	if len(base) < 3 || binary.LittleEndian.Uint16(base) == 65535 || title == "" || end <= start || end > 0x7fffffff {
		return nil, fmt.Errorf("invalid native weekly opening")
	}
	// Parse the owned metadata table before appending. Reject duplicates and
	// nonempty fixed-event extensions rather than treating any last zero as a
	// delimiter. Each present row has five length-prefixed strings.
	off := 2
	skip := func(n int) bool {
		if n < 0 || n > len(base)-off {
			return false
		}
		off += n
		return true
	}
	str := func() bool {
		if !skip(4) {
			return false
		}
		n := uint64(binary.LittleEndian.Uint32(base[off-4 : off]))
		if n > uint64(len(base)-off) {
			return false
		}
		off += int(n)
		return true
	}
	for i := 0; i < int(binary.LittleEndian.Uint16(base)); i++ {
		if !skip(3) {
			return nil, fmt.Errorf("truncated event row")
		}
		if binary.LittleEndian.Uint16(base[off-3:]) == uint16(WeeklyAttendanceEvent115) {
			return nil, fmt.Errorf("duplicate weekly opening")
		}
		present := base[off-1]
		if present == 0 {
			continue
		}
		if present != 1 || !skip(2) || !str() || !str() || !str() || !skip(8) || !str() || !str() || !skip(1) {
			return nil, fmt.Errorf("invalid event snapshot")
		}
	}
	if off != len(base)-1 || base[off] != 0 {
		return nil, fmt.Errorf("unsupported fixed event extension")
	}
	out := append([]byte(nil), base[:off]...)
	binary.LittleEndian.PutUint16(out, binary.LittleEndian.Uint16(base)+1)
	out = add16(out, uint16(WeeklyAttendanceEvent115))
	out = append(out, 1, 0, 0)
	text := func(s string) { out = add32(out, uint32(len(s))); out = append(out, []byte(s)...) }
	text(title)
	text("")
	text("")
	out = add32(out, start)
	out = add32(out, end)
	text("")
	text("")
	return append(out, 0, 0), nil
}

// Current NetworkProcCOMMAND consumes failure/error before the event actor ID.
func WeeklyAttendanceRefusal115(code uint16) []byte {
	return add32([]byte{0, byte(code), byte(code >> 8)}, WeeklyAttendanceEvent115)
}
