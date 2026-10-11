package protocol

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

func TestWeaponSkinEchoMatchesCurrentNativeReader(t *testing.T) {
	// The captured Apply names 101010912 (e04d0506). The CMD registry
	// consumes success=1, then native 1444EE820 reads subtype/result/20 ids.
	want, err := hex.DecodeString("010400000000000000e04d0506" + strings.Repeat("00", 76))
	if err != nil {
		t.Fatal(err)
	}
	got := WeaponSkinSelectionEcho(101010912)
	if len(got) != 89 || !bytes.Equal(got, want) {
		t.Fatalf("native body mismatch: %d/%d %x", len(got), len(want), got)
	}
	clear := WeaponSkinSelectionEcho(0)
	if len(clear) != 89 || clear[0] != 1 || clear[1] != 4 || !bytes.Equal(clear[2:], make([]byte, 87)) {
		t.Fatal("unapply did not clear id")
	}
}
