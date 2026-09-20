package protocol

import (
	"bytes"
	"testing"
)

func TestMenuNativeShapes(t *testing.T) {
	for _, id := range []uint16{7, 1301} {
		if _, e := DecodeMenuRequest(id, nil); e != nil {
			t.Fatal(e)
		}
		if _, e := DecodeMenuRequest(id, []byte{0}); e == nil {
			t.Fatal("unexpected body accepted")
		}
	}
	p := make([]byte, 16)
	p[0] = 1
	if option, e := DecodeMenuRequest(3, p); e != nil || option != 1 {
		t.Fatalf("exit byte: %v", e)
	}
	p[15] = 1
	if _, e := DecodeMenuRequest(3, p); e == nil {
		t.Fatal("bad exit padding accepted")
	}
	if !bytes.Equal(MenuLeaveSuccess(), []byte{1, 0, 0, 0, 0}) {
		t.Fatal("native leave response")
	}
	exit := ExitDialogReady()
	if len(exit) != 3+0x80 || !bytes.Equal(exit[:3], []byte{1, 0, 0}) || !bytes.Equal(exit[3:], make([]byte, 0x80)) {
		t.Fatalf("native exit dialog response: len=%d hex=%x", len(exit), exit)
	}
	for _, p := range [][]byte{{0, 0, 0, 0}, {1, 0, 0, 0}} {
		flag, e := DecodeExitShutdownSignal(p)
		if e != nil || flag != (p[0] == 1) {
			t.Fatalf("exit shutdown signal %x: flag=%v error=%v", p, flag, e)
		}
	}
	for _, p := range [][]byte{{1}, {2, 0, 0, 0}, {1, 0, 1, 0}} {
		if _, e := DecodeExitShutdownSignal(p); e == nil {
			t.Fatalf("invalid exit shutdown signal accepted: %x", p)
		}
	}
	if !bytes.Equal(VillageReturnSuccess(38, 0), []byte{1, 38, 0, 0, 0, 0, 0, 0, 0}) {
		t.Fatal("native return destination")
	}
}
