package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestActorProfileFramesNativeHeader(t *testing.T) {
	s := EntryBasicProbe{ActorServerID: 2, Context: [2]byte{4, 7}, Character: CharacterRow{Name: "FrameTest", Level: 115, ContentClearFlags: [28]byte{1, 2, 3}}}
	before, err := UserInfoBasicProbe(s)
	if err != nil {
		t.Fatal(err)
	}
	s.PartyFrameID, s.RequestFrameID = 20078, 50017
	after, err := UserInfoBasicProbe(s)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != len(after) || binary.LittleEndian.Uint32(after[5+0x2c:]) != 20078 || binary.LittleEndian.Uint32(after[5+0x30:]) != 50017 {
		t.Fatal("profile frame header did not match 145640441/145640454")
	}
	copy(after[5+0x2c:5+0x34], before[5+0x2c:5+0x34])
	if !bytes.Equal(before, after) {
		t.Fatal("profile frames changed other actor fields")
	}
}
