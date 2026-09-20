// Package protocol contains build-specific packet layouts, independent of
// persistence and game rules. Layout evidence is recorded under docs/protocol.
package protocol

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type CreateRequest struct {
	Profession byte
	Name       string
	// GrowthType is the advancement slot the naming window had selected,
	// carried in option byte 8. It indexes the character script's
	// [growtype N] sections as slot N-1.
	GrowthType byte
	Options    []byte
}

func parseName(p []byte) (string, int, error) {
	if len(p) < 4 {
		return "", 0, fmt.Errorf("missing name length")
	}
	n := int(binary.LittleEndian.Uint32(p))
	if n < 1 || n > 31 || n > len(p)-4 {
		return "", 0, fmt.Errorf("invalid name length")
	}
	name := string(p[4 : 4+n])
	if !utf8.ValidString(name) || strings.TrimSpace(name) != name {
		return "", 0, fmt.Errorf("invalid name encoding or surrounding spaces")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", 0, fmt.Errorf("control character in name")
		}
	}
	return name, 4 + n, nil
}
func padding(p []byte, alignment int) error {
	if len(p) >= alignment {
		return fmt.Errorf("excess packet tail")
	}
	for _, b := range p {
		if b != 0 {
			return fmt.Errorf("nonzero packet padding")
		}
	}
	return nil
}

// Native sender 0x1402396f0 writes one length-prefixed string (cipher slot 12).
func DecodeNameRequest(p []byte) (string, error) {
	n, k, e := parseName(p)
	if e != nil {
		return "", e
	}
	return n, padding(p[k:], 16)
}

// Native legacy sender 0x1402394b4 writes ten option bytes. The current
// naming window sender 0x141733fe7 writes twelve, including a selectable
// value at option 8 and two further bytes at 0x1417340ca/0x1417340d9.
func DecodeCreateRequest(p []byte) (CreateRequest, error) {
	var r CreateRequest
	if len(p) < 1 {
		return r, fmt.Errorf("missing profession")
	}
	r.Profession = p[0]
	n, k, e := parseName(p[1:])
	if e != nil {
		return r, e
	}
	r.Name = n
	k++
	if len(p) < k+10 {
		return r, fmt.Errorf("short creation options")
	}
	optionCount := 10
	if len(p)-k >= 12 {
		optionCount = 12
	}
	r.Options = append([]byte(nil), p[k:k+optionCount]...)
	suffix := [5]byte{0, 0, 0, 255, 0}
	for i, b := range suffix {
		if r.Options[i+3] != b {
			return r, fmt.Errorf("unsupported creation option layout")
		}
	}
	if optionCount >= 12 {
		r.GrowthType = r.Options[8]
	}
	return r, padding(p[k+optionCount:], 8)
}
func add16(p []byte, v uint16) []byte   { return binary.LittleEndian.AppendUint16(p, v) }
func add32(p []byte, v uint32) []byte   { return binary.LittleEndian.AppendUint32(p, v) }
func addName(p []byte, s string) []byte { return append(add32(p, uint32(len(s))), []byte(s)...) }

// The native create callback feeds this value into the roster-position lookup
// (0x1401f8a30 -> 0x14021adf0), not into a persistent character-ID lookup.
func CreateSuccess(slot uint16, name string) []byte { return addName(add16([]byte{1}, slot), name) }
func Refusal(code uint16) []byte                    { return add16([]byte{0}, code) }

type Equipment struct {
	Slot byte
	Item uint32
}

// 145639840 calls 1459a0220 with list46 for EVERY row, including weapons.
// That helper consumes a length-prefixed blob before the 26 fixed tail bytes.
func EquipmentAppearance(rows []Equipment) ([]byte, error) {
	if len(rows) > 48 {
		return nil, fmt.Errorf("too many appearance rows")
	}
	p := []byte{byte(len(rows))}
	seen := map[byte]bool{}
	for _, row := range rows {
		if row.Slot >= 48 || row.Item == 0 || seen[row.Slot] {
			return nil, fmt.Errorf("invalid appearance row")
		}
		seen[row.Slot] = true
		p = add32(append(p, row.Slot), row.Item)
		p = add32(p, 0)
		p = append(p, make([]byte, 26)...)
	}
	return p, nil
}

type CharacterRow struct {
	Odyssey          bool
	Slot             uint16
	Name             string
	Profession       byte
	Advancement      byte
	Level            byte
	Equipment        []Equipment
	FatigueRemaining uint16
	FatigueBonus     uint16
}

// Native list parser 0x145637a20, row parser 0x14563e280. Unknown scalar
// fields are zero in this experimental baseline, recorded as such in evidence.
func CharacterList(capacity uint16, roles []CharacterRow) ([]byte, error) {
	if capacity == 0 || len(roles) > int(capacity) {
		return nil, fmt.Errorf("invalid character capacity")
	}
	p := []byte{2, 0, 0}
	p = add16(p, capacity)
	p = add16(p, 0)
	p = add16(p, 0)
	p = add32(p, 0)
	p = add16(p, uint16(len(roles)))
	for index, r := range roles {
		// 0x1401f64d8 builds the native lookup from insertion positions. The
		// row key and create receipt must use that same zero-based position.
		if r.Slot != uint16(index) || r.Level == 0 {
			return nil, fmt.Errorf("invalid character row")
		}
		if _, _, e := parseName(addName(nil, r.Name)); e != nil {
			return nil, e
		}
		p = addName(add16(p, r.Slot), r.Name)
		p = append(p, 0, r.Profession, r.Advancement, r.Level, 0, 0)
		appearance, e := EquipmentAppearance(r.Equipment)
		if e != nil {
			return nil, e
		}
		p = append(p, appearance...)
		p = add32(p, 0)
		p = append(p, 0, 0, 0, 0)
		p = append(p, 0) // cosmetic helper 0x145639b70: zero count
		p = add32(p, 0)
		p = add32(p, 0)
		p = append(p, 0)
		p = add32(p, 0)
		p = append(p, 0)
		p = append(p, 0) // premium PC room helper 0x14563be50
		p = add32(p, 0)
		p = append(p, 1) // client default status bit field
		p = append(p, make([]byte, 8+28)...)
		p = add32(p, 0)
		p = add16(p, 0)
		// Native 14563e9ee stores this third byte at row-info+672.
		var mode byte
		if r.Odyssey {
			mode = 5
		}
		p = append(p, 0, 0, mode)
		for i := 0; i < 4; i++ {
			p = add32(p, 0)
		}
		// 14563ea6c/76 -> info+6d8/+6dc -> 140209be2 ->
		// 14020a04d renders these exact two values as "%d+%d".
		if r.FatigueRemaining > 32767 || r.FatigueBonus > 32767 {
			return nil, fmt.Errorf("roster fatigue exceeds native signed range")
		}
		p = add16(p, r.FatigueRemaining)
		p = add16(p, r.FatigueBonus)
		p = add32(p, 0)
	}
	p = append(p, 1, 0)
	p = add32(p, 0)
	p = add32(p, 0)
	return p, nil
}
