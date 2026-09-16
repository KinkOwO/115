package wire

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

// Live command 433 establishes slot-13 direction: decryption 0x146d96330,
// encryption 0x146d964c0. Vtable order alone does not establish direction.
// Key schedule: 0x146d96670; fixed wrapper rounds: 128.
type dfoMulti2 struct{ key [8]uint32 }

func m2p2(x, k uint32) uint32 {
	y := x + k
	z := bits.RotateLeft32(y, 1) + y - 1
	return bits.RotateLeft32(z, 4) ^ z
}
func m2p3(x, k1, k2 uint32) uint32 {
	y := x + k1
	z := bits.RotateLeft32(y, 2) + y + 1
	a := (bits.RotateLeft32(z, 8) ^ z) + k2
	b := bits.RotateLeft32(a, 1) - a
	return bits.RotateLeft32(b, 16) ^ (b | x)
}
func m2p4(x, k uint32) uint32 {
	y := x + k
	return bits.RotateLeft32(y, 2) + y + 1
}
func newDFOMulti2(raw []byte) (*dfoMulti2, error) {
	if len(raw) != 40 {
		return nil, fmt.Errorf("MULTI2 key must have 40 bytes")
	}
	var system [8]uint32
	for i := range system {
		system[i] = binary.BigEndian.Uint32(raw[i*4:])
	}
	left, right := binary.BigEndian.Uint32(raw[32:]), binary.BigEndian.Uint32(raw[36:])
	b := new(dfoMulti2)
	for i := 0; i < 8; i += 4 {
		right ^= left
		left ^= m2p2(right, system[i])
		b.key[i] = left
		right ^= m2p3(left, system[i+1], system[i+2])
		b.key[i+1] = right
		left ^= m2p4(right, system[i+3])
		b.key[i+2] = left
		b.key[i+3] = left ^ right
	}
	return b, nil
}
func (*dfoMulti2) BlockSize() int { return 8 }
func (b *dfoMulti2) Decrypt(dst, src []byte) {
	left, right := binary.BigEndian.Uint32(src), binary.BigEndian.Uint32(src[4:])
	for group := 31; group >= 0; group-- {
		i := (group & 1) * 4
		left ^= m2p4(right, b.key[i+3])
		right ^= m2p3(left, b.key[i+1], b.key[i+2])
		left ^= m2p2(right, b.key[i])
		right ^= left
	}
	binary.BigEndian.PutUint32(dst, left)
	binary.BigEndian.PutUint32(dst[4:], right)
}
func (b *dfoMulti2) Encrypt(dst, src []byte) {
	left, right := binary.BigEndian.Uint32(src), binary.BigEndian.Uint32(src[4:])
	for group := 0; group < 32; group++ {
		i := (group & 1) * 4
		right ^= left
		left ^= m2p2(right, b.key[i])
		right ^= m2p3(left, b.key[i+1], b.key[i+2])
		left ^= m2p4(right, b.key[i+3])
	}
	binary.BigEndian.PutUint32(dst, left)
	binary.BigEndian.PutUint32(dst[4:], right)
}
