package wire

import (
	"encoding/binary"
	"fmt"
	"golang.org/x/crypto/xtea"
	"hash/crc32"
)

// Native 0x146D77DB0 builds this table. This is deliberately not IEEE CRC32.
var checksumTable = crc32.MakeTable(0x4db89129)

func Checksum(b []byte) byte {
	c := crc32.Checksum(b, checksumTable)
	return byte(c ^ (c >> 8) ^ (c >> 16) ^ (c >> 24) ^ 0x18)
}

func ServerFrame(kind byte, id uint16, ciphertext []byte) ([]byte, error) {
	if kind > 1 || len(ciphertext) > MaxPacketSize-ServerHeaderSize {
		return nil, fmt.Errorf("invalid server packet")
	}
	raw := make([]byte, ServerHeaderSize+len(ciphertext))
	raw[0] = kind
	binary.LittleEndian.PutUint16(raw[1:3], id)
	binary.LittleEndian.PutUint32(raw[3:7], uint32(len(raw)))
	raw[11] = Checksum(ciphertext)
	copy(raw[16:], ciphertext)
	return raw, nil
}

// EncodeXTEA covers cipher slot 0 (packet ID modulo 14). The native wrapper
// pads to 16 bytes, then processes 8-byte XTEA blocks in ECB mode, 32 rounds.
func EncodeXTEA(key, payload []byte) ([]byte, error) {
	block, err := xtea.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padded := make([]byte, (len(payload)+15)/16*16)
	copy(padded, payload)
	result := make([]byte, len(padded))
	for off := 0; off < len(padded); off += block.BlockSize() {
		block.Encrypt(result[off:], padded[off:])
	}
	return result, nil
}
