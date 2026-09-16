package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestSelectCaptureAndBounds(t *testing.T) {
	p := make([]byte, 16)
	p[0] = 2
	slot, e := DecodeSelectRequest(p)
	if e != nil || slot != 2 {
		t.Fatalf("captured LanTest01: %d %v", slot, e)
	}
	p[4] = 1
	if _, e = DecodeSelectRequest(p); e == nil {
		t.Fatal("accepted unverified option")
	}
	if _, e = DecodeSelectRequest(p[:4]); e == nil {
		t.Fatal("accepted truncated frame")
	}
}
func TestSelectNativeReadBoundaries(t *testing.T) {
	p, e := SelectProbeSuccess(SelectProbeState{ActorServerID: 7, CreatedTime: 123456, WorldKind: 2, TutorialCompleted: []byte{3, 12}, Tail16: [2]uint16{9, 10}})
	if e != nil {
		t.Fatal(e)
	}
	if binary.LittleEndian.Uint32(p[5:]) != 123456 || binary.LittleEndian.Uint16(p[9:]) != 7 {
		t.Fatal("native createdTime/MyServerId field order")
	}
	// Independent offsets from the native read trace, including helper loops.
	if len(p) != 242 || binary.LittleEndian.Uint32(p[222:]) != 2 || !bytes.Equal(p[226:230], []byte{0, 2, 3, 12}) || binary.LittleEndian.Uint16(p[230:]) != 9 {
		t.Fatalf("misaligned native select response: %x", p)
	}
	for off := 22; off < 202; off += 6 {
		if binary.LittleEndian.Uint16(p[off:]) != 0xffff {
			t.Fatal("empty quest sentinel changed")
		}
	}
	if _, e = SelectProbeSuccess(SelectProbeState{TutorialCompleted: []byte{101}}); e == nil {
		t.Fatal("unsafe tutorial index")
	}
}

func TestSelectRestoresActiveQuestsAndOverflow(t *testing.T) {
	var q []ActiveQuest
	for i := 1; i <= 31; i++ {
		q = append(q, ActiveQuest{uint16(i), uint32(i * 3)})
	}
	p, e := SelectProbeSuccess(SelectProbeState{ActiveQuests: q, WorldKind: 38})
	if e != nil {
		t.Fatal(e)
	}
	if binary.LittleEndian.Uint16(p[22:]) != 1 || binary.LittleEndian.Uint32(p[24:]) != 3 || binary.LittleEndian.Uint32(p[202:]) != 1 || binary.LittleEndian.Uint16(p[206:]) != 31 || binary.LittleEndian.Uint32(p[208:]) != 93 || binary.LittleEndian.Uint32(p[228:]) != 38 {
		t.Fatal("native fixed/overflow quest cursor misaligned")
	}
	q[1] = q[0]
	if _, e = SelectProbeSuccess(SelectProbeState{ActiveQuests: q}); e == nil {
		t.Fatal("duplicate quest accepted")
	}
}
