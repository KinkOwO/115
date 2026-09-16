package wire

import (
	"encoding/binary"
	"fmt"
)

// Exact slot11: eight rounds, eight-byte blocks, sixteen-byte keys. The
// transport146ca9709 invokes vtable+0x18 ->146d8c650 ->146d945c0 (schedule0).
// Decode uses the transformed schedule+0x48. Live21 CMD39 checksum also
// verifies this direction; a roundtrip alone cannot distinguish the two.
type combatCipher struct{ forward, reverse [9]uint64 }

var combatTables = [8]*[256]uint64{&combatT0, &combatT1, &combatT2, &combatT3, &combatT4, &combatT5, &combatT6, &combatT7}

func combatMix(v uint64) (r uint64) {
	for i, t := range combatTables {
		r ^= t[byte(v>>uint(56-i*8))]
	}
	return
}
func combatSub(v uint64) (r uint64) {
	for i := 0; i < 8; i++ {
		r |= uint64(byte(combatT7[byte(v>>uint(56-i*8))])) << uint(56-i*8)
	}
	return
}
func newCombatCipher(key []byte) (*combatCipher, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("slot11 key must be16 bytes")
	}
	c := new(combatCipher)
	previous, current := binary.BigEndian.Uint64(key[:8]), binary.BigEndian.Uint64(key[8:])
	for r := 0; r < 9; r++ {
		var rc uint64
		for j := 0; j < 8; j++ {
			rc = rc<<8 | uint64(byte(combatT7[r*8+j]))
		}
		next := combatMix(current) ^ previous ^ rc
		c.forward[r] = next
		previous, current = current, next
	}
	c.reverse[0], c.reverse[8] = c.forward[8], c.forward[0]
	for r := 1; r < 8; r++ {
		c.reverse[r] = combatMix(combatSub(c.forward[8-r]))
	}
	return c, nil
}
func (*combatCipher) BlockSize() int { return 8 }
func combatBlock(dst, src []byte, key *[9]uint64) {
	v := binary.BigEndian.Uint64(src) ^ key[0]
	for r := 1; r < 8; r++ {
		v = combatMix(v) ^ key[r]
	}
	binary.BigEndian.PutUint64(dst, combatSub(v)^key[8])
}
func (c *combatCipher) Encrypt(dst, src []byte) { combatBlock(dst, src, &c.forward) }
func (c *combatCipher) Decrypt(dst, src []byte) { combatBlock(dst, src, &c.reverse) }
