package protocol

import (
	"encoding/hex"
	"testing"
)

func TestAdventureSymbolCapturedApplyRequest(t *testing.T) {
	// 20261007_073614_436917_next37, CMD1726 at 07:38:08+08.
	// Native selection: 黑雨终歇之时, PVF index 258. The sender copies
	// all 20 structure bytes; its opaque 13-byte prefix remains in plaintext.
	p, err := hex.DecodeString("000000000000000098f55f000000000002010000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	id, err := DecodeAdventureSymbol(p)
	if err != nil || id != 258 {
		t.Fatalf("captured apply decoded as %d: %v", id, err)
	}
}
