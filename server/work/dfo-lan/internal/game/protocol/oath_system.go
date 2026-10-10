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

// NOTI2839 reads 15 bytes: u32 core selection, u8 truth level,
// ten zero-based option indices for source slots 0..9. The established
// 24-byte transport payload is retained. See oath_truth.go for native evidence.
const (
	// OathSystemInfoWireSize 是客户端解析器真正读取的字节数。
	OathSystemInfoWireSize = 15
	// OathSystemInfoSize 是我们发送的载荷长度（既有实机稳定；客户端只读前 15 字节）。
	OathSystemInfoSize = 24
)

// OathSystemInfo preserves the legacy zero-progress projection.
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
	return body, nil
}
