package protocol

import (
	"encoding/binary"
	"fmt"
)

// DecodeSynopsisRead follows 140f0eeb0: a 17-byte stack buffer contains the
// synopsis ID at offset 13; its first 13 bytes are not initialized by that
// sender. Live requests are padded to 24 bytes. Do not interpret the prefix.
func DecodeSynopsisRead(p []byte) (uint32, error) {
	if len(p) != 24 {
		return 0, fmt.Errorf("synopsis request length: %d", len(p))
	}
	for _, b := range p[17:] {
		if b != 0 {
			return 0, fmt.Errorf("synopsis request padding")
		}
	}
	id := binary.LittleEndian.Uint32(p[13:17])
	if id > 0x7fffffff {
		return 0, fmt.Errorf("negative synopsis ID")
	}
	return id, nil
}

// SynopsisTableInfo is NOTI2310, registered at 14000b93f, reader 140f0e410:
// u8 flag (consumed but unused), signed LE i16 count, then LE i32 IDs.
// 140f0f7e0 replaces the entire read set and 140f0f000 rebuilds unread IDs.
// This is a full snapshot; sending only the newest ID would lose earlier reads.
func SynopsisTableInfo(ids []uint32) ([]byte, error) {
	if len(ids) > 32767 {
		return nil, fmt.Errorf("too many read synopsis IDs")
	}
	p := binary.LittleEndian.AppendUint16([]byte{0}, uint16(len(ids)))
	for _, id := range ids {
		if id > 0x7fffffff {
			return nil, fmt.Errorf("negative synopsis ID")
		}
		p = binary.LittleEndian.AppendUint32(p, id)
	}
	return p, nil
}
