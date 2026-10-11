package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestRaidRewardNativeObservedRecord20261002(t *testing.T) {
	// Existing user-supplied capture SHA256 d4680844...f6, frames seq1352.
	// Opcode names of kind1 acknowledgements are unreliable; this is kind0
	// N587. Reader144ce6990 consumes60 bytes, followed by the capture trailer.
	capture, err := hex.DecodeString("00ffffffff01550003080000006e61697169616e67000100b14d9d0006000000000000000000000000000000000000000000000000000000000000004da1688f3b00000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if capture[0] != 0 || binary.LittleEndian.Uint32(capture[1:]) != ^uint32(0) || capture[5] != 1 || binary.LittleEndian.Uint16(capture[6:]) != 85 {
		t.Fatal("native list header")
	}
	if binary.LittleEndian.Uint32(capture[9:]) != 8 || string(capture[13:21]) != "naiqiang" || capture[22] != 1 || capture[23] != 0 || binary.LittleEndian.Uint32(capture[24:]) != 10309041 || binary.LittleEndian.Uint16(capture[28:]) != 6 {
		t.Fatal("native owned reward record")
	}
	if len(capture[23:60]) != 37 || !bytes.Equal(capture[30:60], make([]byte, 30)) {
		t.Fatal("native item tail")
	}
	encoded, err := RaidRewardList115(0, 85, "naiqiang", []RaidRewardItem115{{Template: 10309041, Amount: 6}})
	if err != nil {
		t.Fatal(err)
	}
	// Both one-byte actor fields are read then unused by the current handler.
	// The observed actor byte is3; the gateway uses the neutral value0.
	want := append([]byte(nil), capture[:60]...)
	want[8] = 0
	if !bytes.Equal(encoded, want) {
		t.Fatalf("native reader record mismatch %x / %x", encoded, want)
	}
}

func TestRaidNativeCardSenderAndBounds(t *testing.T) {
	// Current PE143d4a46d/4a4: two146d75cc0 one-byte writes; no opaque prefix.
	for _, b := range [][]byte{{1, 11}, {0, 11}, {2, 0}, append([]byte{1, 11}, make([]byte, 14)...)} {
		r, err := DecodeRaidCardSelection115(b)
		if err != nil || r.Kind != b[0] || r.Index != b[1] {
			t.Fatal(r, err)
		}
	}
	for _, b := range [][]byte{{0}, {3, 0}, {0, 0, 0}, {1, 0, 1, 0, 0, 0, 0, 0}} {
		if _, err := DecodeRaidCardSelection115(b); err == nil {
			t.Fatal("malformed native selection accepted", b)
		}
	}
	for _, items := range [][]RaidRewardItem115{{{Template: 1, Amount: 0}}, {{Template: 0, Amount: 1}}, {{Template: 1, Amount: 32768}}, make([]RaidRewardItem115, 256)} {
		if _, err := RaidRewardList115(0, 9, "owner", items); err == nil {
			t.Fatal("invalid item accepted")
		}
	}
	if _, err := RaidRewardList115(0, 0, "owner", nil); err == nil {
		t.Fatal("unowned list accepted")
	}
	if !bytes.Equal(RaidRewardDecision115(), []byte{0}) {
		t.Fatal("N2302 native one-byte consumption")
	}
}
