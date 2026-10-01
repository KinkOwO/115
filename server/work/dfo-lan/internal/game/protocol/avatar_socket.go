package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD206 sender 1467AF530 writes the avatar-list slot, its template, then
// the ordinary-bag device slot. The body is exactly one eight-byte block.
type AddAvatarSocketRequest struct {
	AvatarSlot uint16 `json:"avatar_slot"`
	Template   uint32 `json:"template"`
	DeviceSlot uint16 `json:"device_slot"`
}

func DecodeAddAvatarSocket(p []byte) (AddAvatarSocketRequest, error) {
	var r AddAvatarSocketRequest
	if len(p) != 8 {
		return r, fmt.Errorf("invalid avatar socket request length")
	}
	r.AvatarSlot = binary.LittleEndian.Uint16(p)
	r.Template = binary.LittleEndian.Uint32(p[2:])
	r.DeviceSlot = binary.LittleEndian.Uint16(p[6:])
	if r.AvatarSlot == 65535 || r.Template == 0 || r.DeviceSlot < 2 || r.DeviceSlot == 65535 {
		return AddAvatarSocketRequest{}, fmt.Errorf("invalid avatar socket request")
	}
	return r, nil
}

// Reader 145264AF0 consumes only these two slots after the dispatch success
// byte. It decrements the device itself and opens the avatar result window.
func AddAvatarSocketSuccess(r AddAvatarSocketRequest) ([]byte, error) {
	if r.AvatarSlot == 65535 || r.Template == 0 || r.DeviceSlot < 2 || r.DeviceSlot == 65535 {
		return nil, fmt.Errorf("invalid avatar socket result")
	}
	return add16(add16([]byte{1}, r.AvatarSlot), r.DeviceSlot), nil
}
