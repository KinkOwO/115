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

// OathSystemInfo is the 24-byte S2C2839 envelope. Only its first u32 carries
// a confirmed value: the selected option (zero when level-gated).
func OathSystemInfo(level int, option int) ([]byte, error) {
	if option < 0 || option > 3 {
		return nil, fmt.Errorf("invalid oath option")
	}
	if level < 115 {
		option = 0
	} else if option == 0 {
		option = 1
	}
	body := make([]byte, 24)
	binary.LittleEndian.PutUint32(body[:4], uint32(option))
	return body, nil
}
