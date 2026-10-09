package protocol

import (
	"encoding/binary"
	"fmt"
	"time"
)

const NotiMoonLakeInfo uint16 = 2622

// N2622+4 -> mode+76. Moon phase3 checks1 for
// success presentation; phase5 checks2 for the failure cleanup branch.
// Phase alone does not encode the result (G0260 S817/S837/S841 all use1).
//
// ⚠️ 旧注释里的「through 1401B70A0」不要当消费点依据（2026-10-09 IDA 复核）：
// 该地址落在 sub_1401B7050 内部，那是个**通用字节码/属性读取循环**
// （`movzx edi,[rdx] ; cmp edi,80h ; shl eax,3 …`），与 N2622 的字段语义无关。
func MoonLakeOutcome115(p []byte, success bool) ([]byte, error) {
	if len(p) != 126 {
		return nil, fmt.Errorf("invalid Moon result record")
	}
	phase := binary.LittleEndian.Uint32(p)
	if phase < 3 || phase > 5 {
		return nil, fmt.Errorf("Moon outcome outside result lifecycle")
	}
	out := append([]byte(nil), p...)
	value := uint32(2)
	if success {
		value = 1
	}
	binary.LittleEndian.PutUint32(out[4:], value)
	return out, nil
}

func DecodeMoonFever115(p []byte) error {
	if len(p) > 15 {
		return fmt.Errorf("Moon fever has no logical request body")
	}
	for _, b := range p {
		if b != 0 {
			return fmt.Errorf("nonzero Moon fever padding")
		}
	}
	return nil
}

func MoonFeverReply115(score uint32) ([]byte, error) {
	if score < 50 || score > 1000 {
		return nil, fmt.Errorf("invalid Moon fever score")
	}
	p := make([]byte, 5)
	p[0] = 1
	binary.LittleEndian.PutUint32(p[1:], score)
	return p, nil
}

// Bootstrap projection of the native126B record. Unallocated mechanisms use
// the constructor141328FE0's absent sentinels, NOT another player's snapshot.
// The host owns lifecycle, update time and the actual coin projection.
// Named encounters/grid mechanics require their own sourced owner updates.
func MoonLakeBootstrap115(phase uint32, at time.Time, floor ...uint32) []byte {
	p := make([]byte, 126)
	binary.LittleEndian.PutUint32(p, phase)
	if phase != 14 {
		binary.LittleEndian.PutUint64(p[8:], uint64(at.Unix()))
	}
	p[16] = 0 // the host supplies the actual upstream-owned revival balance
	for i := 33; i <= 76; i++ {
		p[i] = 255
	}
	if len(floor) == 1 {
		binary.LittleEndian.PutUint32(p[29:], floor[0])
	}
	for i := 97; i <= 104; i++ {
		p[i] = 255
	}
	if phase == 14 {
		p[16] = 255
		return p
	}
	for i := 78; i <= 81; i++ {
		p[i] = 8
	} // captured first-floor uncleared rows
	// The source door-grid origin is0/4; the captured initial record agrees.
	binary.LittleEndian.PutUint32(p[33:], 0)
	binary.LittleEndian.PutUint32(p[37:], 4)
	return p
}

// The common S1 prefix dispatches to14132EA60. Success returns without
// reading a body extension. Failure reads four u32 member slots even if none
// are selected by the error mask; give it a complete, identity-free record.
func SemiRaidStartReply115(ok bool) []byte {
	if ok {
		return []byte{1}
	}
	p := make([]byte, 19)
	for i := 3; i < len(p); i++ {
		p[i] = 255
	}
	return p
}

type MoonNamedRecord115 struct {
	Slot        byte
	Grid        [2]byte
	Dead        bool
	ResultIndex byte
}

func MoonLakeGauges115(p []byte, troop, fever uint32) ([]byte, error) {
	if len(p) != 126 || troop > 1000 || fever > 1000 {
		return nil, fmt.Errorf("invalid Moon gauge projection")
	}
	out := append([]byte(nil), p...)
	binary.LittleEndian.PutUint32(out[118:], troop)
	binary.LittleEndian.PutUint32(out[122:], fever)
	return out, nil
}

// MoonLakeRevives115 把「本局剩余复活币次数」写进 N2622 的 p[16]。
//
// 协议那格的原注释就是 "the host supplies the actual upstream-owned revival balance" ——
// 此前恒为 0（phase 14 关闭态时是 255 哨兵）。上限来自副本脚本的 [coin limit]
// （沉月湖两层都是 8，见 catalog.DungeonDefinition.CoinLimit）。
//
// ⚠️ **这一格的用途仍未证实**（2026-10-09）：业主实机看到的那格数字是 **38 = 背包里
// 复活币的持有量**（我们这一版已经在写 8），说明**显示不是取自 p[16]**；而 p[16] 到底是
// 「本局可用次数」还是别的，还要靠实机（死满 8 次看第 9 次是否被拒）或继续 IDA 才能定。
// 另：`[coin limit]` / `[coin info]` 在客户端是**脚本命令**（id 8878 / 7108，注册在
// sub_1473BB560 / sub_147444B30 这个 `.dgn` 解释器一族里）⇒ 上限本身是**客户端自己从
// .dgn 读的**，不需要服务端下发。`[revive count]` 是 PVP 模式那一族的标签，与本副本无关。
func MoonLakeRevives115(p []byte, left uint32) ([]byte, error) {
	if len(p) != 126 {
		return nil, fmt.Errorf("wrong Moon Lake info size")
	}
	phase := binary.LittleEndian.Uint32(p)
	if phase == 14 {
		// 关闭态那一格是 255 哨兵，不要覆盖。
		return p, nil
	}
	out := append([]byte(nil), p...)
	binary.LittleEndian.PutUint32(out[16:], left)
	return out, nil
}

func MoonLakeCleared115(p []byte, cleared [5]bool) ([]byte, error) {
	if len(p) != 126 {
		return nil, fmt.Errorf("wrong Moon Lake info size")
	}
	out := append([]byte(nil), p...)
	for i, clear := range cleared {
		if clear {
			out[77+i] = 0
		}
	}
	return out, nil
}

func MoonFirstCounts115(p []byte, counts [5]byte) ([]byte, error) {
	if len(p) != 126 {
		return nil, fmt.Errorf("wrong Moon first-floor population size")
	}
	out := append([]byte(nil), p...)
	copy(out[77:82], counts[:])
	return out, nil
}

// Native UI1415F0E30 reads the tracked named grid at97/101, defeated at105,
// then tests the per-grid population at82+3*x+y for x<5,y<3.
func MoonLakeGrid115(p []byte, counts [5][3]byte, named [2]int32, defeated bool) ([]byte, error) {
	if len(p) != 126 || named[0] < 0 || named[0] >= 5 || named[1] < 0 || named[1] >= 3 {
		return nil, fmt.Errorf("invalid Moon second-floor grid projection")
	}
	out := append([]byte(nil), p...)
	for x := range counts {
		for y, n := range counts[x] {
			out[82+3*x+y] = n
		}
	}
	binary.LittleEndian.PutUint32(out[97:], uint32(named[0]))
	binary.LittleEndian.PutUint32(out[101:], uint32(named[1]))
	out[105] = 0
	if defeated {
		out[105] = 1
	}
	return out, nil
}

// Native four9B records: absent=allFF, spawned=statusFF. The nonnegative
// result token is NOT boolean: G0260 camp/lake/gatekeeper carry0/1/2.
func MoonLakeNamedInfo115(p []byte, rows []MoonNamedRecord115) ([]byte, error) {
	if len(p) != 126 {
		return nil, fmt.Errorf("wrong Moon Lake info size")
	}
	out := append([]byte(nil), p...)
	seen := map[byte]bool{}
	results := map[byte]bool{}
	for _, r := range rows {
		if r.Slot >= 4 || seen[r.Slot] || r.Grid[0] != 0 || r.Grid[1] > 4 {
			return nil, fmt.Errorf("invalid Moon named record")
		}
		seen[r.Slot] = true
		at := 41 + int(r.Slot)*9
		binary.LittleEndian.PutUint32(out[at:], uint32(r.Grid[0]))
		binary.LittleEndian.PutUint32(out[at+4:], uint32(r.Grid[1]))
		out[at+8] = 255
		if r.Dead {
			if r.ResultIndex >= 4 || results[r.ResultIndex] {
				return nil, fmt.Errorf("invalid Moon named result index")
			}
			results[r.ResultIndex] = true
			out[at+8] = r.ResultIndex
		}
		out[77+int(r.Grid[1])] = 0
	}
	return out, nil
}
