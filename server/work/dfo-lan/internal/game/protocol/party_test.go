package protocol

import (
	"bytes"
	"testing"
)

func TestSoloPartyUsesCurrentNativeRoster(t *testing.T) {
	p, e := SoloPartyInfo(3)
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(p, nativeInventoryFixture(t, "solo_party35")) {
		t.Fatal("current solo party cursor mismatch")
	}
	for _, actor := range []uint16{0, 65535} {
		if _, e := SoloPartyInfo(actor); e == nil {
			t.Fatal("invalid actor accepted")
		}
	}
}
