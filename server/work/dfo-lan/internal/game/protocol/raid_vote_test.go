package protocol

import (
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestRaidVoteActualStartAndNativeRows(t *testing.T) {
	p, _ := hex.DecodeString("98f55f0000000000feffffffff00010000000000000000000000000000000000")
	choice, err := DecodeRaidStartVote(p)
	if err != nil || choice != 0 {
		t.Fatalf("live start vote: %d/%v", choice, err)
	}
	for _, choice := range []byte{1, 2} {
		p[13] = choice
		got, err := DecodeRaidStartVote(p)
		if err != nil || got != choice {
			t.Fatal(got, err)
		}
	}
	for _, bad := range [][]byte{p[:17], append(append([]byte{}, p...), 0), make([]byte, 32)} {
		if _, err := DecodeRaidStartVote(bad); err == nil {
			t.Fatal("malformed vote accepted")
		}
	}
	p[31] = 1
	if _, err := DecodeRaidStartVote(p); err == nil {
		t.Fatal("nonzero padding accepted")
	}
	for _, result := range []uint32{0, 1, 2} {
		b, err := RaidSoloStartVote(2, result)
		if err != nil {
			t.Fatal(err)
		}
		if len(b) != 181 || binary.LittleEndian.Uint32(b[13:17]) != 1 || binary.LittleEndian.Uint32(b[17:21]) != result || binary.LittleEndian.Uint16(b[21:23]) != 2 {
			t.Fatalf("native181-byte vote %x", b)
		}
		choice := result
		if choice == 0 {
			choice = 1
		}
		if binary.LittleEndian.Uint32(b[25:29]) != choice {
			t.Fatal("initiating owner consent lost")
		}
		for at := 29; at < 181; at += 8 {
			if binary.LittleEndian.Uint16(b[at:at+2]) != 0 || binary.LittleEndian.Uint32(b[at+4:at+8]) != 3 {
				t.Fatal("fake member or missing no-vote sentinel")
			}
		}
	}
	if _, err := RaidSoloStartVote(0, 1); err == nil {
		t.Fatal("missing actual voter accepted")
	}
}
