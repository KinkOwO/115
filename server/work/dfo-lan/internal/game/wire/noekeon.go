package wire

// Noekeon operations derive from the public-domain LibTomCrypt reference (Unlicense):
// https://github.com/libtom/libtomcrypt/blob/develop/src/ciphers/noekeon.c
// This DFO build rotates word 0 (rather than standard Noekeon's word 1) in pi.
// Compatibility is checked against vectors executed from this exact DFO build.
import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

type noekeon struct {
	key     [4]uint32
	inverse [4]uint32
}

var noekeonRC = [17]uint32{0x80, 0x1b, 0x36, 0x6c, 0xd8, 0xab, 0x4d, 0x9a, 0x2f, 0x5e, 0xbc, 0x63, 0xc6, 0x97, 0x35, 0x6a, 0xd4}

func newNoekeon(key []byte) (*noekeon, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("Noekeon key must be 16 bytes")
	}
	c := &noekeon{}
	for i := range c.key {
		c.key[i] = binary.BigEndian.Uint32(key[i*4:])
	}
	c.inverse = c.key
	theta(&c.inverse, [4]uint32{})
	return c, nil
}
func theta(a *[4]uint32, key [4]uint32) {
	t := a[0] ^ a[2]
	t ^= bits.RotateLeft32(t, 8) ^ bits.RotateLeft32(t, -8)
	a[1] ^= t ^ key[1]
	a[3] ^= t ^ key[3]
	t = a[1] ^ a[3]
	t ^= bits.RotateLeft32(t, 8) ^ bits.RotateLeft32(t, -8)
	a[0] ^= t ^ key[0]
	a[2] ^= t ^ key[2]
}
func noekeonRound(a *[4]uint32) {
	a[0] = bits.RotateLeft32(a[0], 1)
	a[2] = bits.RotateLeft32(a[2], 5)
	a[3] = bits.RotateLeft32(a[3], 2)
	a[1] ^= ^(a[3] | a[2])
	a[0] ^= a[2] & a[1]
	a[0], a[3] = a[3], a[0]
	a[2] ^= a[0] ^ a[1] ^ a[3]
	a[1] ^= ^(a[3] | a[2])
	a[0] ^= a[2] & a[1]
	a[0] = bits.RotateLeft32(a[0], -1)
	a[2] = bits.RotateLeft32(a[2], -5)
	a[3] = bits.RotateLeft32(a[3], -2)
}
func (c *noekeon) BlockSize() int { return 16 }
func (c *noekeon) Encrypt(dst, src []byte) {
	var a [4]uint32
	for i := range a {
		a[i] = binary.BigEndian.Uint32(src[4*i:])
	}
	for r := 0; r < 16; r++ {
		a[0] ^= noekeonRC[r]
		theta(&a, c.key)
		noekeonRound(&a)
	}
	a[0] ^= noekeonRC[16]
	theta(&a, c.key)
	for i, v := range a {
		binary.BigEndian.PutUint32(dst[4*i:], v)
	}
}
func (c *noekeon) Decrypt(dst, src []byte) {
	var a [4]uint32
	for i := range a {
		a[i] = binary.BigEndian.Uint32(src[4*i:])
	}
	for r := 16; r > 0; r-- {
		theta(&a, c.inverse)
		a[0] ^= noekeonRC[r]
		noekeonRound(&a)
	}
	theta(&a, c.inverse)
	a[0] ^= noekeonRC[0]
	for i, v := range a {
		binary.BigEndian.PutUint32(dst[4*i:], v)
	}
}
