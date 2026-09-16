package wire

import (
	"encoding/binary"
	"fmt"
)

// dfoXOR32 implements the current client's slot 10 transform. The native
// setup stores key[0:4]&31 but its copy/in-place transforms only XOR key[4:8].
// Native: setup 0x146d8dd00, copy 0x146d8dbb0, in-place 0x146d8da70.
type dfoXOR32 struct{ key uint32 }

func newDFOXOR32(key []byte) (*dfoXOR32, error) {
	if len(key) != 8 {
		return nil, fmt.Errorf("DFO XOR32 requires an 8-byte key")
	}
	return &dfoXOR32{key: binary.LittleEndian.Uint32(key[4:])}, nil
}

func (b *dfoXOR32) BlockSize() int { return 4 }
func (b *dfoXOR32) Encrypt(dst, src []byte) {
	binary.LittleEndian.PutUint32(dst, binary.LittleEndian.Uint32(src)^b.key)
}
func (b *dfoXOR32) Decrypt(dst, src []byte) { b.Encrypt(dst, src) }
