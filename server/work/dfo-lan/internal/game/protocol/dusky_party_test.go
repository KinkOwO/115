package protocol

import (
	"encoding/binary"
	"testing"
)

func TestDuskyPartyNativeIdentity(t *testing.T) {
	p := make([]byte, 40)
	binary.LittleEndian.PutUint32(p[2:], 4)
	copy(p[6:], []byte("test"))
	binary.LittleEndian.PutUint32(p[10:], 4)
	p[19] = 16
	binary.LittleEndian.PutUint16(p[20:], 1)
	name, err := DecodeDuskyStandbyParty(p)
	if err != nil || string(name) != "test" {
		t.Fatal(name, err)
	}
	reply, err := DuskyStandbyPartyReply(name, 7, [2]byte{3, 91})
	if err != nil || len(reply) != 111 || reply[33] != 16 || reply[99] != 16 || binary.LittleEndian.Uint16(reply[4:]) != 1 {
		t.Fatal(reply, err)
	}
	p[19] = 13
	if _, err = DecodeDuskyStandbyParty(p); err == nil {
		t.Fatal("Cloister party accepted as Dusky")
	}
	for _, bad := range [][]byte{nil, p[:15], append([]byte(nil), p...)} {
		if _, err = DecodeDuskyStandbyParty(bad); err == nil {
			t.Fatal("invalid party accepted")
		}
	}
}
