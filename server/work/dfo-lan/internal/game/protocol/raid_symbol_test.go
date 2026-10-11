package protocol

import (
	"encoding/hex"
	"testing"
)

func TestNativeRaidGlobalSymbolLayout(t *testing.T) {
	p, err := RaidGlobalSymbol115(211, 1)
	if err != nil || hex.EncodeToString(p) != "01d300000001000000" {
		t.Fatal("native persistent symbol reader mismatch", p, err)
	}
	if _, err := RaidGlobalSymbol115(0, 1); err == nil {
		t.Fatal("invalid symbol accepted")
	}
}
