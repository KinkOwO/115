package protocol

import "fmt"

// CharacterCapacityUpdated is NOTI66 subtype8. Current PE registration at
// 1452f96e8 binds handler1452c6e00, which reads u16 subtype, u8 capacity and
// calls140239a10 to update the roster capacity and purchased-slot count.
func CharacterCapacityUpdated(capacity uint16) ([]byte, error) {
	if capacity == 0 || capacity > 255 {
		return nil, fmt.Errorf("character capacity outside native byte range")
	}
	return []byte{8, 0, byte(capacity)}, nil
}

// CharacterSlotTicketRefused includes the standard class1 failure code before
// the use handler's u8 list/u32 instance/u32 item body. Handler14529fd70 uses
// the second u32 at1452a0814 as the item identity. Code17 skips its specialized
// dialogs. The old generic UseStackableRefused omits the framework's error code;
// this verified shape is scoped to this coupon, pending manual acceptance.
func CharacterSlotTicketRefused(r UseStackableRequest) []byte {
	p := append(Refusal(17), r.List)
	p = add32(p, r.Instance)
	return add32(p, r.Template)
}
