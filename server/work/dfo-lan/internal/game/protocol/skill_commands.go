package protocol

import (
	"encoding/binary"
	"fmt"
)

// SkillCommands is the complete CMD331 customization snapshot. Each entry is
// a skill ID followed by one to five native command tokens.
type SkillCommands struct {
	Entries map[uint16][]uint32
	Raw     []byte
}

// DecodeSkillCommands reads CMD331. The decrypted frame is block padded with
// zeros, so Raw keeps only the count and entries that the client actually sent.
func DecodeSkillCommands(p []byte) (SkillCommands, error) {
	var out SkillCommands
	if len(p) == 0 {
		return out, fmt.Errorf("empty skill command snapshot")
	}
	count := int(p[0])
	pos := 1
	out.Entries = make(map[uint16][]uint32, count)
	for i := 0; i < count; i++ {
		if len(p)-pos < 3 {
			return SkillCommands{}, fmt.Errorf("short skill command entry")
		}
		id := binary.LittleEndian.Uint16(p[pos:])
		n := int(p[pos+2])
		pos += 3
		if id == 0 || n < 1 || n > 5 || len(p)-pos < n {
			return SkillCommands{}, fmt.Errorf("invalid skill command entry")
		}
		if _, exists := out.Entries[id]; exists {
			return SkillCommands{}, fmt.Errorf("duplicate skill command entry")
		}
		commands := make([]uint32, n)
		actions := 0
		for j := range commands {
			v := p[pos+j]
			if v == 7 || v > 12 {
				return SkillCommands{}, fmt.Errorf("invalid skill command token")
			}
			if v >= 4 && v <= 6 || v == 8 {
				actions++
			}
			commands[j] = uint32(v)
		}
		last := commands[n-1]
		if actions != 1 || !(last >= 4 && last <= 6 || last == 8) {
			return SkillCommands{}, fmt.Errorf("invalid skill command sequence")
		}
		out.Entries[id] = commands
		pos += n
	}
	for _, b := range p[pos:] {
		if b != 0 {
			return SkillCommands{}, fmt.Errorf("nonzero skill command padding")
		}
	}
	out.Raw = append([]byte(nil), p[:pos]...)
	return out, nil
}
