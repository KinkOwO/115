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
func WeaponBoxSuccess(boxTemplate uint32, r WeaponBoxSelection) []byte {
	return BoosterOpenSuccess(boxTemplate, r.Slot, []BoosterGrantedItem{{Template: r.Template, Count: 1}})
}

type BoosterGrantedItem struct {
	Template uint32
	Count    uint32
}

type AvatarOptionSelection struct {
	Template uint32
	Option   byte
}

type BoosterUseRequest struct {
	Slot          uint16
	Amount        uint32
	Category      uint16
	Selections    []uint32
	AvatarOptions []AvatarOptionSelection
}

// DecodeBoosterUseRequest decodes CMD160 payloads across simple boosters, selectable boosters,
// and avatar boxes with selectable abilities per client sub_14573DC60 / sub_14573E120.
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
	if r.Slot == 0 {
		return r, fmt.Errorf("empty booster slot")
	}

	data := p[8:]
	if len(data) == 0 {
		return r, nil
	}

	// Try to match native wire format from client sub_14573DC60 / sub_14573E120:
	// data contains:
	//   S * uint32 selection templates
	//   1 byte avatarCount A
	//   A * (uint32 template, uint8 option) [5 bytes per entry]
	//   1 byte trailing 0
	//   padding zeros to block size
	found := false
	for s := 0; s <= len(data)/4; s++ {
		off := 4 * s
		if off >= len(data) {
			break
		}
		a := int(data[off])
		if a == 0 {
			if off+1 < len(data) && data[off+1] == 0 {
				allZero := true
				for _, b := range data[off+2:] {
					if b != 0 {
						allZero = false
						break
					}
				}
				if allZero {
					for i := 0; i < s; i++ {
						tpl := binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
						if tpl > 0 {
							r.Selections = append(r.Selections, tpl)
						}
					}
					found = true
					break
				}
			}
		} else if a > 0 && a <= 50 {
			endOff := off + 1 + a*5
			if endOff < len(data) && data[endOff] == 0 {
				allZero := true
				for _, b := range data[endOff+1:] {
					if b != 0 {
						allZero = false
						break
					}
				}
				// Native 14573DC60 writes ability options for the selected avatar
				// templates. Validate that identity before accepting a boundary:
				// a template's low byte can also look like an option count (the
				// captured archer package's fourth template starts with 04).
				if allZero && s > 0 {
					for i := 0; i < a; i++ {
						entryOff := off + 1 + i*5
						tpl := binary.LittleEndian.Uint32(data[entryOff : entryOff+4])
						selected := false
						for j := 0; j < s; j++ {
							if tpl != 0 && tpl == binary.LittleEndian.Uint32(data[j*4:j*4+4]) {
								selected = true
								break
							}
						}
						if !selected {
							allZero = false
							break
						}
					}
				}
				if allZero {
					for i := 0; i < s; i++ {
						tpl := binary.LittleEndian.Uint32(data[i*4 : (i+1)*4])
						if tpl > 0 {
							r.Selections = append(r.Selections, tpl)
						}
					}
					for i := 0; i < a; i++ {
						entryOff := off + 1 + i*5
						tpl := binary.LittleEndian.Uint32(data[entryOff : entryOff+4])
						opt := data[entryOff+4]
						r.AvatarOptions = append(r.AvatarOptions, AvatarOptionSelection{
							Template: tpl,
							Option:   opt,
						})
					}
					found = true
					break
				}
			}
		}
	}

	if !found {
		// Fallback: decode as a flat list of uint32 templates
		for off := 0; off+4 <= len(data); off += 4 {
			tpl := binary.LittleEndian.Uint32(data[off : off+4])
			if tpl > 0 {
				r.Selections = append(r.Selections, tpl)
			}
		}
	}

	// Safety fallback: if no Selections but AvatarOptions were parsed,
	// populate Selections from AvatarOptions so items are granted
	if len(r.Selections) == 0 && len(r.AvatarOptions) > 0 {
		for _, ao := range r.AvatarOptions {
			r.Selections = append(r.Selections, ao.Template)
		}
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
