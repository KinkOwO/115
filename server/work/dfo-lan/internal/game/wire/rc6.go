package wire

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

// dfoRC6 is the supplied build's RC6 variant. The schedule stores 44 bytes,
// not 44 words; the wrapper consumes 60 key bytes but initializes from 32.
// Native setup/encrypt/decrypt: 146D97410, 146D97340, 146D97250.
type dfoRC6 struct{ s [44]byte }

func newDFORC6(key []byte) (*dfoRC6, error) {
	if len(key) != 60 {
		return nil, fmt.Errorf("DFO RC6 requires 60 session key bytes")
	}
	c := &dfoRC6{}
	var l [8]uint32
	for i := range l {
		l[i] = binary.LittleEndian.Uint32(key[i*4:])
	}
	c.s[0] = 0x63
	for i := 1; i < len(c.s); i++ {
		c.s[i] = c.s[i-1] + 0xb9
	}
	var a, b uint32
	for k := 0; k < 132; k++ {
		i, j := k%44, k%8
		a = uint32(byte(bits.RotateLeft32(uint32(c.s[i])+a+b, 3)))
		c.s[i] = byte(a)
		b = bits.RotateLeft32(l[j]+a+b, int((a+b)&31))
		l[j] = b
	}
	return c, nil
}
func (c *dfoRC6) BlockSize() int { return 16 }
func (c *dfoRC6) Encrypt(dst, src []byte) {
	a, b, d, e := binary.LittleEndian.Uint32(src), binary.LittleEndian.Uint32(src[4:]), binary.LittleEndian.Uint32(src[8:]), binary.LittleEndian.Uint32(src[12:])
	b += uint32(c.s[0])
	e += uint32(c.s[1])
	for i := 1; i <= 20; i++ {
		t, u := bits.RotateLeft32(b*(2*b+1), 5), bits.RotateLeft32(e*(2*e+1), 5)
		a = bits.RotateLeft32(a^t, int(u&31)) + uint32(c.s[2*i])
		d = bits.RotateLeft32(d^u, int(t&31)) + uint32(c.s[2*i+1])
		a, b, d, e = b, d, e, a
	}
	a += uint32(c.s[42])
	d += uint32(c.s[43])
	for i, v := range []uint32{a, b, d, e} {
		binary.LittleEndian.PutUint32(dst[i*4:], v)
	}
}
func (c *dfoRC6) Decrypt(dst, src []byte) {
	a, b, d, e := binary.LittleEndian.Uint32(src), binary.LittleEndian.Uint32(src[4:]), binary.LittleEndian.Uint32(src[8:]), binary.LittleEndian.Uint32(src[12:])
	a -= uint32(c.s[42])
	d -= uint32(c.s[43])
	for i := 20; i >= 1; i-- {
		a, b, d, e = e, a, b, d
		t, u := bits.RotateLeft32(b*(2*b+1), 5), bits.RotateLeft32(e*(2*e+1), 5)
		d = bits.RotateLeft32(d-uint32(c.s[2*i+1]), -int(t&31)) ^ u
		a = bits.RotateLeft32(a-uint32(c.s[2*i]), -int(u&31)) ^ t
	}
	b -= uint32(c.s[0])
	e -= uint32(c.s[1])
	for i, v := range []uint32{a, b, d, e} {
		binary.LittleEndian.PutUint32(dst[i*4:], v)
	}
}
