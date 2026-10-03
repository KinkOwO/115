package protocol

import (
	"encoding/binary"
	"fmt"
)

const KnightDeckSize = 5

// 14505DAB0 writes five DWORDs starting at byte zero. The transport pads
// CMD649 (alignment 8) to 24 bytes; padding is not a sixth field.
func DecodeKnightDeck(p []byte) ([KnightDeckSize]uint32, error) {
	var deck [KnightDeckSize]uint32
	if len(p) != 20 && len(p) != 24 {
		return deck, fmt.Errorf("knight deck requires 20 bytes or 24 with zero padding")
	}
	for _, v := range p[20:] {
		if v != 0 {
			return deck, fmt.Errorf("nonzero knight deck padding")
		}
	}
	for i := range deck {
		deck[i] = binary.LittleEndian.Uint32(p[i*4:])
	}
	return deck, nil
}

// The shared dispatcher consumes status and u16 error. Status zero makes
// 14524FCA0 return without writing the client's map or in-flight UI.
func KnightDeckAck() []byte { return []byte{0, 0, 0} }

// NOTI567 / 145310A90 reads exactly five DWORDs, without a status prefix.
func KnightDeckInfo(deck [KnightDeckSize]uint32) []byte {
	p := make([]byte, KnightDeckSize*4)
	for i, v := range deck {
		binary.LittleEndian.PutUint32(p[i*4:], v)
	}
	return p
}
