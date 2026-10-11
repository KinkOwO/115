package protocol

import (
	"encoding/binary"
	"fmt"
)

// OathSystemRequest is the observed current-client C2S2382 body. The variant
// tag and trailing cipher padding are opaque; only the proven fields are read.
type OathSystemRequest struct {
	ItemID            uint32
	Option            byte
	ExplicitSelection bool
}

func DecodeOathSystemRequest(body []byte) (OathSystemRequest, error) {
	if len(body) != 32 {
		return OathSystemRequest{}, fmt.Errorf("oath system request must be 32 bytes")
	}
	var explicit bool
	switch string(body[4:13]) {
	case "\x01\x00\x00\x00\x00\x00\x00\x00\x00":
		explicit = true
	case "\x00\x00\x00\x00\x00\x08\x00\x00\x00":
	default:
		return OathSystemRequest{}, fmt.Errorf("unknown oath system request control")
	}
	r := OathSystemRequest{ItemID: binary.LittleEndian.Uint32(body[13:17]), Option: body[17], ExplicitSelection: explicit}
	if r.ItemID == 0 || r.Option < 1 || r.Option > 3 {
		return OathSystemRequest{}, fmt.Errorf("invalid oath item or option")
	}
	return r, nil
}

// OathSystemInfo 是 S2C 2839（`ENUM_NOTIPACKET_OATH_SYSTEM_INFO`）的载荷。
//
// ⚠️ 2026-10-04 补的载荷几何（IDA 工作副本，客户端解析器 `sub_1474AFDC0` 逐字段拷贝）：
//
//	[0:8)    → obj+0    (u64；低 4 字节 = 当前选中的誓约/选项，已由实机确认)
//	[8:12)   → obj+8    (u32)
//	[12:14)  → obj+12   (u16)
//	[14]     → obj+14   (u8)
//
// 合计客户端真正读 **15 字节**；构造器 `sub_1474A5170` 把 obj+0..15 清零。
// **我们目前只填 `[0:4)` 的 option，其余 11 字节保持 0** —— 用户报的「誓约没有积分」
// 极可能就在这 11 字节里（誓约窗口显示 `0/750次` 与「已添加的誓约积分: 0」）。
// 载荷总长仍发 24 字节：既有实机稳定（客户端只读前 15），改长度会重蹈 2839 的 8 字节崩溃覆辙。
// 缺口与取证见 docs/protocol/oath-set-points-20261004.md。
const (
	// OathSystemInfoWireSize 是客户端解析器真正读取的字节数。
	OathSystemInfoWireSize = 15
	// OathSystemInfoSize 是我们发送的载荷长度（既有实机稳定；客户端只读前 15 字节）。
	OathSystemInfoSize = 24
)

// OathSystemInfo 组 2839 的载荷：目前只有 `[0:4) = option` 是已确认字段。
func OathSystemInfo(level int, option int) ([]byte, error) {
	if option < 0 || option > 3 {
		return nil, fmt.Errorf("invalid oath option")
	}
	if level < 115 {
		option = 0
	} else if option == 0 {
		option = 1
	}
	body := make([]byte, OathSystemInfoSize)
	binary.LittleEndian.PutUint32(body[:4], uint32(option))
	// [4:15) 的三个字段（obj+8 u32 / obj+12 u16 / obj+14 u8）暂留 0：
	// 等 IDA 定量"誓约积分/套装积分"落在哪一格再填，不猜。
	return body, nil
}
