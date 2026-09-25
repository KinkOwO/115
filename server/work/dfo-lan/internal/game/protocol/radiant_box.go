package protocol

import (
	"encoding/binary"
	"fmt"
)

// RadiantBoxEvent is the event id the radiant treasure box window sends. Its
// sender 141aa9240 writes opcode681, this event, sub-command1, and then the mode
// word it takes from the window's own material state.
const RadiantBoxEvent = 4202

// radiantBoxCounterKey is the highest reserved key of the notice's first list:
// keys up to it are stack counters, everything above is a prize template.
const radiantBoxCounterKey = 2

// RadiantBoxOpen is one EVENT_REQUEST for the radiant treasure box.
type RadiantBoxOpen struct {
	Event uint32
	Sub   uint32
	Mode  uint32
}

// DecodeRadiantBoxOpen reads the ordinary event-request envelope. 141aa9240
// sends u32 event, u32 sub-command, u32 parameter; only the event and the
// sub-command identify the request, the mode word follows the window's buttons.
func DecodeRadiantBoxOpen(p []byte) (RadiantBoxOpen, error) {
	if len(p) < 12 {
		return RadiantBoxOpen{}, fmt.Errorf("event request is shorter than three words")
	}
	r := RadiantBoxOpen{
		Event: binary.LittleEndian.Uint32(p),
		Sub:   binary.LittleEndian.Uint32(p[4:]),
		Mode:  binary.LittleEndian.Uint32(p[8:]),
	}
	if r.Event != RadiantBoxEvent {
		return r, fmt.Errorf("event %d is not the radiant treasure box", r.Event)
	}
	if r.Sub != 1 {
		return r, fmt.Errorf("radiant treasure box sub-command %d is not implemented", r.Sub)
	}
	return r, nil
}

// RadiantBoxEntry is one 8-byte entry of NOTI2551's first list. 1410723b0 reads
// a count and then that many {key,value} pairs, and 141073ba0 stores them in one
// map: a prize uses its template as the key, while the stack counters use the
// reserved keys 0 and 2 that 141072df0 reads back (0 fills the bonus slots in
// 141aaae60, 2 is the cumulative counter 141aa96b0 gates its thresholds with).
type RadiantBoxEntry struct {
	Key   uint32
	Value uint32
}

// RadiantBoxRow 是 NOTI2551 第二张表中的一个 24 字节行。141073ba0 以首字段
// 为键，将其余四个字段追加到对应行列表。141aa9db0/141aaa0e0 从 key1 读取
// 1 个或 10 个主结果，141aa9950/141aa9b80 从 key3 读取单个额外奖励；追加字段
// 依次是状态、模板、数量和里程阈值。
type RadiantBoxRow struct {
	Key       uint32
	Pad       uint32
	Flag      uint32
	Template  uint32
	Count     uint32
	Threshold uint32
}

// RadiantBoxNotice 构造 NOTI2551。包体依次为第一表数量、若干 {key,value}、
// 第二表数量和若干 24 字节行。第一表驱动计数器；第二表 key1 既承载主结果，
// 也使客户端在应用通知后刷新窗口。
func RadiantBoxNotice(entries []RadiantBoxEntry, rows []RadiantBoxRow) ([]byte, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("radiant box notice needs its counters")
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("radiant box notice needs its stack state")
	}
	p := add32(nil, uint32(len(entries)))
	for _, z := range entries {
		// The reserved stack counters may legitimately read zero; a prize may not.
		if z.Value == 0 && z.Key > radiantBoxCounterKey {
			return nil, fmt.Errorf("radiant box entry %d has no count", z.Key)
		}
		p = add32(p, z.Key)
		p = add32(p, z.Value)
	}
	p = add32(p, uint32(len(rows)))
	for _, r := range rows {
		p = add32(p, r.Key)
		p = add32(p, r.Pad)
		p = add32(p, r.Flag)
		p = add32(p, r.Template)
		p = add32(p, r.Count)
		p = add32(p, r.Threshold)
	}
	return p, nil
}
