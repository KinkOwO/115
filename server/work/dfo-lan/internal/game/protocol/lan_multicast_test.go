package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// NOTI 22 USER_POSITION: recovered from the current client's handler at
// 0x145312610, which reads +0 u16 actor, +2 u16 x, +4 u16 y, +6 u8 motion and
// +7 u16 speed.
func TestUserPositionLayout(t *testing.T) {
	p, err := UserPosition(0x1234, 0x068D, 0x00DE, 0x05, 0x0064)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 9 {
		t.Fatalf("NOTI 22 body is nine bytes, got %d", len(p))
	}
	if got := binary.LittleEndian.Uint16(p[0:]); got != 0x1234 {
		t.Fatalf("actor: 0x%X", got)
	}
	if got := binary.LittleEndian.Uint16(p[2:]); got != 0x068D {
		t.Fatalf("x: 0x%X", got)
	}
	if got := binary.LittleEndian.Uint16(p[4:]); got != 0x00DE {
		t.Fatalf("y: 0x%X", got)
	}
	if p[6] != 0x05 {
		t.Fatalf("motion: 0x%X", p[6])
	}
	if got := binary.LittleEndian.Uint16(p[7:]); got != 0x0064 {
		t.Fatalf("speed: 0x%X", got)
	}
}

// The movement notification has to carry exactly the body the client itself
// reports in CMD 35, so the two cannot drift apart.
func TestUserPositionMirrorsClientPositionReport(t *testing.T) {
	// Live capture of CMD 35 from this client: 8d06de0005640000
	report := []byte{0x8d, 0x06, 0xde, 0x00, 0x05, 0x64, 0x00, 0x00}
	r, err := DecodePositionRequest(report)
	if err != nil {
		t.Fatal(err)
	}
	out, err := UserPosition(7, r.X, r.Y, r.Motion, r.Speed)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{7, 0}, report[:7]...)
	if !bytes.Equal(out, want) {
		t.Fatalf("NOTI 22 body % X does not mirror the CMD 35 report % X", out, want)
	}
}

// NOTI 6 USER_LEAVE: handler 0x145312170 reads a single u16 actor id.
func TestUserLeaveLayout(t *testing.T) {
	p, err := UserLeave(0x1234)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 2 || binary.LittleEndian.Uint16(p) != 0x1234 {
		t.Fatalf("NOTI 6 body % X is not one actor id", p)
	}
}

func TestUserLeaveRejectsReservedActors(t *testing.T) {
	if _, err := UserLeave(0); err == nil {
		t.Fatal("actor 0 must be rejected")
	}
	if _, err := UserLeave(65535); err == nil {
		t.Fatal("actor 65535 must be rejected")
	}
	if _, err := UserPosition(0, 1, 2, 3, 4); err == nil {
		t.Fatal("actor 0 must be rejected for positions too")
	}
}
