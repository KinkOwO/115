package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestSparseOptionsMatchNativeMerge(t *testing.T) {
	data, err := os.ReadFile("testdata/native_account_options35.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Payload string `json:"payload_hex"`
			Value   uint16 `json:"value"`
		} `json:"cases"`
	}
	if err = json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 2 {
		t.Fatal("missing native cases")
	}
	for _, c := range fixture.Cases {
		got, err := AccountOptions(map[uint16]uint16{198: c.Value})
		want, decodeErr := hex.DecodeString(c.Payload)
		if err != nil || decodeErr != nil || !bytes.Equal(got, want) {
			t.Fatalf("native sparse option mismatch %d", c.Value)
		}
	}
	if _, err = AccountOptions(map[uint16]uint16{286: 1}); err == nil {
		t.Fatal("out of range option accepted")
	}
	if _, err = AccountOptions(map[uint16]uint16{198: 65535}); err == nil {
		t.Fatal("missing-value sentinel accepted")
	}
}
