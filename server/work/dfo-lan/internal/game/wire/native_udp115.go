package wire

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math/bits"
)

// Selected unreliable queue147082c10/14708c1d0 ->consumer147082100 ->
// 147081c90(mode0) ->14708a2a0(type2). Reader147089f20, sequence gate1470866f0.
// The selected path carries u32 sequence/checksum; legacy type0 is not used.
func EncodeNativeUDP115(slot byte, key, sequence uint32, payload []byte) ([]byte, error) {
	if slot >= 10 || len(payload) == 0 || len(payload) >= 32768 {
		return nil, fmt.Errorf("invalid native UDP payload")
	}
	p := make([]byte, 12+len(payload))
	p[0] = 2
	p[1] = slot
	binary.LittleEndian.PutUint32(p[2:], sequence)
	binary.LittleEndian.PutUint16(p[6:], uint16(len(payload)))
	copy(p[8:], payload)
	binary.LittleEndian.PutUint32(p[8+len(payload):], nativeUDPChecksum115(p[:8+len(payload)]))
	for i := 8; i < 8+len(payload); i++ {
		p[i] = bits.RotateLeft8(p[i]^byte(key), -int(key>>8&7))
	}
	return p, nil
}

// One native type2 control packet per datagram. Reliable type1, fragmentation
// and gameplay aggregation are outside this local handshake endpoint.
func DecodeNativeUDP115(p []byte, key uint32) (byte, uint32, []byte, error) {
	if len(p) < 13 || p[0] != 2 || p[1] >= 10 {
		return 0, 0, nil, fmt.Errorf("invalid native UDP header")
	}
	n := int(binary.LittleEndian.Uint16(p[6:]))
	if n == 0 || n >= 32768 || n+12 != len(p) {
		return 0, 0, nil, fmt.Errorf("invalid native UDP length")
	}
	plain := append([]byte(nil), p...)
	for i := 8; i < 8+n; i++ {
		plain[i] = bits.RotateLeft8(plain[i], int(key>>8&7)) ^ byte(key)
	}
	if nativeUDPChecksum115(plain[:8+n]) != binary.LittleEndian.Uint32(plain[8+n:]) {
		return 0, 0, nil, fmt.Errorf("invalid native UDP checksum")
	}
	return p[1], binary.LittleEndian.Uint32(p[2:]), plain[8 : 8+n], nil
}

func nativeUDPChecksum115(p []byte) uint32 {
	// 146d72770 retains the upper CRC bytes, replaces low byte via1401be260.
	c := crc32.Checksum(p, checksumTable)
	return c&0xffffff00 | uint32(byte(c^c>>8^c>>16^c>>24^0x18))
}
