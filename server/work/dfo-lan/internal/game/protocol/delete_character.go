package protocol

import "fmt"

type DeleteCharacterRequest struct {
	Slot uint16
	Name string
}

// 1467bb1a0 writes u16 roster slot followed by the confirmed character name.
// 146d76080 converts the wide string and writes u32 byte length plus bytes.
func DecodeDeleteCharacter(p []byte) (DeleteCharacterRequest, error) {
	var r DeleteCharacterRequest
	if len(p) < 6 {
		return r, fmt.Errorf("short delete request")
	}
	r.Slot = uint16(p[0]) | uint16(p[1])<<8
	n, k, e := parseName(p[2:])
	if e != nil {
		return r, e
	}
	r.Name = n
	return r, padding(p[k+2:], 16)
}

// 1452519d0 reads flag then slot, removes the roster item, and clears both
// busy flags via 14023dfd0/14023dfc0. Dispatcher consumes success/error first.
func DeleteCharacterSuccess(slot uint16) []byte { return add16([]byte{1, 0}, slot) }
