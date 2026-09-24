package legion

import "encoding/binary"

// NOTI2895 LEGION_INFO is the packet the client's entry flow waits for after
// CMD2043. Until now the server answered the entry request with its
// acknowledgement only, so the client's content object stayed on the values its
// own constructor wrote and the entry screen never came up.
//
// The layout below is not a guess: it is the exact byte map of the client's own
// initializer sub_140698280, recovered in analysis/tasks/next75-legion-s2c-structures.md
// §2. The initializer writes every offset and default value the handler then
// overwrites with the packet body, so the offsets and the defaults are both
// pinned by the same evidence. Two independent readings of the same table agree:
// the handler itself and sub_14069A4D0's per-field +104 view.
//
// The client's handler is:
//
//	sub_140698280(buf)      // client's initial values
//	sub_146EA0BE0(buf, 204) // overwrite the whole 204 bytes from the body
//	sub_14069A4D0(singleton, buf)
//	sub_142AC2290(obj, @3)  // state
//	sub_142AC2220(obj, @7)  // mode
//	sub_142AC21B0(obj, @11) // third value
//
// The first field of the packet is the same u16 channel code CMD2354/CMD2896
// carry (OperationChannelCode, 107): the client refuses the packet unless it
// reads exactly that.
//
// What is NOT established: the meaning of the three extracted values (@3, @7,
// @11). next75 §2 records them as unresolved and the earlier assumption that
// they feed the weekly entry/reward counters has since been disproved — those
// gates read *(i16*)(*(qword)(obj+128)+352/354) against a client-side table,
// which is somewhere else entirely. Sending the client's own initializer values
// is therefore the only honest choice available today: it is exactly what the
// client would hold if the packet never arrived, so it cannot invent state.
// Once a capture shows which value the client expects, drive it from here.
const (
	// LegionInfoBodySize is the body the client's reader consumes, verbatim
	// from sub_146EA0BE0(buf, 204).
	LegionInfoBodySize = 204

	// LegionInfoSize is the whole payload: the u16 channel code plus the body.
	LegionInfoSize = 2 + LegionInfoBodySize
)

// Defaults written by the client's initializer sub_140698280. They are exported
// so a caller can hand them back unchanged and so the tests can pin them.
const (
	// LegionInfoDefaultState is @3 (u32). sub_142AC2290 feeds it to the content
	// object's state; 14 is the value the constructor leaves behind.
	LegionInfoDefaultState uint32 = 14
	// LegionInfoDefaultMode is @7 (u32), handed to sub_142AC2220.
	LegionInfoDefaultMode uint32 = 4
	// LegionInfoDefaultThird is @11 (u64), handed to sub_142AC21B0. The
	// initializer writes -1.
	LegionInfoDefaultThird uint64 = ^uint64(0)
)

// LegionInfoValues are the three fields the client extracts out of the body.
// Everything else in the packet is fixed structure.
type LegionInfoValues struct {
	State uint32 // @3
	Mode  uint32 // @7
	Third uint64 // @11
}

// DefaultLegionInfo returns the values the client's own initializer writes,
// which makes LegionInfo(DefaultLegionInfo()) a state-preserving packet.
func DefaultLegionInfo() LegionInfoValues {
	return LegionInfoValues{
		State: LegionInfoDefaultState,
		Mode:  LegionInfoDefaultMode,
		Third: LegionInfoDefaultThird,
	}
}

// LegionInfoBody builds the 204-byte body. Offsets and defaults are the
// initializer's; the three variable fields are taken from v. The gaps the
// initializer leaves (offset 20, and the six 12-byte records at 27..98) stay
// zero, which is what it writes there.
func LegionInfoBody(v LegionInfoValues) []byte {
	b := make([]byte, LegionInfoBodySize)
	// @0 u16 = -1, @2 u8 = -1
	binary.LittleEndian.PutUint16(b[0:], 0xFFFF)
	b[2] = 0xFF
	// @3 / @7 / @11 are the three values the handler hands to the content
	// object after the read.
	binary.LittleEndian.PutUint32(b[3:], v.State)
	binary.LittleEndian.PutUint32(b[7:], v.Mode)
	binary.LittleEndian.PutUint64(b[11:], v.Third)
	// @19 u8 = 0, @20 is not named by the initializer, @21 u16 = 0.
	// @23 u32 = -1
	binary.LittleEndian.PutUint32(b[23:], 0xFFFFFFFF)
	// @27..98: six 12-byte records, cleared by the initializer.
	// @99 u32 = -1
	binary.LittleEndian.PutUint32(b[99:], 0xFFFFFFFF)
	// @103 u32 = 0, @107 16 bytes = 0.
	// @123 u32 = four bytes of 1.
	binary.LittleEndian.PutUint32(b[123:], 0x01010101)
	// @127 u8 = 0, @128 16 bytes = 0, @144 8 bytes = 0,
	// @152/@168/@184 16 bytes each = 0, @200 u32 = 0.
	return b
}

// LegionInfo builds the NOTI2895 payload: the u16 channel code followed by the
// 204-byte body.
func LegionInfo(v LegionInfoValues) []byte {
	out := make([]byte, LegionInfoSize)
	binary.LittleEndian.PutUint16(out, OperationChannelCode)
	copy(out[2:], LegionInfoBody(v))
	return out
}
