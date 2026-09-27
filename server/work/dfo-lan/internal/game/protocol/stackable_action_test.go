package protocol

import (
	"encoding/hex"
	"strings"
	"testing"
)

// stackableFrame is the live CMD507 body: slot u16 at 0, action u32 at 7, every
// other byte zero. Captured 2026-09-26 while using damage-font consumables.
func stackableFrame(slot uint16, action byte, size int) []byte {
	p := make([]byte, size)
	p[0] = byte(slot)
	p[1] = byte(slot >> 8)
	p[7] = action
	return p
}

// addSkinStorageCapture is the 64-byte plain body the client sent for bag slot
// 71 holding template 10358669.
var addSkinStorageCapture = "4700" + strings.Repeat("00", 5) + "a9" + strings.Repeat("00", 56)

func TestAddSkinStorageCaptured(t *testing.T) {
	if len(addSkinStorageCapture) != 128 {
		t.Fatal("vector length")
	}
	p, e := hex.DecodeString(addSkinStorageCapture)
	if e != nil {
		t.Fatal(e)
	}
	slot, action, e := DecodeStackableAction(p)
	if e != nil || slot != 71 || action != 169 {
		t.Fatal(slot, action, e)
	}
	if got, e := DecodeAddSkinStorageAction(p); e != nil || got != 71 {
		t.Fatal(got, e)
	}
	// The fatigue path must keep refusing the skin frame, and the skin path the
	// fatigue frame, so the two uses of CMD507 can never cross.
	if _, e := DecodeFatigueAction(p); e == nil {
		t.Fatal("fatigue decoder accepted action 169")
	}
	q := stackableFrame(66, 54, 64)
	if _, e := DecodeAddSkinStorageAction(q); e == nil {
		t.Fatal("skin decoder accepted action 54")
	}
	if got, e := DecodeFatigueAction(q); e != nil || got != 66 {
		t.Fatal(got, e)
	}
	if _, _, e := DecodeStackableAction(stackableFrame(71, 169, 63)); e == nil {
		t.Fatal("short frame accepted")
	}
	mutated := append([]byte{}, p...)
	mutated[len(mutated)-1] = 1
	if _, _, e := DecodeStackableAction(mutated); e == nil {
		t.Fatal("nonzero tail accepted")
	}
}

// AddSkinStorageVector is the 64-byte plain body the client sent for bag slot 71
// holding template 10358669.
var AddSkinStorageVector = "4700" + strings.Repeat("00", 6) + "a9" + strings.Repeat("00", 55)
