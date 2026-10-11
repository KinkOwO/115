package protocol

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

// This historical fixture proves only the N24 town-area packet reader. It
// does not prove dungeon loading, destination, control or menu recovery.
func TestTownAreaEmptyNativeVector(t *testing.T) {
	raw, err := os.ReadFile("testdata/native_dungeon_scene_ready115.json")
	if err != nil {
		t.Fatal(err)
	}
	var vector struct {
		Payload  string `json:"payload_hex"`
		Consumed int    `json:"consumed"`
	}
	if err := json.Unmarshal(raw, &vector); err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString(vector.Payload)
	if err != nil {
		t.Fatal(err)
	}
	got, err := AreaUsers(38, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if vector.Consumed != len(got) || !bytes.Equal(got, want) {
		t.Fatal("native N24 zero-row town-area reader contract mismatch")
	}
}
