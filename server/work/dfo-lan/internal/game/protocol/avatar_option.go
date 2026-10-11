package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD451 sender145adeef0: consumable slot/template, target location/slot,
// target template, and selected ability. The initial free selection uses
// 0xffff/0xffffffff as the consumable sentinel.
type AvatarOptionRequest struct {
	ConsumableSlot     uint16 `json:"consumable_slot,omitempty"`
	ConsumableTemplate uint32 `json:"consumable_template,omitempty"`
	Paid               bool   `json:"paid,omitempty"`
	Location, Slot     uint16
	Template           uint32
	Option             byte
}

func DecodeAvatarOption(p []byte) (AvatarOptionRequest, error) {
	var r AvatarOptionRequest
	if len(p) != 15 && len(p) != 16 {
		return r, fmt.Errorf("invalid avatar option length")
	}
	if len(p) == 16 && p[15] != 0 {
		return r, fmt.Errorf("invalid avatar option padding")
	}
	r.ConsumableSlot = binary.LittleEndian.Uint16(p)
	r.ConsumableTemplate = binary.LittleEndian.Uint32(p[2:])
	freeSentinel := r.ConsumableSlot == 0xffff && r.ConsumableTemplate == 0xffffffff
	if (r.ConsumableSlot == 0xffff) != (r.ConsumableTemplate == 0xffffffff) {
		return AvatarOptionRequest{}, fmt.Errorf("invalid avatar option consumable identity")
	}
	r.Paid = !freeSentinel
	r.Location = binary.LittleEndian.Uint16(p[6:])
	r.Slot = binary.LittleEndian.Uint16(p[8:])
	r.Template = binary.LittleEndian.Uint32(p[10:])
	r.Option = p[14]
	if r.Paid && (r.ConsumableSlot < 2 || r.ConsumableTemplate == 0 || r.Location != 0) {
		return AvatarOptionRequest{}, fmt.Errorf("unsupported paid avatar option request")
	}
	if !r.Paid && r.Location != 2 || r.Template == 0 || r.Option == 0 || r.Option == 255 {
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
