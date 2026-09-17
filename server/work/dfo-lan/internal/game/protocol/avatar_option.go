package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD451 sender145adeef0: u16/u32 consumable, u16 location, u16 slot,
// u32 template, u8 option. The initial free selection uses -1 consumables.
type AvatarOptionRequest struct {
	Location, Slot uint16
	Template       uint32
	Option         byte
}

func DecodeAvatarOption(p []byte) (AvatarOptionRequest, error) {
	var r AvatarOptionRequest
	if len(p) != 15 && len(p) != 16 {
		return r, fmt.Errorf("invalid avatar option length")
	}
	if len(p) == 16 && p[15] != 0 {
		return r, fmt.Errorf("invalid avatar option padding")
	}
	if binary.LittleEndian.Uint16(p) != 65535 || binary.LittleEndian.Uint32(p[2:]) != 0xffffffff {
		return r, fmt.Errorf("paid avatar reselection requires consumable handling")
	}
	r = AvatarOptionRequest{binary.LittleEndian.Uint16(p[6:]), binary.LittleEndian.Uint16(p[8:]), binary.LittleEndian.Uint32(p[10:]), p[14]}
	if r.Location != 2 || r.Template == 0 || r.Option == 0 || r.Option == 255 {
		return r, fmt.Errorf("unsupported initial avatar option selection")
	}
	return r, nil
}

// Native1413f8156 reads location/slot/option after the dispatcher success byte.
func AvatarOptionSuccess(r AvatarOptionRequest) []byte {
	p := binary.LittleEndian.AppendUint16([]byte{1}, r.Location)
	p = binary.LittleEndian.AppendUint16(p, r.Slot)
	return append(p, r.Option)
}
