package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestRaidSubPartyActualCreateAndRealRoster(t *testing.T) {
	p, _ := hex.DecodeString("0000090000005061727479203a203104ffffffff050000000000000707070707070707ffffffff000000000000000000")
	b, err := RaidSoloSubParty115(2, [2]byte{3, 83}, 1, p)
	if err != nil {
		t.Fatal(err)
	}
	q := b[9:]
	if len(b) != 116 || binary.LittleEndian.Uint32(b[14:18]) != 9 || q[20] != 4 || q[69] != 1 || binary.LittleEndian.Uint16(q[71:73]) != 2 || q[29] != 0 || q[95] != 0 {
		t.Fatal("native real ordinary subparty grammar changed", b)
	}
	for _, assignment := range []byte{0, 2, 4} {
		if _, err := RaidSoloSubParty115(2, [2]byte{3, 83}, assignment, p); err == nil {
			t.Fatal("unassigned or foreign subparty accepted")
		}
	}
	p[24] = 8
	if _, err := RaidSoloSubParty115(2, [2]byte{3, 83}, 1, p); err == nil {
		t.Fatal("special mode admitted as raid ordinary subparty")
	}
}
