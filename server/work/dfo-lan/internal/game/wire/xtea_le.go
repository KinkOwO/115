package wire

import (
	"crypto/cipher"
	"encoding/binary"
	"golang.org/x/crypto/xtea"
)

// Slot 8 uses little endian key/block words, unlike slot 0's big endian form.
// Native setup 0x146d9a0d0, encrypt 0x146d99f60, decrypt 0x146d99bc0.
type xteaLE struct{ cipher.Block }

func newXTEALE(key []byte) (cipher.Block, error) {
	if len(key) != 16 {
		return nil, xtea.KeySizeError(len(key))
	}
	var k [16]byte
	for i := 0; i < 16; i += 4 {
		binary.BigEndian.PutUint32(k[i:], binary.LittleEndian.Uint32(key[i:]))
	}
	b, e := xtea.NewCipher(k[:])
	if e != nil {
		return nil, e
	}
	return &xteaLE{b}, nil
}
func reverseWords(dst, src []byte) {
	for i := 0; i < 8; i += 4 {
		binary.BigEndian.PutUint32(dst[i:], binary.LittleEndian.Uint32(src[i:]))
	}
}
func (b *xteaLE) Encrypt(dst, src []byte) {
	var in, out [8]byte
	reverseWords(in[:], src)
	b.Block.Encrypt(out[:], in[:])
	reverseWords(dst, out[:])
}
func (b *xteaLE) Decrypt(dst, src []byte) {
	var in, out [8]byte
	reverseWords(in[:], src)
	b.Block.Decrypt(out[:], in[:])
	reverseWords(dst, out[:])
}
