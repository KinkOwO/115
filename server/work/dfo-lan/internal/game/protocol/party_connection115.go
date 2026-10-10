package protocol

import (
	"encoding/binary"
	"fmt"
)

// N11 reader145312e80 -> character+7b8 and character+74. IP octets and
// port are network order; the native reader swaps the port into host order.
type PartyPeerEndpoint115 struct {
	Actor uint16
	IP    [4]byte
	Port  uint16
	Key   uint32
	MTU   uint32
}

func PartyPeerEndpoints115(peers []PartyPeerEndpoint115) ([]byte, error) {
	if len(peers) == 0 || len(peers) > 3 {
		return nil, fmt.Errorf("invalid party endpoints")
	}
	p := make([]byte, 1+22*len(peers))
	p[0] = byte(len(peers))
	for i, r := range peers {
		if r.Actor == 0 || r.Actor == 65535 || r.Port == 0 || r.Key == 0 || r.MTU < 512 {
			return nil, fmt.Errorf("invalid party endpoint")
		}
		at := 1 + 22*i
		binary.LittleEndian.PutUint16(p[at:], r.Actor)
		copy(p[at+2:at+6], r.IP[:])
		copy(p[at+6:at+10], r.IP[:])
		binary.BigEndian.PutUint16(p[at+10:], r.Port)
		binary.LittleEndian.PutUint32(p[at+12:], r.Key)
		binary.LittleEndian.PutUint32(p[at+17:], r.MTU)
	}
	return p, nil
}

type PartyConnectionCheck115 struct {
	Mode uint32
	Slot uint16
	Mask uint32
}

// 1466630d0 and UDP receive branch14663f24b emit23 bytes. The first13
// bytes are opaque stack context; do not interpret them as authorization.
func DecodePartyConnectionCheck115(p []byte) (PartyConnectionCheck115, error) {
	var r PartyConnectionCheck115
	if len(p) < 23 || len(p) > 32 {
		return r, fmt.Errorf("invalid connection check length")
	}
	for _, b := range p[23:] {
		if b != 0 {
			return r, fmt.Errorf("invalid connection check alignment")
		}
	}
	r.Mode = binary.LittleEndian.Uint32(p[13:])
	r.Slot = binary.LittleEndian.Uint16(p[17:])
	r.Mask = binary.LittleEndian.Uint32(p[19:])
	if r.Mode > 1 || r.Slot >= 8 || r.Mask == 0 || r.Mask&^uint32(7) != 0 {
		return r, fmt.Errorf("invalid connection check")
	}
	return r, nil
}

// N2373 reader1452db650: relay mask followed by8 two-byte rows.
// Each populated row requires actual bidirectional UDP evidence at the caller.
// Relay mask remains zero: this local service provides direct UDP endpoints.
func PartyDirectConnectionResult115(masks []byte) ([]byte, error) {
	if len(masks) == 0 || len(masks) > 3 {
		return nil, fmt.Errorf("invalid connection results")
	}
	p := make([]byte, 17)
	for i, m := range masks {
		if m == 0 || m&^byte(3) != 0 {
			return nil, fmt.Errorf("missing direct connection evidence")
		}
		p[1+2*(i+1)] = m
		p[2+2*(i+1)] = 1
	}
	return p, nil
}

func PartyCheckConnectUDP115(slot, kind, address byte, actor uint16) ([]byte, error) {
	if slot >= 4 || kind > 4 || address > 1 || actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid direct party check")
	}
	p := []byte{0, 0, 0, slot, kind, address, 0, 0}
	binary.LittleEndian.PutUint16(p[6:], actor)
	return p, nil
}
