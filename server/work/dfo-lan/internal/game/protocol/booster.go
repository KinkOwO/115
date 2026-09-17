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
	p := add16([]byte{1}, 0)
	p = add32(p, 10417789)
	p = add16(p, r.Slot)
	p = add32(p, 0)
	p = add16(p, 1)
	p = add32(add32(p, r.Template), 1)
	return append(p, make([]byte, 7)...)
}
