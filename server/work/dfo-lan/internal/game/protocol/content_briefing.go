package protocol

// CMD2285 (ENUM_CMDPACKET_CONTENT_BRIEFING) is the report the client sends on
// every "结束游戏" click, in the same millisecond as CMD1302
// (ENUM_CMDPACKET_USER_CHECKSTAT_DISTRIBUTION): across the retained sessions the
// two counters are always equal to the Exit Button Clicked counter, so the pair
// belongs to the exit button alone.
//
// The client keeps a receive handler for 2285 (0x145250c80). It reads a fixed
// 0x80-byte body into a zeroed stack buffer and hands that buffer to
// 0x14137f810, which walks it as 32 little-endian u32 slots:
//
//	slot 0..3    -> the four text controls at [window+0x798+i*0x30]. A zero slot
//	                takes the client's own localised default (at14137f94b ->
//	                at14137f951); a non-zero slot is rendered by 0x145a0eb50,
//	                which formats the value with a built-in format string.
//	slot 13, 22, 31 -> three values clamped to 999999 (0xf423f) at
//	                at14137fa72..at14137faa7.
//
// The emulator never answered 2285, so the window this handler fills never
// received content and the in-game exit button had nothing to show.
//
// ExitContentBriefing emits exactly that fixed-width body. The slots stay a
// caller-supplied parameter so the per-field meaning can be pinned by a later
// capture without changing the wire contract.
const contentBriefingSlots = 32

// ExitContentBriefing emits the full response packet: a leading result byte
// followed by the fixed-width body. Every response in this protocol carries
// that byte first — Refusal is {0,...}, MenuLeaveSuccess is {1,...} — and the
// dispatcher hands it to the handler as dl, which 0x145250c80 tests with
// "test dl, dl / je <return>" before reading anything. A body sent without it
// reads as Result : Error on the client and the handler returns immediately,
// which is exactly what an all-zero first slot produces.
func ExitContentBriefing(slots [contentBriefingSlots]uint32) []byte {
	packet := make([]byte, 0, 1+contentBriefingSlots*4)
	packet = append(packet, 1)
	for _, v := range slots {
		packet = add32(packet, v)
	}
	return packet
}

// ExitContentBriefingDefaults is the body sent until a capture pins the real
// per-slot values. Every slot is zero, the encoding the client itself reads as
// "keep the localised default" at14137f94b and at14137fa51, so the window is
// populated from the client's own text instead of invented numbers.
func ExitContentBriefingDefaults() []byte {
	return ExitContentBriefing([contentBriefingSlots]uint32{})
}

// ContentBriefingBodySize is the body length the client's 2285 handler demands
// (mov edx, 0x80 at145250cb1 before its read call). ContentBriefingPacketSize
// adds the leading result byte.
const (
	ContentBriefingBodySize   = contentBriefingSlots * 4
	ContentBriefingPacketSize = ContentBriefingBodySize + 1
)
