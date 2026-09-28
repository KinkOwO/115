package legion

import "encoding/binary"

const (
	// ApocalypseContent 是军团**内容号**：客户端用它在 Contents/2026/Apocalypse/Etc/Apocalypse.ctp
	// 装载末世录。三处都用它：CMD2043 的 u32@13、CMD2354/CMD2896 的首 u16、NOTI2895 的前缀 u16。
	// （分享版 APOCALYPSE-ENTRY-20260921 的实机日志里 content=107 即此；与本地
	//  OperationChannelCode=107 是同一个数，但**语义不同**：那是内容号，不是频道类型——
	//  频道类型是客户端 clientchannelinfo 里的 119。）
	ApocalypseContent uint16 = 107
	// LegionInfoBody 是 NOTI2895 状态体长度（不含前面的 u16 content）。
	LegionInfoBody = 204
	// 兼容上游旧常量名：本补丁把 LegionInfo 的签名改成显式带 content（语义未变），
	// 这两个名字沿用上游，已有调用方/测试不必跟着改。
	LegionInfoBodySize = LegionInfoBody
	LegionInfoSize     = 2 + LegionInfoBody
)

// Current 2.38.3.25: 1424FE570 -> 142AC2A50/29E0/2970 ->
// mode virtual32/48/64 -> mode+116/120/124. These are NOT balances.
// The former Entry/Reward/uint64 Phase labels misidentified adjacent u32s.
type LegionInfoState struct {
	OperationChoice byte   // @2: zero-based difficulty, CTP lookup is choice+1
	State           uint32 // @3: operation confirmation explicitly requires 2
	Outcome         uint32 // @7: independent mode status, not a wallet balance
	Stage           uint32 // @11: CMD2045 entry index; also selects the6x12 row
	Following       uint32 // @15: independent field, not high bits of Stage
}

// DefaultLegionInfoState 是客户端初始化器写死的初值（14 / 4 / -1）。
func DefaultLegionInfoState() LegionInfoState {
	return LegionInfoState{OperationChoice: 255, State: 14, Outcome: 4, Stage: ^uint32(0), Following: ^uint32(0)}
}

// Start's town-side waiting state (source waiting position239/2). Never send
// the constructor-only state14 in response to a successfully accepted start.
func WaitingLegionInfoState() LegionInfoState {
	s := DefaultLegionInfoState()
	s.State, s.Outcome, s.Stage = 2, 0, 0
	return s
}

// LegionInfo 构造 NOTI2895（LEGION_INFO）载荷：u16 content + 204 字节状态。
// 布局（相对 204 字节体，未列出的偏移按客户端初值写 0）：
//
//	0 u16FFFF  2 u8OperationChoice  3 u32State  7 u32Outcome
//	11 u32Stage 15 u32Following 19 u8=0 21 u16=0 23 u32FFFFFFFF
//	27 6×12记录：byte0=FF/u64@4=FFFFFFFFFFFFFFFF，余3B清零
//	107 16 字节清零    123 u32 0x01010101 127 u8 0         128 16 字节清零
//	144 u64 0          152/168/184 各 16 字节清零          200 u32 0
//
// 合计 204。
func LegionInfo(content uint16, state LegionInfoState) []byte {
	p := make([]byte, 2+LegionInfoBody)
	binary.LittleEndian.PutUint16(p[0:], content)
	b := p[2:]
	binary.LittleEndian.PutUint16(b[0:], 0xFFFF)
	b[2] = state.OperationChoice
	binary.LittleEndian.PutUint32(b[3:], state.State)
	binary.LittleEndian.PutUint32(b[7:], state.Outcome)
	binary.LittleEndian.PutUint32(b[11:], state.Stage)
	binary.LittleEndian.PutUint32(b[15:], state.Following)
	binary.LittleEndian.PutUint32(b[23:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(b[99:], 0xFFFFFFFF)
	// Native constructor defaults only. Production Apocalypse projection
	// replaces these four shortcut-open bytes with the shared live/frozen
	// schedule;14069A9C0 tests byte==1, not a u32 balance or party count.
	binary.LittleEndian.PutUint32(b[123:], 0x01010101)
	// 1471B4530 is not a memset: unset destination and timestamp are -1.
	for i := 0; i < 6; i++ {
		off := 27 + 12*i
		b[off] = 255
		binary.LittleEndian.PutUint64(b[off+4:], ^uint64(0))
	}
	return p
}
