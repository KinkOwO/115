package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestPartyPeerEndpoint115NativeReaderVector(t *testing.T) {
	p, e := PartyPeerEndpoints115([]PartyPeerEndpoint115{{Actor: 65534, IP: [4]byte{127, 0, 0, 1}, Port: 35000, Key: 0x11223344, MTU: 512}})
	if e != nil || hex.EncodeToString(p) != "01feff7f0000017f00000188b844332211000002000000" {
		t.Fatalf("native N11 mismatch %x %v", p, e)
	}
}
func TestPartyConnection115CapturedAndMalformed(t *testing.T) {
	p, _ := hex.DecodeString("0000000000000000000000000000000000000004000000000000000000000000")
	r, e := DecodePartyConnectionCheck115(p)
	if e != nil || r.Mode != 0 || r.Slot != 0 || r.Mask != 4 {
		t.Fatal(r, e)
	}
	binary.LittleEndian.PutUint32(p[13:], 1)
	binary.LittleEndian.PutUint16(p[17:], 1)
	binary.LittleEndian.PutUint32(p[19:], 2)
	if r, e = DecodePartyConnectionCheck115(p); e != nil || r.Slot != 1 || r.Mask != 2 {
		t.Fatal(r, e)
	}
	p[23] = 1
	if _, e = DecodePartyConnectionCheck115(p); e == nil {
		t.Fatal("nonzero alignment")
	}
	if _, e = DecodePartyConnectionCheck115(p[:22]); e == nil {
		t.Fatal("short packet")
	}
	if _, e = PartyDirectConnectionResult115([]byte{0}); e == nil {
		t.Fatal("unproven peer")
	}
	p, e = PartyDirectConnectionResult115([]byte{2, 2, 2})
	if e != nil || len(p) != 17 || p[0] != 0 || p[3] != 2 || p[4] != 1 || p[7] != 2 || p[8] != 1 {
		t.Fatal("direct results", p, e)
	}
}
