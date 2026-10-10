package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestSoloPartyUsesCurrentNativeRoster(t *testing.T) {
	raw, err := os.ReadFile("testdata/native_solo_party_state115.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Payload string  `json:"payload_hex"`
		Actor   uint16  `json:"actor"`
		Context [2]byte `json:"context"`
	}
	if err = json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString(v.Payload)
	if err != nil {
		t.Fatal(err)
	}
	got, err := SoloPartyInfo(v.Actor, v.Context)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("current native single-member roster mismatch")
	}
	for _, actor := range []uint16{0, 65535} {
		if _, err := SoloPartyInfo(actor, v.Context); err == nil {
			t.Fatal("invalid actor accepted")
		}
	}
}
