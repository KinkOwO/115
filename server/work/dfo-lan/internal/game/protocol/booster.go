package protocol

import (
	"encoding/binary"
	"fmt"
)

type WeaponBoxSelection struct {
	Slot     uint16
	Category [2]byte
	Template uint32
}

// Native14573dd2e and captured160:4200010000000000d8c2050600000000.
// One box, one chosen template, empty optional selections and slot1 padding.
func DecodeWeaponBoxSelection(p []byte) (WeaponBoxSelection, error) {
	var r WeaponBoxSelection
	if len(p) != 16 || binary.LittleEndian.Uint32(p[2:]) != 1 {
		return r, fmt.Errorf("weapon selection requires one box and 16 bytes")
	}
	for _, v := range p[12:] {
		if v != 0 {
			return r, fmt.Errorf("unsupported weapon selection extras")
		}
	}
	r.Slot = binary.LittleEndian.Uint16(p)
	r.Category = [2]byte{p[6], p[7]}
	r.Template = binary.LittleEndian.Uint32(p[8:])
	if r.Slot == 0 || r.Template == 0 {
		return r, fmt.Errorf("empty selection identity")
	}
	return r, nil
}

// CMD160 native14529cda0: success, error, box ID, box slot, auxiliary
// count, result count, then15 bytes per result. Fresh gear has zero bonuses.
func WeaponBoxSuccess(r WeaponBoxSelection) []byte {
	return BoosterOpenSuccess(10417789, r.Slot, []BoosterGrantedItem{{Template: r.Template, Count: 1}})
}

type BoosterGrantedItem struct {
	Template uint32
	Count    uint32
}

type BoosterUseRequest struct {
	Slot       uint16
	Amount     uint32
	Category   uint16
	Selections []uint32
}

// DecodeBoosterUseRequest decodes CMD160 payloads across both simple booster (8 bytes)
// and selectable boosters (16, 48, or N bytes).
func DecodeBoosterUseRequest(p []byte) (BoosterUseRequest, error) {
	if len(p) < 8 {
		return BoosterUseRequest{}, fmt.Errorf("booster request requires at least 8 bytes, got %d", len(p))
	}
	r := BoosterUseRequest{
		Slot:     binary.LittleEndian.Uint16(p[0:2]),
		Amount:   binary.LittleEndian.Uint32(p[2:6]),
		Category: binary.LittleEndian.Uint16(p[6:8]),
	}
	if r.Amount == 0 {
		r.Amount = 1
	}
	for off := 8; off+4 <= len(p); off += 4 {
		tpl := binary.LittleEndian.Uint32(p[off : off+4])
		if tpl > 0 {
			r.Selections = append(r.Selections, tpl)
		}
	}
	if r.Slot == 0 {
		return r, fmt.Errorf("empty booster slot")
	}
	return r, nil
}

// BoosterOpenSuccess encodes the ACK 160 response per native 14529cda0:
// success byte 1, error u16 0, box template u32, box slot u16, auxiliary u32 0,
// result count u16 N, then 15 bytes per result (template u32, count u32, 7 zeros).
func BoosterOpenSuccess(boxTemplate uint32, boxSlot uint16, granted []BoosterGrantedItem) []byte {
	p := add16([]byte{1}, 0)
	p = add32(p, boxTemplate)
	p = add16(p, boxSlot)
	p = add32(p, 0)
	p = add16(p, uint16(len(granted)))
	for _, g := range granted {
		p = add32(p, g.Template)
		p = add32(p, g.Count)
		p = append(p, make([]byte, 7)...)
	}
	return p
}
