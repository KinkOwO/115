package wire

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

// Native slot 6: setup146d93be0, FO146d93790, encrypt146d93a80.
type kasumiCipher struct{ kl1, kl2, ko1, ko2, ko3, ki1, ki2, ki3 [8]uint16 }

func newKasumi(key []byte) (*kasumiCipher, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("kasumi key length")
	}
	var k, p [8]uint16
	for i := range k {
		k[i] = binary.BigEndian.Uint16(key[i*2:])
		p[i] = k[i] ^ kasumiC[i]
	}
	c := new(kasumiCipher)
	for i := 0; i < 8; i++ {
		c.kl1[i] = bits.RotateLeft16(k[i], 1)
		c.kl2[i] = p[(i+2)&7]
		c.ko1[i] = bits.RotateLeft16(k[(i+1)&7], 5)
		c.ko2[i] = bits.RotateLeft16(k[(i+5)&7], 8)
		c.ko3[i] = bits.RotateLeft16(k[(i+6)&7], 13)
		c.ki1[i] = p[(i+4)&7]
		c.ki2[i] = p[(i+3)&7]
		c.ki3[i] = p[(i+7)&7]
	}
	return c, nil
}
func (*kasumiCipher) BlockSize() int { return 8 }
func kasumiFI(v, k uint16) uint16 {
	a, b := v>>7, v&127
	a = kasumiS9[a] ^ b
	b = kasumiS7[b] ^ (a & 127)
	a ^= k & 511
	b ^= k >> 9
	a = kasumiS9[a] ^ b
	b = kasumiS7[b] ^ (a & 127)
	return b<<9 | a
}
func (c *kasumiCipher) fl(v uint32, i int) uint32 {
	a, b := uint16(v>>16), uint16(v)
	b ^= bits.RotateLeft16(a&c.kl1[i], 1)
	a ^= bits.RotateLeft16(b|c.kl2[i], 1)
	return uint32(a)<<16 | uint32(b)
}
func (c *kasumiCipher) fo(v uint32, i int) uint32 {
	a, b := uint16(v>>16), uint16(v)
	a = kasumiFI(a^c.ko1[i], c.ki1[i]) ^ b
	b = kasumiFI(b^c.ko2[i], c.ki2[i]) ^ a
	a = kasumiFI(a^c.ko3[i], c.ki3[i]) ^ b
	return uint32(b)<<16 | uint32(a)
}
func (c *kasumiCipher) Encrypt(dst, src []byte) {
	a, b := binary.BigEndian.Uint32(src), binary.BigEndian.Uint32(src[4:])
	for i := 0; i < 8; i += 2 {
		b ^= c.fo(c.fl(a, i), i)
		a ^= c.fl(c.fo(b, i+1), i+1)
	}
	binary.BigEndian.PutUint32(dst, a)
	binary.BigEndian.PutUint32(dst[4:], b)
}
func (c *kasumiCipher) Decrypt(dst, src []byte) {
	a, b := binary.BigEndian.Uint32(src), binary.BigEndian.Uint32(src[4:])
	for i := 7; i >= 1; i -= 2 {
		a ^= c.fl(c.fo(b, i), i)
		b ^= c.fo(c.fl(a, i-1), i-1)
	}
	binary.BigEndian.PutUint32(dst, a)
	binary.BigEndian.PutUint32(dst[4:], b)
}
