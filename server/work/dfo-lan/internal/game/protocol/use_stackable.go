package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD44 uses one stackable item from a bag slot.
//
// Recovered from this build, not ported: the client's own sender writes
// u16, u8, u32, u32, u32 through the calibrated field writers (u8 146d75cc0,
// u16 146d76180, u32 146d75ce0) after begin-command 146d746e0 with edx=44,
// giving a 15-byte body padded to 16. Its registered class1 handler
// 14529fd70 takes the success flag as an argument (test dl,dl) and then reads
// u16, u8, u32, u32 on the success path and u8, u32, u32 on the failure path.
//
// The same shape is documented in the supplied 90 reference as
// "i16 slot | u8 list | i32 instance | i32 item | u32 reserved", which is how
// the field names below are assigned; the widths and order are this client's.
type UseStackableRequest struct {
	Slot     uint16
	List     byte
	Instance uint32
	Template uint32
	Reserved uint32
}

func DecodeUseStackable(p []byte) (UseStackableRequest, error) {
	var r UseStackableRequest
	if len(p) != 16 {
		return r, fmt.Errorf("use stackable must contain15 bytes padded to16")
	}
	if e := padding(p[15:], 16); e != nil {
		return r, e
	}
	r.Slot = binary.LittleEndian.Uint16(p)
	r.List = p[2]
	r.Instance = binary.LittleEndian.Uint32(p[3:])
	r.Template = binary.LittleEndian.Uint32(p[7:])
	r.Reserved = binary.LittleEndian.Uint32(p[11:])
	if r.Template == 0 {
		return r, fmt.Errorf("use stackable without an item identity")
	}
	return r, nil
}

// UseStackableSuccess echoes the consumed slot back. Handler 14529fd70's
// success path reads u16 slot, u8 list, u32 instance, u32 item after the
// framework has consumed the success flag, so the body is that flag plus
// eleven bytes.
//
// The recovery effect itself is applied by the client: the native success path
// carries no amount or restored value, so the server owns the durable
// decrement and this acknowledgement, never an invented heal.
func UseStackableSuccess(r UseStackableRequest) ([]byte, error) {
	if r.Template == 0 {
		return nil, fmt.Errorf("invalid consumed item identity")
	}
	p := append([]byte{1}, byte(r.Slot), byte(r.Slot>>8), r.List)
	p = add32(p, r.Instance)
	return add32(p, r.Template), nil
}

// UseStackableRefused is the failure shape: handler 14529fd70's failure path
// reads u8 then two u32 after the flag. The reference names that order as
// list, item, instance — the client's read sequence alone cannot distinguish
// the two u32 fields, so this follows the reference and is the one field
// ordering here that a capture should still confirm.
func UseStackableRefused(r UseStackableRequest) []byte {
	p := append([]byte{0}, r.List)
	p = add32(p, r.Template)
	return add32(p, r.Instance)
}
