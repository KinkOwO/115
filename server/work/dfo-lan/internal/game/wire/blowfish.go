package wire

import (
	"encoding/binary"
	"fmt"
)

// This build changes the Blowfish expansion at 146d90f70 and 146d90fb0:
// only P[0:10] and the first 128 entries of each S-box are expanded.
// The block rounds and big-endian wire words are otherwise conventional.
type dfoBlowfish struct {
	p [18]uint32
	s [4][256]uint32
}

func newDFOBlowfish(key []byte) (*dfoBlowfish, error) {
	if len(key) != 56 {
		return nil, fmt.Errorf("DFO Blowfish key must have 56 bytes")
	}
	b := &dfoBlowfish{p: blowfishPiP, s: [4][256]uint32{blowfishPiS0, blowfishPiS1, blowfishPiS2, blowfishPiS3}}
	j := 0
	for i := range b.p {
		var w uint32
		for k := 0; k < 4; k++ {
			w = w<<8 | uint32(key[j])
			j = (j + 1) % len(key)
		}
		b.p[i] ^= w
	}
	var l, r uint32
	for i := 0; i < 10; i += 2 {
		l, r = b.encrypt(l, r)
		b.p[i], b.p[i+1] = l, r
	}
	for n := range b.s {
		for i := 0; i < 128; i += 2 {
			l, r = b.encrypt(l, r)
			b.s[n][i], b.s[n][i+1] = l, r
		}
	}
	return b, nil
}
func (b *dfoBlowfish) f(x uint32) uint32 {
	return ((b.s[0][x>>24] + b.s[1][byte(x>>16)]) ^ b.s[2][byte(x>>8)]) + b.s[3][byte(x)]
}
func (b *dfoBlowfish) encrypt(l, r uint32) (uint32, uint32) {
	for i := 0; i < 16; i++ {
		l ^= b.p[i]
		r ^= b.f(l)
		l, r = r, l
	}
	l, r = r, l
	return l ^ b.p[17], r ^ b.p[16]
}
func (b *dfoBlowfish) decrypt(l, r uint32) (uint32, uint32) {
	for i := 17; i > 1; i-- {
		l ^= b.p[i]
		r ^= b.f(l)
		l, r = r, l
	}
	l, r = r, l
	return l ^ b.p[0], r ^ b.p[1]
}
func (b *dfoBlowfish) BlockSize() int { return 8 }
func (b *dfoBlowfish) Encrypt(dst, src []byte) {
	l, r := b.encrypt(binary.BigEndian.Uint32(src), binary.BigEndian.Uint32(src[4:]))
	binary.BigEndian.PutUint32(dst, l)
	binary.BigEndian.PutUint32(dst[4:], r)
}
func (b *dfoBlowfish) Decrypt(dst, src []byte) {
	l, r := b.decrypt(binary.BigEndian.Uint32(src), binary.BigEndian.Uint32(src[4:]))
	binary.BigEndian.PutUint32(dst, l)
	binary.BigEndian.PutUint32(dst[4:], r)
}
