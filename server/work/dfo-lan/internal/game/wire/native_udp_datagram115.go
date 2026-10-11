package wire

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

// Native readers147089e60/147089f20 and datagram loop1470810e0.
// Fragmented reliable frames are deliberately rejected until reassembly is implemented.
type NativeUDPFrame115 struct {
	Type     byte
	Slot     byte
	Sequence uint32
	Payload  []byte
}

func DecodeNativeUDPDatagram115(p []byte, key uint32) ([]NativeUDPFrame115, error) {
	if len(p) == 0 || len(p) > 4096 {
		return nil, fmt.Errorf("native UDP datagram bound")
	}
	var out []NativeUDPFrame115
	for len(p) > 0 {
		if len(out) >= 64 || len(p) < 4 || p[1] >= 10 {
			return nil, fmt.Errorf("native UDP frame bound")
		}
		typ, h, n, tail := p[0], 4, int(binary.LittleEndian.Uint16(p[2:])), 2
		seq := uint32(0)
		switch typ {
		case 0:
		case 1, 2:
			if len(p) < 8 {
				return nil, fmt.Errorf("short native UDP header")
			}
			h, n, tail = 8, int(binary.LittleEndian.Uint16(p[6:])), 4
			seq = binary.LittleEndian.Uint32(p[2:])
			if typ == 1 {
				h = 9
				if len(p) < 9 || p[8] != 0 {
					return nil, fmt.Errorf("unsupported native UDP fragment")
				}
			}
		default:
			return nil, fmt.Errorf("unknown native UDP frame")
		}
		if n < 1 || n >= 32768 || h+n+tail > len(p) {
			return nil, fmt.Errorf("short native UDP payload")
		}
		plain := append([]byte(nil), p[:h+n]...)
		for i := h; i < h+n; i++ {
			plain[i] = bits.RotateLeft8(plain[i], int(key>>8&7)) ^ byte(key)
		}
		got, want := nativeUDPChecksum115(plain), binary.LittleEndian.Uint16(p[h+n:])
		if (tail == 2 && uint16(got) != want) || (tail == 4 && got != binary.LittleEndian.Uint32(p[h+n:])) {
			return nil, fmt.Errorf("native UDP checksum")
		}
		out = append(out, NativeUDPFrame115{typ, p[1], seq, plain[h:]})
		p = p[h+n+tail:]
	}
	return out, nil
}

func EncodeNativeUDPReliable115(slot byte, key, sequence uint32, payload []byte) ([]byte, error) {
	if slot >= 10 || len(payload) == 0 || len(payload) > 4000 {
		return nil, fmt.Errorf("reliable UDP bound")
	}
	p := make([]byte, 13+len(payload))
	p[0] = 1
	p[1] = slot
	binary.LittleEndian.PutUint32(p[2:], sequence)
	binary.LittleEndian.PutUint16(p[6:], uint16(len(payload)))
	copy(p[9:], payload)
	binary.LittleEndian.PutUint32(p[9+len(payload):], nativeUDPChecksum115(p[:9+len(payload)]))
	for i := 9; i < 9+len(payload); i++ {
		p[i] = bits.RotateLeft8(p[i]^byte(key), -int(key>>8&7))
	}
	return p, nil
}

// Cumulative ACK payload used by1470863d0; count0 requires the native spare byte.
func EncodeNativeUDPAck115(slot byte, key, next uint32) ([]byte, error) {
	if slot >= 10 {
		return nil, fmt.Errorf("ACK slot")
	}
	p := make([]byte, 12)
	p[1] = slot
	binary.LittleEndian.PutUint16(p[2:], 6)
	binary.LittleEndian.PutUint32(p[4:], next)
	binary.LittleEndian.PutUint16(p[10:], uint16(nativeUDPChecksum115(p[:10])))
	for i := 4; i < 10; i++ {
		p[i] = bits.RotateLeft8(p[i]^byte(key), -int(key>>8&7))
	}
	return p, nil
}

// Native14707ec80 appends u16 lengths before each reliable game message.
func SplitNativeUDPReliable115(p []byte) ([][]byte, error) {
	var out [][]byte
	for len(p) > 0 {
		if len(p) < 2 || len(out) >= 256 {
			return nil, fmt.Errorf("UDP bundle bound")
		}
		n := int(binary.LittleEndian.Uint16(p))
		p = p[2:]
		if n < 3 || n > len(p) {
			return nil, fmt.Errorf("UDP bundle length")
		}
		out = append(out, append([]byte(nil), p[:n]...))
		p = p[n:]
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty UDP bundle")
	}
	return out, nil
}
