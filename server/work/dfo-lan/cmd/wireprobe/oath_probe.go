package main

// 注入器：按 id / 长度 / 填充字节 / 哨兵字段 向客户端发**任意 noti**，挂在本副本加载应答上。
//
// 为什么需要它（2026-09-27 取证链）：
//   - `runtime/strexplore` 枚举客户端字符串表后证实，**全客户端只有两个 grade getter**：
//     `getPrimerGrade()`(28460867) 与 `getOathGrade()`(28460901)（`Grade()` 只命中这三个字符串，
//     另一个是 `t:getMonsterGrade()`）。
//   - 而且 `c:season*` 这类脚本变量**一条都不存在**（`c:oath*` 6 条、`c:primer*` 7 条，
//     全部属于本副本自己的状态机）⇒ 两个 getter 读的是**引擎态**，只可能由某个服务端包
//     （noti 2837/2839/2842 那一族 —— 其 handler 把载荷整块存下来）或本地数据喂。
//
// 于是这就成了一个现成的 oracle：
//
//	发一条候选 noti → 进图打一下天平 → 读日志槽里的**实时** getPrimerGrade()（探针 v3 已放进去）。
//
// 只要某个 (id, 长度, 字段偏移) 让那个数从 72 变成别的值，**载体与几何就同时到手**。
//
// ⚠️ 2026-09-27 06:1x 实机事故（务必先读）：发一条 **8 字节**的 2839 直接把客户端打崩了。
// 崩溃栈（会话里的 client-direct.out）`0xc0000005 violationAddr=0x0 @ 0x146EA0C30`，
// 而 0x146EA0C30 是个**故意的空写陷阱**：
//
//	sub_146EA0BE0:
//	  cmp  cs:dword_14F1BF878, esi   ; 可用字节数 vs 请求字节数
//	  jl   loc_146EA0C30             ; 请求 > 可用 → 跳进陷阱
//	loc_146EA0C30:
//	  mov  dword ptr ds:0, 0         ; 空指针写 = 0xc0000005 @ 0
//
// 调用者（sub_1405757F0，在通知分派链上）读 **15 字节** ⇒ 客户端对 noti 载荷有**长度下限**，
// 短了就自杀。所以载荷**必须给足**（经验值 256 字节全 0，别拿长度当变量扫）。
// 副产品：这次崩溃同时证明 **2839 在通知分派链里确实有活处理器** ⇒ 它是个真候选。
//
// 用法（默认关闭）：
//
//	-oath-inject "2839:256:0"               一条候选、256 字节全 0（**推荐的起点**）
//	-oath-inject "2839:256:0;8:71;12:45"    再在偏移 8 放 71、偏移 12 放 45，探字段
//	-oath-inject "2837:256:0,2839:256:0"    多个候选（每个都够长）
//
// 环境变量 DFO_OATH_INJECT 等价。

import (
	"fmt"
	"strconv"
	"strings"
)

// oathInjectMinSize 是载荷长度下限。2026-09-27 实测：8 字节的 2839 让客户端在自己
// 的通知读取路径上踩到长度护栏并空写自杀（0xc0000005 @ 0x0, eip=0x146EA0C30）。
// oathInjectRequired maps each notification id to the exact number of payload bytes
// its own parser reads, taken from the parser bodies themselves:
//
//	2836 OMEN_OF_ORDER_PARTY_INFO   69
//	2837 ENDKEEPER_OF_ORDER_INFO    20   (four party character ids + one u32)
//	2838 ENDKEEPER_OF_ORDER_REWARD   8   (two u32 -> singleton +88 / +92, default 72/72)
//	2839 OATH_SYSTEM_INFO           15
//	2841                             6
//	2842 PRIMER_COLLECTION         104
//
// Every parser starts with one fixed-size sub_146EA0BE0(&buf, N) and then consumes buf,
// so a payload shorter than N trips the client's own guard (loc_146EA0C30, a deliberate
// null write) and kills the process - which is what an 8-byte 2839 did on 2026-09-27.
// For ids not in the table, give plenty rather than gamble on the length.
var oathInjectRequired = map[uint16]int{
	2836: 69,
	2837: 20,
	2838: 8,
	2839: 15,
	2841: 6,
	2842: 104,
}

// oathInjectUnknownFloor is what an id outside the table must clear.
const oathInjectUnknownFloor = 256

// oathInjectFloor returns the minimum payload length the client accepts for id.
func oathInjectFloor(id uint16) int {
	if n, ok := oathInjectRequired[id]; ok {
		return n
	}
	return oathInjectUnknownFloor
}

// oathInjectSpec 是一条候选通知：先填满 fill，再把 set 里的偏移逐个改成指定字节。
type oathInjectSpec struct {
	ID   uint16
	Size int
	Fill byte
	Set  map[int]byte
}

func (s oathInjectSpec) payload() []byte {
	p := make([]byte, s.Size)
	for i := range p {
		p[i] = s.Fill
	}
	for off, v := range s.Set {
		if off >= 0 && off < len(p) {
			p[off] = v
		}
	}
	return p
}

// parseOathInject 解析 "id:size:fill;off:val;off:val,..." 形式（逗号分隔多条）。
// 空串返回 nil（= 关闭）。
func parseOathInject(spec string) ([]oathInjectSpec, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return nil, nil
	}
	var out []oathInjectSpec
	for _, one := range strings.Split(spec, ",") {
		one = strings.TrimSpace(one)
		if one == "" {
			continue
		}
		parts := strings.Split(one, ";")
		head := strings.Split(parts[0], ":")
		if len(head) != 3 {
			return nil, fmt.Errorf("bad inject %q: want id:size:fill", one)
		}
		id, err := strconv.ParseUint(strings.TrimSpace(head[0]), 10, 16)
		if err != nil {
			return nil, fmt.Errorf("bad inject id in %q: %w", one, err)
		}
		size, err := strconv.Atoi(strings.TrimSpace(head[1]))
		if err != nil || size > 4096 {
			return nil, fmt.Errorf("bad inject size in %q", one)
		}
		// 长度下限不是洁癖：客户端对 noti 载荷有下限，短一条就打崩（见文件头的崩溃栈）。
		if size < oathInjectFloor(uint16(id)) {
			return nil, fmt.Errorf("inject size %d in %q is below the %d bytes id %d's own parser reads; "+
				"a short payload trips the client's guard and crashes it",
				size, one, oathInjectFloor(uint16(id)), id)
		}
		fill, err := strconv.ParseUint(strings.TrimSpace(head[2]), 10, 8)
		if err != nil {
			return nil, fmt.Errorf("bad inject fill in %q: %w", one, err)
		}
		s := oathInjectSpec{ID: uint16(id), Size: size, Fill: byte(fill), Set: map[int]byte{}}
		for _, kv := range parts[1:] {
			kv = strings.TrimSpace(kv)
			if kv == "" {
				continue
			}
			pair := strings.Split(kv, ":")
			if len(pair) != 2 {
				return nil, fmt.Errorf("bad inject field %q in %q: want off:val", kv, one)
			}
			off, err1 := strconv.Atoi(strings.TrimSpace(pair[0]))
			val, err2 := strconv.ParseUint(strings.TrimSpace(pair[1]), 10, 8)
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("bad inject field %q in %q", kv, one)
			}
			if off < 0 || off >= size {
				return nil, fmt.Errorf("inject field offset %d outside %d bytes in %q", off, size, one)
			}
			s.Set[off] = byte(val)
		}
		out = append(out, s)
	}
	return out, nil
}

// oathInjectNext 取队列里的**下一条**并推进一格。
//
// 这样「一次服务启动 + 反复进出副本」就能把整串候选扫完：客户端每进一次副本、天平每开场
// 一次都会重新读那两个 getter，而探针 v3 让「碰一下天平」就能把实时值打进日志。
// 发出去的包会被 writePackets 记成 oath_inject 事件（带 plain_hex），所以候选取哪一条
// 从日志就能认出来，不需要额外打标。
func (w *worldSession) oathInjectNext() []outboundPacket {
	if w == nil || len(w.oathInject) == 0 {
		return nil
	}
	s := w.oathInject[w.oathNext%len(w.oathInject)]
	w.oathNext++
	return []outboundPacket{{"oath_inject", 0, s.ID, s.payload()}}
}
