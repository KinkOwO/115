package protocol

import (
	"encoding/binary"
	"fmt"
)

// Current145af7135..145af71eb writes29 bytes, padded to32; additional
// post-target fields are retained and validated before ordinary moves.
type ItemMoveRequest struct {
	SourceList, DestinationList        byte
	SourceSlot, DestinationSlot        uint16
	SourceItem, DestinationItem, Count uint32
	Extra                              uint32
	Selection                          uint32
	Flags                              [3]byte
}

func DecodeItemMove(p []byte) (ItemMoveRequest, error) {
	var r ItemMoveRequest
	if len(p) != 29 && len(p) != 32 {
		return r, fmt.Errorf("item move requires current29-byte body")
	}
	for _, b := range p[29:] {
		if b != 0 {
			return r, fmt.Errorf("nonzero item move padding")
		}
	}
	r.SourceList = p[0]
	r.SourceSlot = binary.LittleEndian.Uint16(p[1:])
	r.SourceItem = binary.LittleEndian.Uint32(p[3:])
	r.Count = binary.LittleEndian.Uint32(p[7:])
	r.DestinationList = p[11]
	r.DestinationSlot = binary.LittleEndian.Uint16(p[12:])
	r.DestinationItem = binary.LittleEndian.Uint32(p[14:])
	r.Extra = binary.LittleEndian.Uint32(p[18:])
	r.Selection = binary.LittleEndian.Uint32(p[22:])
	copy(r.Flags[:], p[26:])
	return r, nil
}

func ItemMoveSuccess(r ItemMoveRequest, count uint32) []byte {
	p := add32(add16([]byte{1, r.SourceList}, r.SourceSlot), count)
	return append(add16(append(p, r.DestinationList), r.DestinationSlot), 0)
}

func ItemMoveRefused(r ItemMoveRequest, code uint16) []byte {
	p := append(Refusal(code), r.SourceList, r.DestinationList)
	return append(add32(p, 0), 0)
}
