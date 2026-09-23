package protocol

import "encoding/binary"

// BoosterGage encodes the NOTI398 (ENUM_NOTIPACKET_BOOSTER_GAGE, id 0x018E)
// frame that hides the town top-left "Liberation Trace" mileage panel.
//
// The panel cannot be removed by any packet: builder sub_1455C6F90 is called
// unconditionally during UI construction (single caller sub_145526430+0x2D1E,
// the condition only tests the freshly built object, no server data), and
// randomboostermileage2.xui has no IsVisible on its root node. NOTI398 is the
// only server-side handle: handler sub_1455C6D30 / refresher sub_1455C8D20
// computes displayValue / [sub gage] (Etc/BoosterGage.etc = 50) and calls
// SetVisible(quotient > 0) on the child control, so displayValue = 0 collapses
// the panel.
//
// ⚠️ Body is exactly 14 bytes, NOT 18. The reader at 0x1455C6D8E uses the
// fixed-4-byte reader 0x146EA0BA0 (mov eax,dword[rdx]; add rdx,4); a first
// pass read it as u64 and encoded u64+u64+u8+u8 = 18 bytes, which overruns
// the declared frame length and the writer swallows following CMDs (the known
// wire-overflow-report-217 path). Field order:
//
//	u8 incA, u8 incB, u64 rawA, u32 displayValue
//
// The two u8s are cumulative increments (add eax,0xC4; add [rsi+4],ecx, then
// compared with the cap [r14+0x380]), not set values: keep them 0. They must
// never be reused to "reset" booster progress.
func BoosterGage(displayValue uint32) []byte {
	p := []byte{0, 0}
	p = binary.LittleEndian.AppendUint64(p, 0)
	return binary.LittleEndian.AppendUint32(p, displayValue)
}
